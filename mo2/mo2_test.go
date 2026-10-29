package mo2

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Rethunk-Tech/go-modmanagers/internal/testwrite"
)

func write(t *testing.T, path, body string) { testwrite.File(t, path, body) }

func TestOpenFollowsSettings(t *testing.T) {
	t.Parallel()
	t.Run("defaults and game name", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		write(t, filepath.Join(dir, "ModOrganizer.ini"), "\xef\xbb\xbf[General]\r\ngameName=Stardew Valley\r\n; c\r\n[Settings]\r\n")
		in, err := Open(dir)
		if err != nil || in.GameName != "Stardew Valley" || in.Mods != filepath.Join(dir, "mods") || in.Profiles != filepath.Join(dir, "profiles") {
			t.Fatalf("instance = %+v, %v", in, err)
		}
	})
	t.Run("moved base", func(t *testing.T) {
		t.Parallel()
		dir, base := t.TempDir(), filepath.Join(t.TempDir(), "MO2 data")
		write(t, filepath.Join(dir, "ModOrganizer.ini"), "[Settings]\nbase_directory=@ByteArray("+filepath.ToSlash(base)+")\n")
		in, err := Open(dir)
		if err != nil || in.Mods != filepath.Join(base, "mods") || in.Profiles != filepath.Join(base, "profiles") {
			t.Fatalf("instance = %+v, %v", in, err)
		}
	})
	t.Run("moved mod_directory in a quoted escaped value", func(t *testing.T) {
		t.Parallel()
		dir, mods := t.TempDir(), filepath.Join(t.TempDir(), "staged")
		write(t, filepath.Join(dir, "ModOrganizer.ini"),
			"[Settings]\nmod_directory=\""+strings.ReplaceAll(filepath.ToSlash(mods), "/", `\\`)+"\"\n")
		if in, err := Open(dir); err != nil || in.Mods != mods {
			t.Fatalf("mods = %q want %q, %v", in.Mods, mods, err)
		}
	})
	t.Run("BASE_DIR expands", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		write(t, filepath.Join(dir, "ModOrganizer.ini"), "[Settings]\nprofiles_directory=%BASE_DIR%/prof\n")
		if in, err := Open(dir); err != nil || in.Profiles != filepath.Join(dir, "prof") {
			t.Fatalf("profiles = %q, %v", in.Profiles, err)
		}
	})
	t.Run("relative folders resolve against the instance", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		write(t, filepath.Join(dir, "ModOrganizer.ini"), "[Settings]\nbase_directory=data\nmod_directory=staged\n")
		in, err := Open(dir)
		if err != nil || in.Mods != filepath.Join(dir, "staged") || in.Profiles != filepath.Join(dir, "data", "profiles") {
			t.Fatalf("instance = %+v, %v", in, err)
		}
	})
	t.Run("a missing ini is an error", func(t *testing.T) {
		t.Parallel()
		if _, err := Open(t.TempDir()); err == nil {
			t.Fatal("no error")
		}
	})
}

func TestModlist(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	write(t, filepath.Join(dir, "ModOrganizer.ini"), "[General]\n")
	write(t, filepath.Join(dir, "profiles", "Default", "modlist.txt"),
		"\xef\xbb\xbf# generated\r\n*DLC\r\n-Graphics_separator\r\n-SomeLocal\r\n+ContentPatcher\r\n\r\n+SomeLocal\r\n-SomeLocal\r\nx\r\n+\r\n")
	in, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	names, err := in.ProfileNames()
	if err != nil || !slices.Equal(names, []string{"Default"}) {
		t.Fatalf("names = %v, %v", names, err)
	}
	got, err := in.ReadModlist("Default")
	want := []Entry{{"SomeLocal", false}, {"ContentPatcher", true}}
	if err != nil || !slices.Equal(got, want) {
		t.Fatalf("modlist = %v, %v", got, err)
	}
	if _, err := in.ReadModlist("Nope"); err == nil {
		t.Fatal("a missing modlist was read")
	}
}

func TestInstancesAndMeta(t *testing.T) {
	t.Parallel()
	local := t.TempDir()
	if got, err := Instances(local); err != nil || got != nil {
		t.Fatalf("missing: %v, %v", got, err)
	}
	if err := os.MkdirAll(filepath.Join(local, "ModOrganizer", "Inst"), 0o750); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(local, "ModOrganizer", "stray.txt"), "x")
	if got, err := Instances(local); err != nil || !slices.Equal(got, []string{filepath.Join(local, "ModOrganizer", "Inst")}) {
		t.Fatalf("instances = %v, %v", got, err)
	}
	mod := filepath.Join(local, "m")
	write(t, filepath.Join(mod, "meta.ini"), "[General]\r\nmodid=1915\r\nversion=2.0.0\r\n")
	if got := ReadMeta(mod); got != (Meta{NexusID: 1915, Version: "2.0.0"}) {
		t.Fatalf("meta = %+v", got)
	}
	write(t, filepath.Join(mod, "meta.ini"), "[General]\nmodid=0\nurl=https://www.nexusmods.com/stardewvalley/mods/77\n")
	if got := ReadMeta(mod); got.NexusID != 77 {
		t.Fatalf("meta from url = %+v", got)
	}
	if got := ReadMeta(filepath.Join(local, "none")); got != (Meta{}) {
		t.Fatalf("missing meta = %+v", got)
	}
}

func FuzzModlist(f *testing.F) {
	f.Add([]byte("# c\n*DLC\n+A\n-B\n+A\n-x_separator\n"))
	f.Add([]byte("\r\n+\r\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		seen := map[string]bool{}
		for _, e := range ParseModlist(data) {
			if e.Name == "" || seen[e.Name] || strings.HasPrefix(e.Name, "*") || strings.HasSuffix(e.Name, "_separator") {
				t.Fatalf("bad entry %+v", e)
			}
			seen[e.Name] = true
		}
	})
}

func FuzzOpen(f *testing.F) {
	f.Add([]byte("[Settings]\nbase_directory=@ByteArray(x)\nmod_directory=\"a\\\\b\"\n"))
	f.Add([]byte("[Settings]\nmod_directory=\\"))
	dir := f.TempDir()
	f.Fuzz(func(t *testing.T, data []byte) {
		in := parseInstance(dir, data)
		if in.Mods == "" || in.Profiles == "" || !filepath.IsAbs(in.Mods) || !filepath.IsAbs(in.Profiles) {
			t.Fatalf("instance = %+v", in)
		}
	})
}
