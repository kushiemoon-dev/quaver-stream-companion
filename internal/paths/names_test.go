package paths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEqualASCIIFoldLetterBoundaries(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want bool
	}{
		{"A", "a", true},
		{"a", "A", true},
		{"Z", "z", true},
		{"z", "Z", true},
		{"@", "`", false},
		{"[", "{", false},
		{"A", "b", false},
		{"Quaver.DB", "quaver.db", true},
	} {
		if got := equalASCIIFold(tc.a, tc.b); got != tc.want {
			t.Errorf("equalASCIIFold(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestDirNamesOfAnEmptyFolderIsNotAnError(t *testing.T) {
	names, err := dirNames(t.TempDir(), 10)
	if err != nil || len(names) != 0 {
		t.Fatalf("names %v, err %v", names, err)
	}
}

func TestDirNamesOfAMissingFolderFails(t *testing.T) {
	if _, err := dirNames(filepath.Join(t.TempDir(), "absent"), 10); err == nil {
		t.Fatal("expected an error")
	}
}

func TestUniqueDirsMergesDifferentSpellingsOfOneFolder(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "lib")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	sep := string(filepath.Separator)
	spelled := root + sep + "lib" + sep + ".." + sep + "lib"
	got := uniqueDirs([]string{sub, spelled})
	if len(got) != 1 || got[0] != sub {
		t.Fatalf("got %q, want only %q", got, sub)
	}
}
