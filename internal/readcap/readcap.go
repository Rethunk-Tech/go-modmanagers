// Package readcap reads files another program wrote under a size cap.
package readcap

import (
	"fmt"
	"path/filepath"

	"github.com/Rethunk-Tech/go-safefs"
)

// MaxFile is the most any single file read here may hold.
const MaxFile = 16 << 20

// File reads the file at path, refusing one larger than limit (an error wrapping safefs.ErrTooLarge) instead of
// holding it whole.
func File(path string, limit int64) ([]byte, error) {
	b, err := safefs.ReadFileCapped(path, limit)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	return b, nil
}

// PlainName reports whether name can be joined below a folder without leaving it: one path element, not "." or "..",
// with no separator, drive or stream colon, or NUL.
func PlainName(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	for _, r := range name {
		if r == '/' || r == '\\' || r == ':' || r == 0 {
			return false
		}
	}
	return true
}
