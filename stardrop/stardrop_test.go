package stardrop

import (
	"path/filepath"
	"slices"
	"strconv"
	"testing"

	"github.com/Rethunk-Tech/go-modmanagers/internal/testwrite"
)

func write(t *testing.T, path, body string) { testwrite.File(t, path, body) }

func TestProfiles(t *testing.T) {
	t.Parallel()
	data := DataDir(t.TempDir())
	if ids, err := ProfileIDs(data); err != nil || ids != nil {
		t.Fatalf("missing folder: %v, %v", ids, err)
	}
	write(t, filepath.Join(data, "Profiles", "Farm.json"),
		`{"Name":"  Farm ","EnabledModIds":["A.One"," B.Two "],"ModData":[{"UniqueId":"A.One","Version":"1.0","Name":"One","ModPageUri":"https://www.nexusmods.com/stardewvalley/mods/12"}]}`)
	write(t, filepath.Join(data, "Profiles", "Kept.JSON"),
		`{"EnabledModIds":[{"UniqueId":"C.Three","SourceId":"collection-1"}],"PreservedModConfigs":{"c.three":{"Volume":3}}}`)
	write(t, filepath.Join(data, "Profiles", "notes.txt"), "x")
	ids, err := ProfileIDs(data)
	slices.Sort(ids)
	if err != nil || !slices.Equal(ids, []string{"Farm", "Kept"}) {
		t.Fatalf("ids = %v, %v", ids, err)
	}
	farm, err := ReadProfile(data, "Farm")
	if err != nil || farm.Name != "Farm" || !slices.Equal(farm.EnabledModIDs, []string{"A.One", "B.Two"}) || farm.ModData[0].NexusID() != 12 {
		t.Fatalf("farm = %+v, %v", farm, err)
	}
	kept, err := ReadProfile(data, "Kept")
	if err != nil || kept.Name != "Kept" || !slices.Equal(kept.EnabledModIDs, []string{"C.Three"}) || string(kept.PreservedModConfigs["c.three"]) != `{"Volume":3}` {
		t.Fatalf("kept = %+v, %v", kept, err)
	}
	if _, err := ReadProfile(data, "Nope"); err == nil {
		t.Fatal("a missing profile was read")
	}
}

func TestModsFolderPrecedence(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	fallback := filepath.Join(dir, "fallback", "..", "game")
	if got := ModsFolder(dir, fallback); got != filepath.Join(dir, "game") {
		t.Fatalf("fallback = %q", got)
	}
	abs, err := filepath.Abs(string(filepath.Separator))
	if err != nil {
		t.Fatal(err)
	}
	smapi, mods := filepath.Join(abs, "smapi"), filepath.Join(abs, "mods")
	write(t, filepath.Join(dir, "Settings.json"), `{"SMAPIFolderPath":`+strconv.Quote(smapi)+`}`)
	if got := ModsFolder(dir, fallback); got != filepath.Join(smapi, "Mods") {
		t.Fatalf("smapi = %q", got)
	}
	write(t, filepath.Join(dir, "Settings.json"), `{"SMAPIFolderPath":`+strconv.Quote(smapi)+`,"ModFolderPath":`+strconv.Quote(mods+string(filepath.Separator))+`}`)
	if got := ModsFolder(dir, fallback); got != mods {
		t.Fatalf("mods = %q", got)
	}
}

func TestModsFolderResolvesARelativeSettingAgainstTheDataFolder(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	write(t, filepath.Join(dir, "Settings.json"), `{"ModFolderPath":"Mods"}`)
	if got := ModsFolder(dir, "fb"); got != filepath.Join(dir, "Mods") {
		t.Fatalf("relative = %q", got)
	}
}

func FuzzProfile(f *testing.F) {
	f.Add([]byte(`{"Name":"n","EnabledModIds":["a",{"UniqueId":"b"}],"ModData":[{}],"PreservedModConfigs":{"x":null}}`))
	f.Add([]byte(`{"EnabledModIds":[1]}`))
	f.Add([]byte(`[`))
	f.Fuzz(func(t *testing.T, data []byte) {
		p, err := parseProfile(data, "stem")
		if err == nil && p.Name == "" {
			t.Fatal("empty name")
		}
	})
}

func TestReadProfileRefusesAnIDThatIsAPath(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	write(t, filepath.Join(base, "secret", "loot.json"), `{"name":"Pwned"}`)
	data := filepath.Join(base, "data")
	for _, id := range []string{"../../secret/loot", "..", ".", `a\b`, "a/b", "x.json:stream", ""} {
		if _, err := ReadProfile(data, id); err == nil {
			t.Errorf("ReadProfile(%q) read a file", id)
		}
	}
}

func TestReadProfileOpensAStemWithADot(t *testing.T) {
	t.Parallel()
	data := DataDir(t.TempDir())
	write(t, filepath.Join(data, "Profiles", "v1.2.json"), `{"EnabledModIds":["A.One"]}`)
	ids, err := ProfileIDs(data)
	if err != nil || !slices.Equal(ids, []string{"v1.2"}) {
		t.Fatalf("ids = %v, %v", ids, err)
	}
	p, err := ReadProfile(data, ids[0])
	if err != nil || p.Name != "v1.2" || !slices.Equal(p.EnabledModIDs, []string{"A.One"}) {
		t.Fatalf("profile = %+v, %v", p, err)
	}
}
