// Package testwrite writes test fixtures.
package testwrite

import (
	"os"
	"path/filepath"
	"testing"
)

// File writes body to path, creating its folders.
func File(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}
