package managerbase

import (
	"os"
	"runtime"
	"slices"
	"testing"
)

func TestBasesIsEmptyWhenNoFolderIsKnown(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the Linux folder is the caller's")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	if got := Bases(func(string) string { return "" }); got != nil {
		t.Errorf("no folder known, got %v", got)
	}
	if got := Bases(func(h string) string { return h + "/cfg" }); !slices.Equal(got, []string{home + "/cfg"}) {
		t.Errorf("got %v (HOME %s)", got, os.Getenv("HOME"))
	}
}
