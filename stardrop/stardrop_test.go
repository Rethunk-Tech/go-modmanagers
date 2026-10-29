package stardrop

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

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
	kept, err := ReadProfile(data, "Kept.JSON")
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
	write(t, filepath.Join(dir, "Settings.json"), `{"SMAPIFolderPath":"/smapi"}`)
	if got := ModsFolder(dir, fallback); got != filepath.Join("/smapi", "Mods") {
		t.Fatalf("smapi = %q", got)
	}
	write(t, filepath.Join(dir, "Settings.json"), `{"SMAPIFolderPath":"/smapi","ModFolderPath":"/mods/"}`)
	if got := ModsFolder(dir, fallback); got != filepath.Clean("/mods") {
		t.Fatalf("mods = %q", got)
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
