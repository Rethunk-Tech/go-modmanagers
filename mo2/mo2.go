// Package mo2 reads Mod Organizer 2's instance folders: ModOrganizer.ini, profiles/<p>/modlist.txt and
// mods/<m>/meta.ini. It never writes them.
package mo2

import (
	"cmp"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Rethunk-Tech/go-modmanagers/internal/nexusurl"
	safefs "github.com/Rethunk-Tech/go-safefs"
)

// Instances lists the global instance folders, <localAppData>/ModOrganizer/<name>. A missing folder has none.
func Instances(localAppData string) ([]string, error) {
	root := filepath.Join(localAppData, "ModOrganizer")
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, filepath.Join(root, e.Name()))
		}
	}
	return out, nil
}

// Instance is one instance folder's ModOrganizer.ini, resolved.
type Instance struct {
	GameName string // [General] gameName
	Mods     string // [Settings] mod_directory, %BASE_DIR% resolved, default <base>/mods
	Profiles string // [Settings] profiles_directory, likewise, default <base>/profiles
}

// Open reads <dir>/ModOrganizer.ini, for any instance folder the caller names (global, portable or chosen).
// base is [Settings] base_directory, default dir (MO2's settings.cpp PathSettings). Values are decoded as
// QSettings wrote them (@ByteArray(...), quotes, backslash escapes); a relative path resolves against dir, so an
// absolute dir gives absolute Mods and Profiles. A missing or unreadable ini is an error.
func Open(dir string) (Instance, error) {
	data, err := safefs.ReadFile(filepath.Join(dir, "ModOrganizer.ini"))
	if err != nil {
		return Instance{}, err
	}
	return parseInstance(dir, data), nil
}

func parseInstance(dir string, data []byte) Instance {
	settings := iniSection(data, "settings")
	// MO2 leaves a relative path relative to its own working directory, which is the instance folder.
	abs := func(p string) string {
		p = filepath.FromSlash(strings.ReplaceAll(p, `\`, "/"))
		if !filepath.IsAbs(p) {
			p = filepath.Join(dir, p)
		}
		return filepath.Clean(p)
	}
	base := dir
	if b := iniPath(settings["base_directory"]); b != "" {
		base = abs(b)
	}
	resolve := func(key, def string) string {
		path := cmp.Or(iniPath(settings[key]), "%BASE_DIR%/"+def)
		return abs(strings.ReplaceAll(path, "%BASE_DIR%", filepath.ToSlash(base)))
	}
	return Instance{
		GameName: iniSection(data, "general")["gamename"],
		Mods:     resolve("mod_directory", "mods"),
		Profiles: resolve("profiles_directory", "profiles"),
	}
}

// ProfileNames lists the folders under in.Profiles. A missing folder has none.
func (in Instance) ProfileNames() ([]string, error) {
	entries, err := os.ReadDir(in.Profiles)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	return out, nil
}

// Entry is one mod in a profile's modlist.txt.
type Entry struct {
	Name    string
	Enabled bool
}

// ReadModlist reads <in.Profiles>/<profile>/modlist.txt in file order, as ParseModlist reads it.
func (in Instance) ReadModlist(profile string) ([]Entry, error) {
	data, err := safefs.ReadFile(filepath.Join(in.Profiles, profile, "modlist.txt"))
	if err != nil {
		return nil, err
	}
	return ParseModlist(data), nil
}

// ParseModlist reads modlist.txt in file order: '+' enabled, '-' disabled; '*' (unmanaged) and "_separator"
// entries, blank lines and comments are skipped. A repeated name keeps its first place and its last flag.
func ParseModlist(data []byte) []Entry {
	seen := map[string]int{}
	var out []Entry
	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" || strings.HasPrefix(line, "#") || strings.HasSuffix(line, "_separator") || len(line) < 2 {
			continue
		}
		flag, name := line[0], line[1:]
		if flag != '+' && flag != '-' {
			continue
		}
		item := Entry{Name: name, Enabled: flag == '+'}
		if i, ok := seen[name]; ok {
			out[i] = item
			continue
		}
		seen[name] = len(out)
		out = append(out, item)
	}
	return out
}

// Meta is what a mod folder's meta.ini records.
type Meta struct {
	NexusID int    // [General] modid, else the id in [General] url
	Version string // [General] version
}

// ReadMeta reads <modDir>/meta.ini. A missing or unreadable file is the zero Meta.
func ReadMeta(modDir string) Meta {
	data, err := safefs.ReadFile(filepath.Join(modDir, "meta.ini"))
	if err != nil {
		return Meta{}
	}
	general := iniSection(data, "general")
	id, _ := strconv.Atoi(strings.TrimSpace(general["modid"]))
	if id == 0 {
		id = nexusurl.ID(general["url"])
	}
	return Meta{NexusID: id, Version: general["version"]}
}

// iniPath decodes a path value as QSettings wrote it: a QByteArray is "@ByteArray(...)", and a QString with
// special characters is double-quoted, both with backslash escapes (a Windows path's backslashes are doubled).
func iniPath(raw string) string {
	if inner, ok := strings.CutPrefix(raw, "@ByteArray("); ok {
		raw = strings.TrimSuffix(inner, ")")
	}
	raw = strings.TrimSpace(raw)
	if len(raw) >= 2 && raw[0] == '"' && raw[len(raw)-1] == '"' {
		raw = raw[1 : len(raw)-1]
	}
	var b strings.Builder
	for i := 0; i < len(raw); i++ {
		if raw[i] != '\\' || i+1 == len(raw) {
			b.WriteByte(raw[i])
			continue
		}
		i++
		switch raw[i] {
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'r':
			b.WriteByte('\r')
		default:
			b.WriteByte(raw[i])
		}
	}
	return b.String()
}

// iniSection reads the key=value pairs of the named section (lowercase), keys lowercased.
func iniSection(data []byte, want string) map[string]string {
	out := map[string]string{}
	section := ""
	for line := range strings.SplitSeq(strings.TrimPrefix(string(data), "\ufeff"), "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(strings.TrimSpace(line[1 : len(line)-1]))
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || section != want {
			continue
		}
		out[strings.ToLower(strings.TrimSpace(key))] = strings.TrimSpace(value)
	}
	return out
}
