// Package gale reads Gale's profile database (data.sqlite3) and its profile sync codes. It never writes them.
package gale

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Rethunk-Tech/go-desktopenv/userdirs"
	"github.com/Rethunk-Tech/go-modmanagers/internal/managerbase"
	safefs "github.com/Rethunk-Tech/go-safefs"
	_ "modernc.org/sqlite" // registers the "sqlite" driver Profiles opens
)

// AppID is the folder Gale keeps its data in below the user's data directory (Kesomannen/gale
// src-tauri/src/util/path.rs, APP_GUID and default_app_data_dir).
const AppID = "com.kesomannen.gale"

// databaseFile is Gale's database file (src-tauri/src/db/mod.rs:24).
const databaseFile = "data.sqlite3"

// Bases is the folder Gale's data directory sits in, per OS, as the dirs crate resolves it: on Linux the host's
// XDG data home (the host's even inside a Flatpak, never the sandbox's own), on Windows %AppData%, elsewhere the
// user config directory.
func Bases() []string { return managerbase.Bases(userdirs.HostDataHome) }

// DataDirs lists <base>/com.kesomannen.gale for each base, those that exist, without repeats.
func DataDirs(bases ...string) []string {
	var out []string
	for _, base := range bases {
		p := filepath.Join(base, AppID)
		if safefs.IsDir(p) && !slices.Contains(out, p) {
			out = append(out, p)
		}
	}
	return out
}

// HasDatabase reports <dir>/data.sqlite3 as a regular file.
func HasDatabase(dir string) bool { return safefs.IsFile(Database(dir)) }

// Database is <dir>/data.sqlite3.
func Database(dir string) string { return filepath.Join(dir, databaseFile) }

// Profile is one row of Gale's profiles table (migrations/01-initial/up.sql).
type Profile struct {
	Name, Path string
	Slug       string // game_slug, the game's Thunderstore community
	mods       string // the mods column, unparsed until Mods()
}

// Profiles reads `SELECT name, path, game_slug, mods FROM profiles ORDER BY id` from db, opened read-only and
// immutable so a running Gale is not disturbed (what it has not yet checkpointed from its write-ahead log is not
// seen). A missing db has none. Path is the folder as this process can reach it: the recorded path, else the same
// <slug>/... tail below the database's folder.
func Profiles(ctx context.Context, db string) ([]Profile, error) {
	if db == "" {
		return nil, nil
	}
	if _, err := safefs.Stat(db); err != nil {
		return nil, nil
	}
	conn, err := sql.Open("sqlite", readOnlyURI(db))
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()
	rows, err := conn.QueryContext(ctx, "SELECT name, path, game_slug, mods FROM profiles ORDER BY id")
	if err != nil {
		return nil, fmt.Errorf("reading Gale's database: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []Profile
	for rows.Next() {
		var p Profile
		if err := rows.Scan(&p.Name, &p.Path, &p.Slug, &p.mods); err != nil {
			return nil, err
		}
		p.Path = reachablePath(p.Path, p.Slug, filepath.Dir(db))
		out = append(out, p)
	}
	return out, rows.Err()
}

// readOnlyURI is SQLite's read-only, immutable URI for a database path. A Windows path ("C:/...") needs the
// leading slash too, or SQLite takes the drive for the URI's authority and refuses it.
func readOnlyURI(db string) string {
	p := filepath.ToSlash(db)
	if len(p) > 1 && p[1] == ':' {
		p = "/" + p
	}
	return (&url.URL{Scheme: "file", Path: p, RawQuery: "mode=ro&immutable=1"}).String()
}

// reachablePath is a profile's folder as this process can reach it. Gale records the absolute path it used, below
// <data dir>/<game slug>; when that path is not here (a Flatpak reaching the data folder through the document
// portal, or a data folder since moved) the same folder below the database's own folder is used.
func reachablePath(path, slug, dataDir string) string {
	if safefs.IsDir(path) {
		return path
	}
	parts := strings.FieldsFunc(path, func(r rune) bool { return r == '/' || r == '\\' })
	at := slices.Index(parts, slug)
	if at < 0 {
		return path
	}
	moved := filepath.Join(append([]string{dataDir}, parts[at:]...)...)
	if safefs.IsDir(moved) {
		return moved
	}
	return path
}

// Mod is one entry of a profile's mods JSON (src-tauri/src/profile/mod.rs ProfileMod, ThunderstoreMod, LocalMod).
type Mod struct {
	// FullName is Namespace-Name-Version for a Thunderstore mod; "" for one added from disk.
	FullName     string
	Enabled      bool // absent means enabled
	Name, Author string
	Version      string
	Dependencies []string
}

// Mods parses the profile's mods column. It is separate from Profiles so one bad row does not hide the others.
func (p Profile) Mods() ([]Mod, error) {
	var wire []struct {
		FullName     string   `json:"fullName"`
		Enabled      *bool    `json:"enabled"`
		Name         string   `json:"name"`
		Author       string   `json:"author"`
		Version      string   `json:"version"`
		Dependencies []string `json:"dependencies"`
	}
	if err := json.Unmarshal([]byte(p.mods), &wire); err != nil {
		return nil, fmt.Errorf("reading the mods of Gale profile %s: %w", p.Name, err)
	}
	out := make([]Mod, 0, len(wire))
	for _, w := range wire {
		out = append(out, Mod{
			FullName: w.FullName, Enabled: w.Enabled == nil || *w.Enabled,
			Name: w.Name, Author: w.Author, Version: w.Version, Dependencies: w.Dependencies,
		})
	}
	return out, nil
}
