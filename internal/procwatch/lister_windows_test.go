package procwatch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWindowsListerSeesOwnProcessByName(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	l, ok := NewLister()
	if !ok {
		t.Fatal("the process check must be supported on Windows")
	}
	names, err := l.Names()
	if err != nil {
		t.Fatal(err)
	}
	own := filepath.Base(exe)
	for _, n := range names {
		if strings.EqualFold(n, own) {
			return
		}
	}
	t.Fatalf("own process %q not listed", own)
}
