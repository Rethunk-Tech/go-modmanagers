// Package vortex reads Vortex's persisted state, the LevelDB database at <data>/state.v2. It works on a private
// copy and never opens the live files for writing.
package vortex

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	safefs "github.com/Rethunk-Tech/go-safefs"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/iterator"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

// Vortex persists its Redux state as a LevelDB database. Each leaf is stored under a "###"-joined path
// (hive###key###...) with a JSON-encoded value (src/main/src/store/LevelPersist.ts, SEPARATOR; values are written
// via JSON.stringify in ReduxPersistorIPC.ts).
const keySeparator = "###"

// ErrLocked means the database files are held open without read sharing, as a running Vortex holds its
// write-ahead log and manifest on Windows; a copy without them would be stale.
var ErrLocked = errors.New("vortex: database is held open by another process")

// Options tunes Read.
type Options struct {
	// TempDir is where the private copy of the database is made ("" means os.TempDir()). The copy is removed
	// with safefs.RemoveAll before Read returns.
	TempDir string
}

// State is Vortex's state tree as read, and the data folder it came from.
type State struct {
	dataDir string
	tree    map[string]json.RawMessage
}

// Read copies <dataDir>/state.v2 (tables first, CURRENT last, never LOCK), opens the copy read-only and
// rebuilds Vortex's state tree from its "###"-joined keys. The live files are never opened for writing. A missing
// state.v2 is an error matching fs.ErrNotExist.
func Read(dataDir string, opts Options) (*State, error) {
	src := filepath.Join(dataDir, "state.v2")
	info, err := os.Stat(src)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("read Vortex state %s: not a LevelDB directory", src)
	}
	tmp, err := os.MkdirTemp(opts.TempDir, "vortex-")
	if err != nil {
		return nil, err
	}
	defer func() { _ = safefs.RemoveAll(tmp) }()
	if err := copyLevelDB(src, tmp); err != nil {
		return nil, err
	}
	db, err := leveldb.OpenFile(tmp, &opt.Options{ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("read Vortex state %s: %w", src, err)
	}
	defer func() { _ = db.Close() }()
	it := db.NewIterator(nil, nil)
	defer it.Release()
	tree, err := buildTree(it)
	if err != nil {
		return nil, err
	}
	return &State{dataDir: dataDir, tree: tree}, nil
}

// copyLevelDB copies regular files except LOCK. Table files go first and CURRENT last so a compaction racing the
// copy cannot leave CURRENT pointing at a manifest newer than the tables copied.
func copyLevelDB(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	var names []string
	for _, e := range entries {
		if e.Type().IsRegular() && e.Name() != "LOCK" {
			names = append(names, e.Name())
		}
	}
	rank := func(n string) int {
		switch {
		case strings.HasSuffix(n, ".ldb"), strings.HasSuffix(n, ".sst"):
			return 0
		case n == "CURRENT":
			return 2
		}
		return 1
	}
	slices.SortStableFunc(names, func(a, b string) int { return cmp.Compare(rank(a), rank(b)) })
	for _, name := range names {
		data, err := safefs.ReadFile(filepath.Join(src, name))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if fileLocked(err) {
			return ErrLocked
		}
		if err != nil {
			return err
		}
		if err := safefs.WriteFile(filepath.Join(dst, name), data, 0o600); err != nil {
			return err
		}
	}
	return nil
}

// buildTree rebuilds the nested state map from the flat "###" keys. Parents sort before their children, so a
// blob stored at an intermediate path is extended by deeper leaf keys. A value that is not JSON is skipped.
func buildTree(it iterator.Iterator) (map[string]json.RawMessage, error) {
	root := map[string]any{}
	for it.Next() {
		var value any
		if json.Unmarshal(it.Value(), &value) != nil {
			continue
		}
		parts := strings.Split(string(it.Key()), keySeparator)
		node := root
		for _, part := range parts[:len(parts)-1] {
			child, ok := node[part].(map[string]any)
			if !ok {
				child = map[string]any{}
				node[part] = child
			}
			node = child
		}
		last := parts[len(parts)-1]
		if existing, ok := node[last].(map[string]any); ok {
			if incoming, ok := value.(map[string]any); ok {
				maps.Copy(existing, incoming)
				continue
			}
		}
		node[last] = value
	}
	if err := it.Error(); err != nil {
		return nil, err
	}
	out := make(map[string]json.RawMessage, len(root))
	for k, v := range root {
		data, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		out[k] = data
	}
	return out, nil
}

// Profile is one persistent.profiles entry.
type Profile struct {
	ID, GameID, Name string
	Enabled          map[string]bool // modState.<modID>.enabled
}

type wireProfile struct {
	ID       string `json:"id"`
	GameID   string `json:"gameId"`
	Name     string `json:"name"`
	ModState map[string]struct {
		Enabled bool `json:"enabled"`
	} `json:"modState"`
}

func (w wireProfile) profile() Profile {
	p := Profile{ID: w.ID, GameID: w.GameID, Name: w.Name, Enabled: make(map[string]bool, len(w.ModState))}
	for id, m := range w.ModState {
		p.Enabled[id] = m.Enabled
	}
	return p
}

// gameMatches is true for every game when want is empty, else case-insensitively.
func gameMatches(have, want string) bool { return want == "" || strings.EqualFold(have, want) }

// Profiles lists persistent.profiles for gameID (case-insensitive); "" lists every game's. An entry's id falls
// back to its key and its name to its id. A game-keyed map of profiles is read entry by entry, each decoded on its
// own, and an entry with no gameId is skipped.
func (s *State) Profiles(gameID string) []Profile {
	var direct map[string]json.RawMessage
	if json.Unmarshal(objectValue(s.tree, "persistent")["profiles"], &direct) != nil {
		return nil
	}
	var out []Profile
	for key, value := range direct {
		var w wireProfile
		if json.Unmarshal(value, &w) != nil {
			continue
		}
		if w.GameID != "" {
			w.ID = cmp.Or(w.ID, key)
			if gameMatches(w.GameID, gameID) {
				w.Name = cmp.Or(w.Name, w.ID)
				out = append(out, w.profile())
			}
			continue
		}
		var nested map[string]json.RawMessage
		if json.Unmarshal(value, &nested) != nil {
			continue
		}
		for nestedID, nestedValue := range nested {
			var np wireProfile
			if err := json.Unmarshal(nestedValue, &np); err != nil || np.GameID == "" || !gameMatches(np.GameID, gameID) {
				continue
			}
			np.ID = cmp.Or(np.ID, nestedID)
			np.Name = cmp.Or(np.Name, np.ID)
			out = append(out, np.profile())
		}
	}
	return out
}

// Mod is one persistent.mods.<gameID> entry.
type Mod struct {
	ID, InstallationPath string
	Attributes           map[string]json.RawMessage
}

// Mods lists persistent.mods.<gameID> (case-insensitive); an entry's id falls back to its key.
func (s *State) Mods(gameID string) []Mod {
	var raw json.RawMessage
	for id, v := range objectValue(objectValue(s.tree, "persistent"), "mods") {
		if strings.EqualFold(id, gameID) {
			raw = v
			break
		}
	}
	var values map[string]json.RawMessage
	if json.Unmarshal(raw, &values) != nil {
		return nil
	}
	out := make([]Mod, 0, len(values))
	for key, value := range values {
		var w struct {
			ID               string                     `json:"id"`
			InstallationPath string                     `json:"installationPath"`
			Attributes       map[string]json.RawMessage `json:"attributes"`
		}
		if json.Unmarshal(value, &w) != nil {
			continue
		}
		out = append(out, Mod{ID: cmp.Or(w.ID, key), InstallationPath: w.InstallationPath, Attributes: w.Attributes})
	}
	return out
}

// Folder is installationPath, else the id, below staging unless absolute.
func (m Mod) Folder(staging string) string {
	return cleanPath(staging, cmp.Or(m.InstallationPath, m.ID))
}

// Attr is the first of keys holding a JSON string.
func (m Mod) Attr(keys ...string) string { return rawString(m.Attributes, keys...) }

// AttrInt is the attribute key as a JSON number, or a string holding one; 0 when absent or neither.
func (m Mod) AttrInt(key string) int {
	raw, ok := m.Attributes[key]
	if !ok {
		return 0
	}
	var number int
	if json.Unmarshal(raw, &number) == nil {
		return number
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		if _, err := fmt.Sscan(text, &number); err != nil {
			return 0
		}
	}
	return number
}

// installPattern is the staging folder Vortex uses when none is stored
// (mod_management/util/getInstallPath.ts getInstallPathPattern).
const installPattern = "{USERDATA}/{GAME}/mods"

var placeholder = regexp.MustCompile(`(?i)\{(userdata|username|game)\}`)

// StagingFolder resolves settings.mods.installPath.<gameID> as Vortex's resolveInstallPath does: {USERDATA},
// {USERNAME} and {GAME} expand case-insensitively, backslashes are accepted, the default is
// "{USERDATA}/{GAME}/mods", and a relative result is relative to the data dir. {USERNAME} is the last segment of
// the OS user's name.
func (s *State) StagingFolder(gameID string) string {
	pattern := rawString(objectValue(objectValue(objectValue(s.tree, "settings"), "mods"), "installPath"), gameID)
	pattern = cmp.Or(pattern, installPattern)
	expanded := placeholder.ReplaceAllStringFunc(pattern, func(m string) string {
		switch strings.ToLower(m[1 : len(m)-1]) {
		case "userdata":
			return s.dataDir
		case "game":
			return gameID
		}
		return username()
	})
	return cleanPath(s.dataDir, filepath.FromSlash(strings.ReplaceAll(expanded, `\`, "/")))
}

func username() string {
	u, err := user.Current()
	if err != nil {
		return ""
	}
	name := u.Username
	return name[strings.LastIndexAny(name, `\/`)+1:]
}

// MultiUser is the flag Vortex stores at user###multiUser in the per-user database (src/main/src/Application.ts,
// setupPersistence): the literal true.
func (s *State) MultiUser() bool { return string(objectValue(s.tree, "user")["multiUser"]) == "true" }

// Roots lists where Vortex may keep state.v2, most specific first: chosen (when set), <appData>/Vortex unless
// multiUser, then on Windows <programData>/vortex (Application.ts multiUserPath).
func Roots(appData, programData, chosen string, windows, multiUser bool) []string {
	var roots []string
	if chosen != "" {
		roots = append(roots, filepath.Clean(chosen))
	}
	if !multiUser {
		roots = append(roots, filepath.Join(appData, "Vortex"))
	}
	if windows {
		roots = append(roots, filepath.Join(programData, "vortex"))
	}
	return roots
}

func cleanPath(root, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(root, path)
}

func objectValue(values map[string]json.RawMessage, key string) map[string]json.RawMessage {
	raw, ok := values[key]
	if !ok {
		return map[string]json.RawMessage{}
	}
	var out map[string]json.RawMessage
	if json.Unmarshal(raw, &out) != nil {
		return map[string]json.RawMessage{}
	}
	return out
}

func rawString(values map[string]json.RawMessage, keys ...string) string {
	for _, key := range keys {
		raw, ok := values[key]
		if !ok {
			continue
		}
		var value string
		if json.Unmarshal(raw, &value) == nil {
			return value
		}
	}
	return ""
}
