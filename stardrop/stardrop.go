// Package stardrop reads Stardrop's profile files and settings. It never writes them.
package stardrop

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Rethunk-Tech/go-modmanagers/internal/readcap"

	"github.com/Rethunk-Tech/go-modmanagers/internal/nexusurl"
)

// DataDir is <appData>/Stardrop/Data, where .NET's ApplicationData puts it (%APPDATA% on Windows, the XDG
// config home elsewhere).
func DataDir(appData string) string { return filepath.Join(appData, "Stardrop", "Data") }

// Profile is one Stardrop profile file.
type Profile struct {
	Name string
	// EnabledModIDs holds a bare id or the UniqueId of an {UniqueId, SourceId} object, trimmed.
	EnabledModIDs []string
	ModData       []ModData
	// PreservedModConfigs is each mod's config.json kept for this profile, keyed as Stardrop wrote the key.
	PreservedModConfigs map[string]json.RawMessage
}

// ModData is the record Stardrop keeps for a mod it listed.
type ModData struct {
	UniqueID, Version, Name, ModPageURI string
}

// NexusID is the id in ModPageURI, 0 when none.
func (m ModData) NexusID() int { return nexusurl.ID(m.ModPageURI) }

type modRef string

// UnmarshalJSON accepts a bare mod id or an object that also names the collection that installed the mod.
func (r *modRef) UnmarshalJSON(b []byte) error {
	var id string
	if json.Unmarshal(b, &id) == nil {
		*r = modRef(strings.TrimSpace(id))
		return nil
	}
	var ref struct {
		ID string `json:"UniqueId"`
	}
	if err := json.Unmarshal(b, &ref); err != nil {
		return err
	}
	*r = modRef(strings.TrimSpace(ref.ID))
	return nil
}

type wireModData struct {
	ID         string `json:"UniqueId"`
	Version    string `json:"Version"`
	Name       string `json:"Name"`
	ModPageURI string `json:"ModPageUri"`
}

type wireProfile struct {
	Name                string                     `json:"Name"`
	EnabledModIDs       []modRef                   `json:"EnabledModIds"`
	ModData             []wireModData              `json:"ModData"`
	PreservedModConfigs map[string]json.RawMessage `json:"PreservedModConfigs"`
}

// ProfileIDs lists the *.json files (extension case-insensitive) in <dataDir>/Profiles, by file stem. A missing
// folder has none.
func ProfileIDs(dataDir string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(dataDir, "Profiles"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".json") {
			continue
		}
		out = append(out, strings.TrimSuffix(e.Name(), filepath.Ext(e.Name())))
	}
	return out, nil
}

// ReadProfile reads <dataDir>/Profiles/<id>: the *.json file with that stem, else id as a file name (".json" added when id has no extension). The name is trimmed and
// falls back to the file stem.
func ReadProfile(dataDir, id string) (Profile, error) {
	if !readcap.PlainName(id) {
		return Profile{}, fmt.Errorf("stardrop profile id %q is not a file name", id)
	}
	dir := filepath.Join(dataDir, "Profiles")
	// ProfileIDs hands back stems, and a stem such as "v1.2" has an extension of its own, so the folder is
	// searched for a *.json file with that stem before id is taken as a literal file name.
	name := id
	if filepath.Ext(id) == "" {
		name = id + ".json"
	}
	if entries, err := os.ReadDir(dir); err == nil {
		for _, e := range entries {
			ext := filepath.Ext(e.Name())
			if !e.IsDir() && strings.EqualFold(ext, ".json") && strings.TrimSuffix(e.Name(), ext) == id {
				name = e.Name()
				break
			}
		}
	}
	path := filepath.Join(dir, name)
	data, err := readcap.File(path, readcap.MaxFile)
	if err != nil {
		return Profile{}, err
	}
	return parseProfile(data, strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
}

func parseProfile(data []byte, stem string) (Profile, error) {
	var w wireProfile
	if err := json.Unmarshal(data, &w); err != nil {
		return Profile{}, err
	}
	p := Profile{Name: cmp.Or(strings.TrimSpace(w.Name), stem), PreservedModConfigs: w.PreservedModConfigs}
	for _, r := range w.EnabledModIDs {
		p.EnabledModIDs = append(p.EnabledModIDs, string(r))
	}
	for _, m := range w.ModData {
		p.ModData = append(p.ModData, ModData{m.ID, m.Version, m.Name, m.ModPageURI})
	}
	return p, nil
}

// ModsFolder is Settings.json ModFolderPath, else <SMAPIFolderPath>/Mods, else fallback, cleaned. A relative setting
// is relative to dataDir, not to the caller's working directory.
func ModsFolder(dataDir, fallback string) string {
	path := fallback
	data, err := readcap.File(filepath.Join(dataDir, "Settings.json"), readcap.MaxFile)
	if err == nil {
		var s struct {
			ModFolderPath   string `json:"ModFolderPath"`
			SMAPIFolderPath string `json:"SMAPIFolderPath"`
		}
		if json.Unmarshal(data, &s) == nil {
			if s.ModFolderPath != "" {
				path = s.ModFolderPath
			} else if s.SMAPIFolderPath != "" {
				path = filepath.Join(s.SMAPIFolderPath, "Mods")
			}
			if path != fallback && !filepath.IsAbs(path) {
				path = filepath.Join(dataDir, path)
			}
		}
	}
	return filepath.Clean(path)
}
