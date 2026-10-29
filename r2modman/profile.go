package r2modman

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/Rethunk-Tech/go-desktopenv/userdirs"
	"github.com/Rethunk-Tech/go-modmanagers/internal/managerbase"
	safefs "github.com/Rethunk-Tech/go-safefs"
	"go.yaml.in/yaml/v3"
)

// IsProfileFolder reports a folder holding a mods.yml file.
func IsProfileFolder(dir string) bool {
	st, err := os.Stat(filepath.Join(dir, "mods.yml"))
	return err == nil && !st.IsDir()
}

// Profile is an r2modman profile folder, <data>/<GameFolder>/profiles/<Name>.
type Profile struct {
	Name, GameFolder string // the folder's base, and the grandparent folder's base
	Mods             []Mod
}

// ReadProfile reads <dir>/mods.yml, a top-level list, at most 16 MiB (a larger one wraps safefs.ErrTooLarge).
func ReadProfile(dir string) (Profile, error) {
	raw, err := readCapped(filepath.Join(dir, "mods.yml"))
	if err != nil {
		return Profile{}, err
	}
	var ws []wireMod
	if err := yaml.Unmarshal(raw, &ws); err != nil {
		return Profile{}, fmt.Errorf("reading mods.yml: %w", err)
	}
	return Profile{
		Name:       filepath.Base(dir),
		GameFolder: filepath.Base(filepath.Dir(filepath.Dir(dir))),
		Mods:       mods(ws),
	}, nil
}

// File is a file read from a profile folder.
type File struct {
	Path string // slash-separated
	Data []byte
}

// Configs reads every regular file below <profileDir>/BepInEx/config as "config/<rel>" (slash-separated), each at
// most 16 MiB (a larger one wraps safefs.ErrTooLarge); none when the folder is missing. Gale profile folders share
// the layout.
func Configs(profileDir string) ([]File, error) {
	cfg := filepath.Join(profileDir, "BepInEx", "config")
	var out []File
	err := filepath.WalkDir(cfg, func(p string, e fs.DirEntry, err error) error {
		if err != nil {
			if p == cfg && errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if !e.Type().IsRegular() {
			return nil
		}
		b, err := readCapped(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(cfg, p)
		out = append(out, File{Path: "config/" + filepath.ToSlash(rel), Data: b})
		return nil
	})
	return out, err
}

// readCapped reads a file of at most 16 MiB, naming it in the error of a larger one.
func readCapped(file string) ([]byte, error) {
	f, err := safefs.Open(file)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	b, err := safefs.ReadCapped(f, maxFile)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(file), err)
	}
	return b, nil
}

// Bases is the folder r2modman keeps its data under, per OS: on Linux the host's XDG config home (the host's
// even inside a Flatpak, never the sandbox's own), on Windows %AppData%, elsewhere the user config directory.
func Bases() []string { return managerbase.Bases(userdirs.HostConfigHome) }

// DataDirs lists, of <base>/r2modmanPlus-local and <base>/Thunderstore Mod Manager/DataFolder for each base,
// those that exist, without repeats.
func DataDirs(bases ...string) []string {
	var out []string
	for _, base := range bases {
		for _, p := range []string{
			filepath.Join(base, "r2modmanPlus-local"),
			filepath.Join(base, "Thunderstore Mod Manager", "DataFolder"),
		} {
			if safefs.IsDir(p) && !slices.Contains(out, p) {
				out = append(out, p)
			}
		}
	}
	return out
}

// IsDataFolder reports a folder holding <game>/profiles folders.
func IsDataFolder(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	return slices.ContainsFunc(entries, func(e os.DirEntry) bool { return safefs.IsDir(filepath.Join(dir, e.Name(), "profiles")) })
}
