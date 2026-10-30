// Package r2modman reads r2modman's profile exports (.r2z files and share codes), its profile folders, and
// Thunderstore's profile-code API. It never writes them.
package r2modman

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
	"strconv"
	"strings"

	archive "github.com/Rethunk-Tech/go-archive"
	safefs "github.com/Rethunk-Tech/go-safefs"
	"go.yaml.in/yaml/v3"
)

const (
	exportFile = "export.r2x"
	maxFile    = 16 << 20
	maxEntries = 5000
	// maxTotal bounds a whole export's unpacked size, so many small-looking entries cannot add up to a zip bomb.
	maxTotal = 256 << 20
)

// Version is the {major, minor, patch} r2modman writes.
type Version struct{ Major, Minor, Patch int }

// String is "M.m.p".
func (v Version) String() string { return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch) }

// ParseVersion reads "major.minor.patch": exactly three non-negative decimal numbers.
func ParseVersion(s string) (Version, error) {
	parts := strings.Split(s, ".")
	var n [3]int
	if len(parts) != len(n) {
		return Version{}, fmt.Errorf("version %q is not major.minor.patch", s)
	}
	for i, p := range parts {
		if p == "" || strings.Trim(p, "0123456789") != "" {
			return Version{}, fmt.Errorf("version %q is not major.minor.patch", s)
		}
		v, err := strconv.Atoi(p)
		if err != nil {
			return Version{}, fmt.Errorf("version %q is not major.minor.patch: %w", s, err)
		}
		n[i] = v
	}
	return Version{n[0], n[1], n[2]}, nil
}

// Mod is one mod of a profile.
type Mod struct {
	Name    string  // Namespace-Name, as written; not validated
	Version Version // export.r2x "version", else mods.yml "versionNumber"
	Enabled bool    // absent means enabled
}

type wireVersion struct {
	Major int `yaml:"major"`
	Minor int `yaml:"minor"`
	Patch int `yaml:"patch"`
}

// wireMod is one mod in export.r2x and in mods.yml, which spells the version versionNumber. A zero version
// counts as absent.
type wireMod struct {
	Name          string      `yaml:"name"`
	Version       wireVersion `yaml:"version"`
	VersionNumber wireVersion `yaml:"versionNumber"`
	Enabled       *bool       `yaml:"enabled"`
}

// outMod is a mod as export.r2x writes it: only the spellings r2modman itself emits.
type outMod struct {
	Name    string      `yaml:"name"`
	Version wireVersion `yaml:"version"`
	Enabled bool        `yaml:"enabled"`
}

func (w wireMod) mod() Mod {
	v := w.Version
	if v == (wireVersion{}) {
		v = w.VersionNumber
	}
	return Mod{Name: w.Name, Version: Version(v), Enabled: w.Enabled == nil || *w.Enabled}
}

func mods(ws []wireMod) []Mod {
	out := make([]Mod, 0, len(ws))
	for _, w := range ws {
		out = append(out, w.mod())
	}
	return out
}

// Export is a read .r2z.
type Export struct {
	ProfileName, Community string
	Mods                   []Mod
	Files                  map[string][]byte // every other file, by cleaned slash path
}

// ReadExport reads an .r2z body: a zip with export.r2x. It reads under go-archive's admission rules: paths that
// leave the archive, link and special-file entries, two paths that differ only by case on any prefix, an exact
// duplicate, more than 5,000 entries, a file over 16 MiB or 256 MiB in all are refused, as is any name holding ':'
// (a drive letter would root a Windows join at it).
func ReadExport(zip []byte) (Export, error) {
	files, err := readZip(zip)
	if err != nil {
		return Export{}, err
	}
	raw, ok := files[exportFile]
	if !ok {
		return Export{}, errors.New("profile has no " + exportFile)
	}
	var doc struct {
		Name      string    `yaml:"profileName"`
		Community string    `yaml:"community"`
		Mods      []wireMod `yaml:"mods"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return Export{}, fmt.Errorf("reading %s: %w", exportFile, err)
	}
	delete(files, exportFile)
	return Export{ProfileName: doc.Name, Community: doc.Community, Mods: mods(doc.Mods), Files: files}, nil
}

// WriteExport writes e as an .r2z body that ReadExport reads back: export.r2x, then Files in path order. A
// Files key of export.r2x is refused, since it would shadow the mod list.
func WriteExport(e Export) ([]byte, error) {
	if _, clash := e.Files[exportFile]; clash {
		return nil, errors.New("files hold " + exportFile)
	}
	doc := struct {
		Name      string   `yaml:"profileName"`
		Community string   `yaml:"community,omitempty"`
		Mods      []outMod `yaml:"mods"`
	}{Name: e.ProfileName, Community: e.Community, Mods: make([]outMod, 0, len(e.Mods))}
	for _, m := range e.Mods {
		doc.Mods = append(doc.Mods, outMod{m.Name, wireVersion(m.Version), m.Enabled})
	}
	raw, err := yaml.Marshal(doc)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	write := func(name string, data []byte) error {
		w, err := zw.Create(name)
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err
	}
	if err := write(exportFile, raw); err != nil {
		return nil, err
	}
	for _, name := range slices.Sorted(maps.Keys(e.Files)) {
		if err := write(name, e.Files[name]); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// readZip returns the zip's files by cleaned slash path. Names are judged only by go-archive's admission rules.
func readZip(data []byte) (map[string][]byte, error) {
	// ExtractFunc detects the format by magic bytes and would take a tar or 7z; an .r2z is a zip.
	if !bytes.HasPrefix(data, []byte("PK")) {
		return nil, errors.New("not a zip archive")
	}
	out := map[string][]byte{}
	err := archive.ExtractFunc(bytes.NewReader(data), int64(len(data)), "profile.r2z",
		archive.Options{MaxEntries: maxEntries, MaxTotalBytes: maxTotal, MaxEntryBytes: maxFile},
		func(name string, isDir bool, r io.Reader) error {
			if isDir {
				return nil
			}
			if strings.Contains(name, ":") {
				return fmt.Errorf("archive path %q leaves the archive", name)
			}
			b, err := safefs.ReadCapped(r, maxFile)
			if err != nil {
				return fmt.Errorf("archive file %q: %w", name, err)
			}
			out[name] = b
			return nil
		})
	if err != nil {
		return nil, err
	}
	return out, nil
}
