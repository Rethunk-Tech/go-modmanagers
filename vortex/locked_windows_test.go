package vortex

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// Vortex opens its write-ahead log without read sharing; reading the database meanwhile must say so.
func TestReadReportsLocked(t *testing.T) {
	dir := newData(t, vortex28())
	wal := filepath.Join(dir, "state.v2", "000099.log")
	if err := os.WriteFile(wal, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	name, err := syscall.UTF16PtrFromString(wal)
	if err != nil {
		t.Fatal(err)
	}
	h, err := syscall.CreateFile(name, syscall.GENERIC_WRITE, 0, nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = syscall.CloseHandle(h) }()
	if _, err := Read(dir, Options{TempDir: t.TempDir()}); !errors.Is(err, ErrLocked) {
		t.Fatalf("error = %v", err)
	}
}
