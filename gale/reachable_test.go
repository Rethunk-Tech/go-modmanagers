package gale

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func TestProfilesReachablePathAnchorsOnSlugThenProfiles(t *testing.T) {
	dataDir := t.TempDir()
	want := filepath.Join(dataDir, "lethal-company", "profiles", "Crew")
	if err := os.MkdirAll(want, 0o755); err != nil {
		t.Fatal(err)
	}
	stolen := filepath.Join(dataDir, "lethal-company", ".local", "share", "com.kesomannen.gale", "lethal-company", "profiles", "Crew")
	if err := os.MkdirAll(stolen, 0o755); err != nil {
		t.Fatal(err)
	}

	db := filepath.Join(dataDir, databaseFile)
	conn, err := sql.Open("sqlite", db)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(`CREATE TABLE profiles (id INTEGER PRIMARY KEY, name TEXT, path TEXT, game_slug TEXT, mods TEXT)`); err != nil {
		t.Fatal(err)
	}
	recorded := filepath.Join(t.TempDir(), "gale-slug", "lethal-company", ".local", "share", "com.kesomannen.gale", "lethal-company", "profiles", "Crew")
	if _, err := conn.Exec(`INSERT INTO profiles (name, path, game_slug, mods) VALUES (?, ?, ?, ?)`, "Crew", recorded, "lethal-company", "[]"); err != nil {
		t.Fatal(err)
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}

	got, err := Profiles(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d profiles, want 1", len(got))
	}
	if got[0].Path != want {
		t.Fatalf("Path = %q, want %q", got[0].Path, want)
	}
}
