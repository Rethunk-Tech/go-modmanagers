package readcap

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/Rethunk-Tech/go-safefs"
)

func FuzzPlainName(f *testing.F) {
	f.Add("ok")
	f.Add("..")
	f.Add("../x")
	f.Add("a/b")
	f.Add(`a\b`)
	f.Add("a:b")
	f.Add("")
	f.Add(".")
	f.Add("x.json:stream")
	f.Fuzz(func(t *testing.T, name string) {
		if !PlainName(name) {
			return
		}
		if filepath.Base(name) != name || name == "." || name == ".." {
			t.Fatalf("PlainName(%q) is not one path element", name)
		}
		root := string(filepath.Separator) + "root"
		rel, err := filepath.Rel(root, filepath.Join(root, name))
		if err != nil || !filepath.IsLocal(rel) {
			t.Fatalf("PlainName(%q) joins outside the root", name)
		}
	})
}

func TestFile(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, n int) string {
		t.Helper()
		p := filepath.Join(dir, name)
		b := make([]byte, n)
		for i := range b {
			b[i] = byte(i)
		}
		if err := os.WriteFile(p, b, 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	const limit int64 = 8
	under := write("under.bin", 7)
	exact := write("exact.bin", 8)
	over := write("over.bin", 9)
	missing := filepath.Join(dir, "missing.bin")

	tests := []struct {
		name    string
		path    string
		wantLen int
		wantErr error
	}{
		{name: "under", path: under, wantLen: 7},
		{name: "exact", path: exact, wantLen: 8},
		{name: "over", path: over, wantErr: safefs.ErrTooLarge},
		{name: "missing", path: missing, wantErr: fs.ErrNotExist},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := File(tt.path, limit)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			want := make([]byte, tt.wantLen)
			for i := range want {
				want[i] = byte(i)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("got %q, want %q", got, want)
			}
		})
	}
}
