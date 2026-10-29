package nexusurl

import "testing"

func TestID(t *testing.T) {
	t.Parallel()
	for raw, want := range map[string]int{
		"https://www.nexusmods.com/stardewvalley/mods/1915":      1915,
		"https://www.nexusmods.com/stardewvalley/Mods/1915/?tab": 1915,
		"https://www.nexusmods.com/stardewvalley/mods/0":         0,
		"https://www.nexusmods.com/stardewvalley/mods/x":         0,
		"":                                                       0,
		"://bad":                                                 0,
	} {
		if got := ID(raw); got != want {
			t.Errorf("ID(%q) = %d, want %d", raw, got, want)
		}
	}
}
