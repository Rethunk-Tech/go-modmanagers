package vortex

import (
	"crypto/sha256"
	"errors"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	safefs "github.com/Rethunk-Tech/go-safefs"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/storage"
)

// newData builds a Vortex data folder whose state.v2 is a real LevelDB holding kvs.
func newData(t *testing.T, kvs map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	db, err := leveldb.OpenFile(filepath.Join(dir, "state.v2"), nil)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range kvs {
		if err := db.Put([]byte(k), []byte(v), nil); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	return dir
}

// vortex28 is shaped as Vortex 2.8 writes a profile and a mod installed from a zip.
func vortex28() map[string]string {
	const mod = "persistent###mods###stardewvalley###TestRetexture-1.0.1###"
	return map[string]string{
		"persistent###profiles###6LuTh37Vu###id":                                       `"6LuTh37Vu"`,
		"persistent###profiles###6LuTh37Vu###gameId":                                   `"stardewvalley"`,
		"persistent###profiles###6LuTh37Vu###name":                                     `"Default"`,
		"persistent###profiles###6LuTh37Vu###modState###TestRetexture-1.0.1###enabled": `true`,
		"persistent###profiles###p2###modState###off###enabled":                        `false`,
		"persistent###profiles###p2###gameId":                                          `"StardewValley"`,
		"persistent###mods###stardewvalley###disabled-1":                               `{"id":"disabled-1","installationPath":"Disabled-1"}`,
		mod + "id":                    `"TestRetexture-1.0.1"`,
		mod + "installationPath":      `"TestRetexture-1.0.1"`,
		mod + "attributes###name":     `"TestRetexture-1.0.1"`,
		mod + "attributes###uniqueId": `"Tester.TestRetexture"`,
		mod + "attributes###modId":    `1915`,
		mod + "attributes###version":  `"1.0.1"`,
		"settings###gameMode###discovered###stardewvalley###path": `"C:\\Games\\Stardew Valley"`,
		"user###multiUser": `true`,
		"garbage":          `{not json`,
	}
}

func TestReadBuildsProfilesAndMods(t *testing.T) {
	t.Parallel()
	dir := newData(t, vortex28())
	s, err := Read(dir, Options{TempDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	profiles := s.Profiles("stardewvalley")
	slices.SortFunc(profiles, func(a, b Profile) int { return strings.Compare(a.ID, b.ID) })
	if len(profiles) != 2 || profiles[0].ID != "6LuTh37Vu" || profiles[0].Name != "Default" || !profiles[0].Enabled["TestRetexture-1.0.1"] ||
		profiles[1].ID != "p2" || profiles[1].Name != "p2" || profiles[1].Enabled["off"] {
		t.Fatalf("profiles = %+v", profiles)
	}
	if len(s.Profiles("")) != 2 || len(s.Profiles("other")) != 0 {
		t.Fatal("game filter")
	}
	mods := s.Mods("StardewValley")
	slices.SortFunc(mods, func(a, b Mod) int { return strings.Compare(a.ID, b.ID) })
	if len(mods) != 2 || mods[0].ID != "TestRetexture-1.0.1" || mods[1].InstallationPath != "Disabled-1" {
		t.Fatalf("mods = %+v", mods)
	}
	m := mods[0]
	if m.Attr("nope", "uniqueId") != "Tester.TestRetexture" || m.AttrInt("modId") != 1915 || m.Attr("modId") != "" {
		t.Fatalf("attrs: %q %d", m.Attr("uniqueId"), m.AttrInt("modId"))
	}
	staging := s.StagingFolder("stardewvalley")
	if staging != filepath.Join(dir, "stardewvalley", "mods") || m.Folder(staging) != filepath.Join(staging, "TestRetexture-1.0.1") {
		t.Fatalf("staging = %q, folder = %q", staging, m.Folder(staging))
	}
	if !s.MultiUser() {
		t.Fatal("multiUser")
	}
}

func TestAttrIntReadsAStringHoldingANumber(t *testing.T) {
	t.Parallel()
	s, err := Read(newData(t, map[string]string{
		"persistent###mods###g###a###attributes###modId": `"42"`,
		"persistent###mods###g###b###attributes###modId": `"x"`,
	}), Options{TempDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]int{}
	for _, m := range s.Mods("g") {
		got[m.ID] = m.AttrInt("modId")
	}
	if got["a"] != 42 || got["b"] != 0 {
		t.Fatalf("modIds = %v", got)
	}
}

func TestStagingFolderDefaultsAndPlaceholders(t *testing.T) {
	t.Parallel()
	u, err := user.Current()
	if err != nil {
		t.Skip(err)
	}
	name := u.Username[strings.LastIndexAny(u.Username, `\/`)+1:]
	outside := filepath.Join(t.TempDir(), "staging")
	cases := map[string]struct{ pattern, want string }{
		"unset":       {"", ""},
		"placeholder": {`"{USERDATA}\\{game}\\mods"`, ""},
		"username":    {`"{userdata}/{USERNAME}/{Game}"`, name + string(filepath.Separator) + "g"},
		"absolute":    {`"` + filepath.ToSlash(outside) + `"`, outside},
	}
	for label, tc := range cases {
		t.Run(label, func(t *testing.T) {
			t.Parallel()
			kvs := map[string]string{"settings###mods###installPath###g": tc.pattern}
			if tc.pattern == "" {
				kvs = map[string]string{"persistent###x": `1`}
			}
			dir := newData(t, kvs)
			s, err := Read(dir, Options{TempDir: t.TempDir()})
			if err != nil {
				t.Fatal(err)
			}
			want := filepath.Join(dir, "g", "mods")
			switch label {
			case "username":
				want = filepath.Join(dir, tc.want)
			case "absolute":
				want = tc.want
			}
			if got := s.StagingFolder("g"); got != want {
				t.Fatalf("staging = %q want %q", got, want)
			}
		})
	}
}

func TestNestedProfilesDecodeIndependently(t *testing.T) {
	t.Parallel()
	s, err := Read(newData(t, map[string]string{"persistent###profiles": `{"stardewvalley":{
			"a":{"gameId":"stardewvalley","name":"A","modState":{"m1":{"enabled":true},"m2":{"enabled":true}}},
			"b":{"gameId":"stardewvalley","modState":{"m3":{"enabled":true}}},
			"c":{"name":"no game"}}}`}), Options{TempDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]Profile{}
	for _, p := range s.Profiles("") {
		got[p.ID] = p
	}
	if len(got) != 2 || got["a"].Name != "A" || len(got["a"].Enabled) != 2 || got["b"].Name != "b" || len(got["b"].Enabled) != 1 {
		t.Fatalf("profiles = %#v", got)
	}
}

func TestRootsOrder(t *testing.T) {
	t.Parallel()
	got := Roots("cfg", "pd", "chosen", true, false)
	if want := []string{"chosen", filepath.Join("cfg", "Vortex"), filepath.Join("pd", "vortex")}; !slices.Equal(got, want) {
		t.Fatalf("windows roots = %v, want %v", got, want)
	}
	if got = Roots("cfg", "pd", "chosen", true, true); !slices.Equal(got, []string{"chosen", filepath.Join("pd", "vortex")}) {
		t.Fatalf("multi-user roots = %v", got)
	}
	if got = Roots("cfg", "pd", "", false, false); !slices.Equal(got, []string{filepath.Join("cfg", "Vortex")}) {
		t.Fatalf("linux roots = %v", got)
	}
}

func TestReadNeverWritesTheLiveFolder(t *testing.T) {
	t.Parallel()
	dir := newData(t, vortex28())
	if err := os.WriteFile(filepath.Join(dir, "state.v2", "LOCK"), []byte("held"), 0o600); err != nil {
		t.Fatal(err)
	}
	before := hashTree(t, dir)
	tmp := t.TempDir()
	if _, err := Read(dir, Options{TempDir: tmp}); err != nil {
		t.Fatal(err)
	}
	if after := hashTree(t, dir); len(after) != len(before) || !mapsEqual(before, after) {
		t.Fatal("the live database changed")
	}
	if left, _ := os.ReadDir(tmp); len(left) != 0 {
		t.Fatalf("copy left behind: %v", left)
	}
}

func mapsEqual(a, b map[string][32]byte) bool {
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func hashTree(t *testing.T, dir string) map[string][32]byte {
	t.Helper()
	out := map[string][32]byte{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := safefs.ReadFile(path)
		out[path] = sha256.Sum256(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestReadMissingStateIsNotExist(t *testing.T) {
	t.Parallel()
	if _, err := Read(t.TempDir(), Options{}); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("err = %v", err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "state.v2"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(dir, Options{}); err == nil {
		t.Fatal("a file was read as a database")
	}
}

func FuzzTree(f *testing.F) {
	f.Add("persistent###profiles###p###gameId", `"g"`, "persistent###profiles###p", `{"name":"n","modState":{"m":{"enabled":true}}}`)
	f.Add("a###b", `1`, "a###b###c", `2`)
	f.Add("", `[`, "###", `null`)
	f.Fuzz(func(t *testing.T, k1, v1, k2, v2 string) {
		db, err := leveldb.Open(storage.NewMemStorage(), nil)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = db.Close() }()
		for k, v := range map[string]string{k1: v1, k2: v2} {
			if k == "" {
				continue
			}
			if err := db.Put([]byte(k), []byte(v), nil); err != nil {
				t.Fatal(err)
			}
		}
		it := db.NewIterator(nil, nil)
		defer it.Release()
		tree, err := buildTree(it)
		if err != nil {
			t.Fatal(err)
		}
		s := &State{dataDir: "d", tree: tree}
		s.Profiles("")
		s.Mods(k1)
		s.StagingFolder(k2)
		s.MultiUser()
	})
}
