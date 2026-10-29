// Package managerbase names the per-OS folder another mod manager keeps its data under.
package managerbase

import (
	"os"
	"runtime"

	"github.com/Rethunk-Tech/go-desktopenv/userdirs"
)

// Bases is the folder holding another program's per-user data: on Linux linuxHome, which must be one of
// userdirs' Host functions so that inside a Flatpak it is the host's folder and never the sandbox's own; on
// Windows %AppData%; elsewhere os.UserConfigDir. It is empty when no home is known.
func Bases(linuxHome func(home string) string) []string {
	home, _ := os.UserHomeDir()
	var base string
	switch runtime.GOOS {
	case "linux":
		base = linuxHome(home)
	case "windows":
		base = userdirs.AppData(home)
	default:
		base, _ = os.UserConfigDir()
	}
	if base == "" {
		return nil
	}
	return []string{base}
}
