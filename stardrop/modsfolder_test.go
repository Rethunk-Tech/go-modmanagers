package stardrop

import (
	"os"
	"path/filepath"
	"testing"
)

func TestModsFolderJoinsRelativeSettingEqualToFallback(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dataDir, "Settings.json"), []byte(`{"ModFolderPath":"Mods"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got := ModsFolder(dataDir, "Mods")
	want := filepath.Join(dataDir, "Mods")
	if got != want {
		t.Fatalf("ModsFolder = %q, want %q", got, want)
	}
}
