package r2modman

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/Rethunk-Tech/go-modmanagers/internal/testwrite"

	archive "github.com/Rethunk-Tech/go-archive"
	httpkit "github.com/Rethunk-Tech/go-httpkit"
	safefs "github.com/Rethunk-Tech/go-safefs"
)

// entry is one zip entry; mode is applied when nonzero.
type entry struct {
	name, body string
	mode       fs.FileMode
}

func buildZip(t testing.TB, entries ...entry) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		h := &zip.FileHeader{Name: e.name, Method: zip.Deflate}
		if e.mode != 0 {
			h.SetMode(e.mode)
		}
		w, err := zw.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(e.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func files(t testing.TB, kv map[string]string) []byte {
	var es []entry
	for k, v := range kv {
		es = append(es, entry{name: k, body: v})
	}
	return buildZip(t, es...)
}

// A real r2modman file carries more keys per mod than the reader names; they must not matter.
const export = `profileName: Friends
community: lethal-company
extra: {a: [1, 2], b: x}
mods:
  - name: BepInEx-BepInExPack
    version: {major: 5, minor: 4, patch: 2100}
    enabled: true
    icon: icon.png
    tags: [a, b]
    nested: {k: v}
  - name: Alice-MoreCompany
    version: {major: 1, minor: 2, patch: 3}
    enabled: false
  - name: Bob-NoEnabledKey
    version: {major: 0, minor: 0, patch: 1}
`

// yamlMergeCrash panicked gopkg.in/yaml.v3 v3.0.1: a merge uuid beside an unhashable uuid.
const yamlMergeCrash = "base: &b {name: A-B}\nmods:\n- <<: *b\n  ? [x]\n  : 1\n"

func TestReadExport(t *testing.T) {
	t.Parallel()
	got, err := ReadExport(files(t, map[string]string{exportFile: export, "config/a.cfg": "x=1", "BepInEx/plugins/note.txt": "hi"}))
	if err != nil {
		t.Fatal(err)
	}
	want := []Mod{
		{"BepInEx-BepInExPack", Version{5, 4, 2100}, true},
		{"Alice-MoreCompany", Version{1, 2, 3}, false},
		{"Bob-NoEnabledKey", Version{0, 0, 1}, true},
	}
	if got.ProfileName != "Friends" || got.Community != "lethal-company" || !slices.Equal(got.Mods, want) {
		t.Fatalf("export = %+v", got)
	}
	if len(got.Files) != 2 || string(got.Files["config/a.cfg"]) != "x=1" || got.Files["BepInEx/plugins/note.txt"] == nil {
		t.Fatalf("files = %v", got.Files)
	}
	if want[0].Version.String() != "5.4.2100" {
		t.Fatal("Version.String")
	}
}

func TestReadExportRefusesHostileZips(t *testing.T) {
	t.Parallel()
	ok := entry{name: exportFile, body: export}
	var many []entry
	for i := range maxEntries + 1 {
		many = append(many, entry{name: fmt.Sprintf("f%d", i)})
	}
	cases := map[string][]byte{
		"traversal":           buildZip(t, ok, entry{name: "../evil", body: "x"}),
		"absolute":            buildZip(t, ok, entry{name: "/etc/x", body: "x"}),
		"drive letter":        buildZip(t, ok, entry{name: "C:/x", body: "x"}),
		"colon in a name":     buildZip(t, ok, entry{name: "a/b:c", body: "x"}),
		"symlink entry":       buildZip(t, ok, entry{name: "link", body: "/etc/passwd", mode: fs.ModeSymlink | 0o777}),
		"device entry":        buildZip(t, ok, entry{name: "dev", mode: fs.ModeDevice | 0o600}),
		"case-only collision": buildZip(t, ok, entry{name: "Sub/a.cfg", body: "1"}, entry{name: "sub/b.cfg", body: "2"}),
		"exact duplicate":     buildZip(t, ok, entry{name: "a.cfg", body: "1"}, entry{name: "a.cfg", body: "2"}),
		"too many entries":    buildZip(t, append(many, ok)...),
		"file over the cap":   buildZip(t, ok, entry{name: "big.bin", body: strings.Repeat("x", maxFile+1)}),
		"no export.r2x":       buildZip(t, entry{name: "config/a.cfg", body: "x"}),
		"not a zip":           []byte("hello"),
		"a tar, not a zip":    append([]byte("ustar"), make([]byte, 600)...),
		"bad yaml":            buildZip(t, entry{name: exportFile, body: ":\n- ["}),
	}
	for name, data := range cases {
		if _, err := ReadExport(data); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if _, err := ReadExport(buildZip(t, ok, entry{name: "big.bin", body: strings.Repeat("x", maxFile+1)})); !errors.Is(err, archive.ErrEntryTooLarge) {
		t.Errorf("oversize file error = %v", err)
	}
}

func TestReadExportMergeKeyCrash(t *testing.T) {
	t.Parallel()
	if _, err := ReadExport(files(t, map[string]string{exportFile: yamlMergeCrash})); err == nil {
		t.Fatal("want an error")
	}
}

func TestCodes(t *testing.T) {
	t.Parallel()
	r2z := files(t, map[string]string{exportFile: export})
	wrapped := base64.StdEncoding.EncodeToString(r2z)
	wrapped = wrapped[:20] + "\n  " + wrapped[20:40] + "\r\n" + wrapped[40:]
	got, err := DecodeCode(" " + CodePrefix + "\n" + wrapped + "\n")
	if err != nil || !bytes.Equal(got, r2z) {
		t.Fatalf("decode: %v", err)
	}
	for _, bad := range []string{"hello", CodePrefix + "!!!"} {
		if _, err := DecodeCode(bad); err == nil {
			t.Errorf("DecodeCode(%q) accepted", bad)
		}
	}
	const uuid = "0123abcd-0123-0123-0123-0123456789AB"
	for text, want := range map[string]bool{
		uuid: true, " " + uuid + "\n": true, "gale://profile/import/" + uuid: true, "gale://profile/import/" + uuid + "/": true,
		"gale://profile/import/E5RG8F": false, "https://x/" + uuid: false, "hello": false, "": false,
	} {
		if got := IsCodeKey(text); got != want {
			t.Errorf("IsCodeKey(%q) = %v", text, got)
		}
	}
	if k, _ := CodeKey("gale://profile/import/" + uuid + "/"); k != uuid {
		t.Errorf("CodeKey = %q", k)
	}
	if !IsCode("  "+CodePrefix+"\nabc") || IsCode(uuid) {
		t.Error("IsCode")
	}
}

func TestFetchCode(t *testing.T) {
	t.Parallel()
	const uuid = "0123abcd-0123-0123-0123-0123456789ab"
	var agent string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		agent = r.UserAgent()
		switch r.URL.Path {
		case "/api/experimental/legacyprofile/get/" + uuid + "/":
			_, _ = w.Write([]byte(CodePrefix + "\nAAAA"))
		case "/api/experimental/legacyprofile/get/limited/":
			w.WriteHeader(http.StatusTooManyRequests)
		case "/api/experimental/legacyprofile/get/boom/":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	c := Client{BaseURL: srv.URL, UserAgent: "test-agent"}
	if got, err := c.FetchCode(t.Context(), uuid); err != nil || got != CodePrefix+"\nAAAA" || agent != "test-agent" {
		t.Fatalf("got %q, %v, agent %q", got, err, agent)
	}
	if _, err := c.FetchCode(t.Context(), "missing"); !errors.Is(err, httpkit.ErrNotFound) {
		t.Errorf("404: %v", err)
	}
	if _, err := c.FetchCode(t.Context(), "limited"); !errors.Is(err, httpkit.ErrRateLimited) {
		t.Errorf("429: %v", err)
	}
	var se *httpkit.StatusError
	if _, err := c.FetchCode(t.Context(), "boom"); !errors.As(err, &se) || se.Code != http.StatusInternalServerError {
		t.Errorf("500: %v", err)
	}
	c.MaxBytes = 4
	if _, err := c.FetchCode(t.Context(), uuid); !errors.Is(err, safefs.ErrTooLarge) {
		t.Errorf("cap: %v", err)
	}
}

func write(t *testing.T, path, body string) { testwrite.File(t, path, body) }

func TestProfileFolder(t *testing.T) {
	t.Parallel()
	data := t.TempDir()
	dir := filepath.Join(data, "LethalCompany", "profiles", "Crew")
	if IsProfileFolder(dir) || IsDataFolder(data) {
		t.Fatal("empty tree detected")
	}
	write(t, filepath.Join(dir, "mods.yml"), `- name: A-B
  versionNumber: {major: 1, minor: 2, patch: 3}
  enabled: false
  extraList: [1, 2]
  extraMap: {k: v}
  extraScalar: s
- name: C-D
  version: {major: 4, minor: 0, patch: 0}
  versionNumber: {major: 9, minor: 9, patch: 9}
`)
	write(t, filepath.Join(dir, "BepInEx", "config", "a.cfg"), "x")
	write(t, filepath.Join(dir, "BepInEx", "config", "sub", "b.cfg"), "y")
	if !IsProfileFolder(dir) || !IsDataFolder(data) {
		t.Fatal("tree not detected")
	}
	p, err := ReadProfile(dir)
	want := []Mod{{"A-B", Version{1, 2, 3}, false}, {"C-D", Version{4, 0, 0}, true}}
	if err != nil || p.Name != "Crew" || p.GameFolder != "LethalCompany" || !slices.Equal(p.Mods, want) {
		t.Fatalf("profile = %+v, %v", p, err)
	}
	cfgs, err := Configs(dir)
	slices.SortFunc(cfgs, func(a, b File) int { return strings.Compare(a.Path, b.Path) })
	if err != nil || len(cfgs) != 2 || cfgs[0].Path != "config/a.cfg" || cfgs[1].Path != "config/sub/b.cfg" || string(cfgs[1].Data) != "y" {
		t.Fatalf("configs = %+v, %v", cfgs, err)
	}
	if none, err := Configs(t.TempDir()); err != nil || none != nil {
		t.Fatalf("missing configs = %v, %v", none, err)
	}
}

func TestReadProfileCapsAndErrors(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if _, err := ReadProfile(dir); err == nil {
		t.Fatal("missing mods.yml")
	}
	write(t, filepath.Join(dir, "mods.yml"), "# "+strings.Repeat("x", maxFile))
	if _, err := ReadProfile(dir); !errors.Is(err, safefs.ErrTooLarge) {
		t.Fatalf("oversize: %v", err)
	}
	write(t, filepath.Join(dir, "mods.yml"), yamlMergeCrash)
	if _, err := ReadProfile(dir); err == nil {
		t.Fatal("merge-uuid crash seed parsed")
	}
	write(t, filepath.Join(dir, "BepInEx", "config", "big.cfg"), strings.Repeat("x", maxFile+1))
	if _, err := Configs(dir); !errors.Is(err, safefs.ErrTooLarge) {
		t.Fatalf("oversize config: %v", err)
	}
}

func TestDataDirsListsExistingFoldersOnce(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	local := filepath.Join(base, "r2modmanPlus-local")
	tsmm := filepath.Join(base, "Thunderstore Mod Manager", "DataFolder")
	for _, d := range []string{local, tsmm} {
		if err := os.MkdirAll(d, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	if got := DataDirs(base, t.TempDir(), base); !slices.Equal(got, []string{local, tsmm}) {
		t.Fatalf("dirs = %v", got)
	}
}

// The host's folders are listed and the Flatpak sandbox's own never are; a relative HOST_XDG value is ignored.
func TestBasesFollowTheHostXDGHome(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the host XDG homes are a Flatpak sandbox escape, which Linux alone has")
	}
	home, host := t.TempDir(), t.TempDir()
	hostR2 := filepath.Join(host, "config", "r2modmanPlus-local")
	sandboxR2 := filepath.Join(home, ".var", "app", "x", "config", "r2modmanPlus-local")
	for _, d := range []string{hostR2, sandboxR2} {
		if err := os.MkdirAll(d, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".var", "app", "x", "config"))
	t.Setenv("FLATPAK_ID", "org.example.App")
	t.Setenv("HOST_XDG_CONFIG_HOME", filepath.Join(host, "config"))
	if got := DataDirs(Bases()...); !slices.Contains(got, hostR2) || slices.Contains(got, sandboxR2) {
		t.Fatalf("dirs %v: want the host's %s and not the sandbox's own", got, hostR2)
	}
	t.Setenv("HOST_XDG_CONFIG_HOME", "relative")
	if got := DataDirs(Bases()...); slices.Contains(got, hostR2) {
		t.Fatalf("relative HOST_XDG_CONFIG_HOME honoured: %v", got)
	}
}

func FuzzReadExport(f *testing.F) {
	f.Add(files(f, map[string]string{exportFile: export, "config/a.cfg": "x"}))
	f.Add(files(f, map[string]string{exportFile: ":\n- ["}))
	f.Add(files(f, map[string]string{"../evil": "x"}))
	f.Add(files(f, map[string]string{exportFile: yamlMergeCrash}))
	f.Add([]byte("PK"))
	f.Fuzz(func(t *testing.T, data []byte) {
		e, err := ReadExport(data)
		if err != nil {
			return
		}
		total := 0
		spelling := map[string]string{}
		for name, b := range e.Files {
			total += len(b)
			segs := strings.Split(name, "/")
			if strings.HasPrefix(name, "/") || strings.Contains(name, ":") || slices.Contains(segs, "..") {
				t.Fatalf("path escapes: %q", name)
			}
			for i := range segs {
				orig := strings.Join(segs[:i+1], "/")
				if prev, ok := spelling[strings.ToLower(orig)]; ok && prev != orig {
					t.Fatalf("%q and %q differ only by case", prev, orig)
				}
				spelling[strings.ToLower(orig)] = orig
			}
		}
		if total > maxTotal {
			t.Fatalf("total %d over the cap", total)
		}
	})
}

func FuzzDecodeCode(f *testing.F) {
	f.Add(CodePrefix + "\nAAAA")
	f.Add("hello")
	f.Add(CodePrefix + " not base64 !")
	f.Fuzz(func(t *testing.T, text string) {
		if _, err := DecodeCode(text); err == nil && !IsCode(text) {
			t.Fatal("decoded text without the prefix")
		}
		CodeKey(text)
	})
}

func FuzzModsYML(f *testing.F) {
	f.Add([]byte("- name: A-B\n  version: {major: 1, minor: 2, patch: 3}\n"))
	f.Add([]byte(yamlMergeCrash))
	f.Add([]byte("- [\n"))
	dir := f.TempDir()
	f.Fuzz(func(t *testing.T, data []byte) {
		write(t, filepath.Join(dir, "mods.yml"), string(data))
		p, err := ReadProfile(dir)
		if err != nil {
			return
		}
		for _, m := range p.Mods {
			if parts := strings.Split(m.Version.String(), "."); len(parts) != 3 {
				t.Fatalf("version %q", m.Version)
			}
		}
	})
}
