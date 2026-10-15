package diagnose

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
)

func hasShortNames(p string) bool {
	in, err := syscall.UTF16PtrFromString(p)
	if err != nil {
		return false
	}
	buf := make([]uint16, 512)
	n, err := syscall.GetShortPathName(in, &buf[0], uint32(len(buf)))
	if err != nil || n == 0 || int(n) > len(buf) {
		return false
	}
	leaf := filepath.Base(syscall.UTF16ToString(buf[:n]))
	return strings.Contains(leaf, "~") && leaf != filepath.Base(p)
}

// longNamedDir creates a folder whose name needs an 8.3 alias and returns it with the alias,
// or skips when the volume has no short names.
func longNamedDir(t *testing.T) (dir, short string) {
	t.Helper()
	dir = filepath.Join(t.TempDir(), "a long folder name")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if !hasShortNames(dir) {
		t.Skip("this volume has no 8.3 short names")
	}
	return dir, shortPath(dir)
}

func TestShortPathGivesTheAliasOfALongName(t *testing.T) {
	dir, short := longNamedDir(t)
	if !strings.Contains(filepath.Base(short), "~") || strings.EqualFold(short, dir) {
		t.Fatalf("shortPath(%q) = %q", dir, short)
	}
}

func TestShortPathIsEmptyWithoutAFileOrWithNUL(t *testing.T) {
	if got := shortPath(filepath.Join(t.TempDir(), "absent")); got != "" {
		t.Errorf("absent path gave %q", got)
	}
	if got := shortPath("a\x00b"); got != "" {
		t.Errorf("path with NUL gave %q", got)
	}
}

func TestUserNamesIncludeTheShortAliasOfTheHome(t *testing.T) {
	dir, short := longNamedDir(t)
	names := userNames(dir)
	if want := lastSegment(short); !slices.Contains(names, want) {
		t.Fatalf("userNames(%q) = %q, missing %q", dir, names, want)
	}
}
