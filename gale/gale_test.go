package gale

import (
	"database/sql"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"

	httpkit "github.com/Rethunk-Tech/go-httpkit"
	safefs "github.com/Rethunk-Tech/go-safefs"
)

// buildDB makes a database whose profiles table has the columns Gale's migrations add beyond the four read.
func buildDB(t *testing.T, dir string, rows ...string) string {
	t.Helper()
	file := Database(dir)
	db, err := sql.Open("sqlite", file)
	if err != nil {
		t.Fatal(err)
	}
	stmts := append([]string{
		`CREATE TABLE profiles (id INTEGER PRIMARY KEY, name TEXT NOT NULL, path TEXT NOT NULL, game_slug TEXT NOT NULL, mods JSON NOT NULL,
			modpack JSON, ignored_updates JSON, sync_data JSON, custom_args TEXT, custom_args_enabled INTEGER DEFAULT 0, excluded_export_files JSON)`,
	}, rows...)
	for _, q := range stmts {
		if _, err := db.ExecContext(t.Context(), q); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	return file
}

func TestProfiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	profile := filepath.Join(dir, "lethal-company", "profiles", "Crew")
	moved := filepath.Join(dir, "lethal-company", "profiles", "Moved")
	for _, p := range []string{profile, moved} {
		if err := os.MkdirAll(p, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	file := buildDB(t, dir,
		`INSERT INTO profiles (name, path, game_slug, mods, custom_args) VALUES ('Crew', '`+strings.ReplaceAll(profile, "'", "''")+`', 'lethal-company', '[
			{"enabled":true,"fullName":"BepInEx-BepInExPack-5.4.2100","packageUuid":"u","versionUuid":"v"},
			{"enabled":false,"fullName":"Alice-MoreCompany-1.2.3"},
			{"name":"Mine","author":"Me","version":"0.1.0","dependencies":["A-B-1.0.0"],"uuid":"x"}]', '--x')`,
		`INSERT INTO profiles (name, path, game_slug, mods) VALUES ('Other', '/elsewhere', 'risk-of-rain-2', '[')`,
		// Recorded at a host path this process cannot reach, as a Flatpak sees a data folder opened through the portal.
		`INSERT INTO profiles (name, path, game_slug, mods) VALUES ('Moved', '/mnt/gone/lethal-company/profiles/Moved', 'lethal-company', '[]')`,
	)
	got, err := Profiles(t.Context(), file)
	if err != nil || len(got) != 3 {
		t.Fatalf("profiles = %+v, %v", got, err)
	}
	if got[0].Name != "Crew" || got[0].Slug != "lethal-company" || got[0].Path != profile {
		t.Fatalf("crew = %+v", got[0])
	}
	if got[2].Path != moved || got[1].Path != "/elsewhere" {
		t.Fatalf("paths: moved %q, other %q", got[2].Path, got[1].Path)
	}
	mods, err := got[0].Mods()
	want := []Mod{
		{FullName: "BepInEx-BepInExPack-5.4.2100", Enabled: true},
		{FullName: "Alice-MoreCompany-1.2.3"},
		{Name: "Mine", Author: "Me", Version: "0.1.0", Dependencies: []string{"A-B-1.0.0"}, Enabled: true},
	}
	if err != nil || len(mods) != 3 || mods[0].FullName != want[0].FullName || !mods[0].Enabled || mods[1].Enabled ||
		mods[2].Name != "Mine" || !mods[2].Enabled || !slices.Equal(mods[2].Dependencies, want[2].Dependencies) {
		t.Fatalf("mods = %+v, %v", mods, err)
	}
	if _, err := got[1].Mods(); err == nil {
		t.Fatal("one bad row's mods parsed")
	}
}

func TestProfilesOfAMissingDatabaseListNothing(t *testing.T) {
	t.Parallel()
	for _, db := range []string{"", filepath.Join(t.TempDir(), "none.sqlite3")} {
		if got, err := Profiles(t.Context(), db); got != nil || err != nil {
			t.Fatalf("Profiles(%q) = %v, %v", db, got, err)
		}
	}
	notADB := filepath.Join(t.TempDir(), "x")
	if err := os.WriteFile(notADB, []byte("not sqlite"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Profiles(t.Context(), notADB); err == nil {
		t.Fatal("a non-database was read")
	}
}

func TestReadOnlyURIGivesWindowsDrivesALeadingSlash(t *testing.T) {
	t.Parallel()
	if got := readOnlyURI("C:/Users/a b/data.sqlite3"); got != "file:///C:/Users/a%20b/data.sqlite3?mode=ro&immutable=1" {
		t.Fatalf("uri = %q", got)
	}
}

func TestCodeID(t *testing.T) {
	t.Parallel()
	const legacy = "ASNFZ4mrze8BI0VniavN7w"
	for text, want := range map[string]string{
		"E5RG8F": "E5RG8F", "e5rg8f": "E5RG8F", legacy: legacy,
		"https://gale.kesomannen.com/api/desktop/profile/sync/clone/E5RG8F": "E5RG8F",
		"gale://profile/sync/clone/e5rg8f/":                                 "E5RG8F",
		"gale://profile/sync/clone/" + legacy:                               legacy,
		"E5RG8":                                                             "", "E5RG8FX": "", "E5RG-8": "", "hello": "", "": "", "#r2modman": "",
		"0123abcd-0123-0123-0123-0123456789ab": "",
		"ASNFZ4mrze8BI0VniavN7":                "", legacy + "=": "",
		"https://gale.kesomannen.com/api/desktop/profile/sync/clone/hello": "",
		"https://example.com/E5RG8F":                                       "",
	} {
		id, ok := CodeID(text)
		if id != want || ok != (want != "") {
			t.Errorf("CodeID(%q) = %q, %v", text, id, ok)
		}
	}
}

func TestCodeIDTrimsPastedWhitespace(t *testing.T) {
	t.Parallel()
	if id, ok := CodeID(" e5rg8f\n"); !ok || id != "E5RG8F" {
		t.Fatalf("CodeID = %q, %v", id, ok)
	}
}

func TestFetchCodeFollowsTheRedirect(t *testing.T) {
	t.Parallel()
	var agent string
	mux := http.NewServeMux()
	mux.HandleFunc("/profile/E5RG8F", func(w http.ResponseWriter, r *http.Request) {
		agent = r.UserAgent()
		http.Redirect(w, r, "/cdn/p", http.StatusSeeOther)
	})
	mux.HandleFunc("/cdn/p", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("zip-bytes")) })
	mux.HandleFunc("/profile/LIMITD", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTooManyRequests) })
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	c := Client{BaseURL: srv.URL, UserAgent: "test-agent"}
	if b, err := c.FetchCode(t.Context(), "E5RG8F"); err != nil || string(b) != "zip-bytes" || agent != "test-agent" {
		t.Fatalf("got %q, %v, agent %q", b, err, agent)
	}
	if _, err := c.FetchCode(t.Context(), "ZZZZZZ"); !errors.Is(err, httpkit.ErrNotFound) {
		t.Errorf("unknown code: %v", err)
	}
	if _, err := c.FetchCode(t.Context(), "LIMITD"); !errors.Is(err, httpkit.ErrRateLimited) {
		t.Errorf("rate limit: %v", err)
	}
	c.MaxBytes = 4
	if _, err := c.FetchCode(t.Context(), "E5RG8F"); !errors.Is(err, safefs.ErrTooLarge) {
		t.Errorf("cap: %v", err)
	}
}

func TestDataDirsAndHasDatabase(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	if got := DataDirs(base); got != nil || HasDatabase(filepath.Join(base, AppID)) {
		t.Fatalf("empty base: %v", got)
	}
	dir := filepath.Join(base, AppID)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if got := DataDirs(base, base); !slices.Equal(got, []string{dir}) || HasDatabase(dir) {
		t.Fatalf("dirs = %v", got)
	}
	buildDB(t, dir)
	if !HasDatabase(dir) {
		t.Fatal("database not seen")
	}
}

// The host's data home is listed; the Flatpak sandbox's own never is, and a relative HOST_XDG value is ignored.
func TestBasesFollowTheHostXDGHome(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the host XDG homes are a Flatpak sandbox escape, which Linux alone has")
	}
	home, host := t.TempDir(), t.TempDir()
	hostGale := filepath.Join(host, "data", AppID)
	sandboxGale := filepath.Join(home, ".var", "app", "x", "data", AppID)
	for _, d := range []string{hostGale, sandboxGale} {
		if err := os.MkdirAll(d, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".var", "app", "x", "data"))
	t.Setenv("FLATPAK_ID", "org.example.App")
	t.Setenv("HOST_XDG_DATA_HOME", filepath.Join(host, "data"))
	if got := DataDirs(Bases()...); !slices.Equal(got, []string{hostGale}) {
		t.Fatalf("dirs %v: want only the host's %s", got, hostGale)
	}
	t.Setenv("HOST_XDG_DATA_HOME", "relative")
	if got := DataDirs(Bases()...); slices.Contains(got, hostGale) {
		t.Fatalf("relative HOST_XDG_DATA_HOME honoured: %v", got)
	}
}

func FuzzCodeID(f *testing.F) {
	f.Add("E5RG8F")
	f.Add("gale://profile/sync/clone/e5rg8f/")
	f.Add("ASNFZ4mrze8BI0VniavN7w")
	f.Add("")
	sixUpper := regexp.MustCompile(`^[A-Z0-9]{6}$`)
	f.Fuzz(func(t *testing.T, text string) {
		id, ok := CodeID(text)
		if !ok {
			if id != "" {
				t.Fatalf("id %q without ok", id)
			}
			return
		}
		if b, err := base64.RawURLEncoding.DecodeString(id); !sixUpper.MatchString(id) && (err != nil || len(id) != 22 || len(b) != 16) {
			t.Fatalf("id %q is neither shape", id)
		}
	})
}

func FuzzMods(f *testing.F) {
	f.Add(`[{"enabled":true,"fullName":"A-B-1.0.0"},{"name":"Mine"}]`)
	f.Add(`[`)
	f.Add(`[{"fullName":"-"},{"enabled":null,"dependencies":[1]}]`)
	f.Fuzz(func(t *testing.T, mods string) {
		if _, err := (Profile{mods: mods}).Mods(); err != nil && !strings.Contains(err.Error(), "Gale profile") {
			t.Fatalf("unwrapped error: %v", err)
		}
	})
}
