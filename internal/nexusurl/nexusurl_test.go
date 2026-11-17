package nexusurl

import "testing"

func FuzzID(f *testing.F) {
	f.Add("https://www.nexusmods.com/stardewvalley/mods/1915")
	f.Add("https://www.nexusmods.com/stardewvalley/Mods/1915/?tab")
	f.Add("https://www.nexusmods.com/stardewvalley/mods/0")
	f.Add("https://www.nexusmods.com/stardewvalley/mods/x")
	f.Add("://bad")
	f.Add("")
	f.Fuzz(func(t *testing.T, raw string) {
		if id := ID(raw); id < 0 {
			t.Fatalf("ID(%q) = %d", raw, id)
		}
	})
}

func TestID(t *testing.T) {
	t.Parallel()
	for raw, want := range map[string]int{
		"https://www.nexusmods.com/stardewvalley/mods/1915":      1915,
		"https://www.nexusmods.com/stardewvalley/Mods/1915/?tab": 1915,
		"https://www.nexusmods.com/stardewvalley/mods/0":         0,
		"https://www.nexusmods.com/stardewvalley/mods/x":         0,
		"":       0,
		"://bad": 0,
	} {
		if got := ID(raw); got != want {
			t.Errorf("ID(%q) = %d, want %d", raw, got, want)
		}
	}
}
