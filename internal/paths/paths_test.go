package paths

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFindOverride(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "Quaver.exe"))
	got, err := Find(dir, nil)
	if err != nil || got != dir {
		t.Fatalf("Find = %q, %v", got, err)
	}
}

func TestFindOverrideInvalid(t *testing.T) {
	_, err := Find(t.TempDir(), nil)
	if err == nil || !strings.Contains(err.Error(), "not a Quaver folder") {
		t.Fatalf("err = %v", err)
	}
}

func TestFindInSteamRoot(t *testing.T) {
	root := t.TempDir()
	want := filepath.Join(root, "steamapps", "common", "Quaver")
	touch(t, filepath.Join(want, "Quaver.exe"))
	got, err := Find("", []string{root})
	if err != nil || got != want {
		t.Fatalf("Find = %q, %v", got, err)
	}
}

func TestFindInSecondLibrary(t *testing.T) {
	root := t.TempDir()
	lib := t.TempDir()
	want := filepath.Join(lib, "steamapps", "common", "Quaver")
	touch(t, filepath.Join(want, "Quaver.exe"))

	esc := func(p string) string { return strings.ReplaceAll(p, `\`, `\\`) }
	vdf := fmt.Sprintf("\"libraryfolders\"\n{\n\t\"0\"\n\t{\n\t\t\"path\"\t\t\"%s\"\n\t}\n\t\"1\"\n\t{\n\t\t\"path\"\t\t\"%s\"\n\t}\n}\n", esc(root), esc(lib))
	touch(t, filepath.Join(root, "steamapps", "libraryfolders.vdf"))
	if err := os.WriteFile(filepath.Join(root, "steamapps", "libraryfolders.vdf"), []byte(vdf), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Find("", []string{root})
	if err != nil || got != want {
		t.Fatalf("Find = %q, %v", got, err)
	}
}

func TestFindNotFoundMentionsFlag(t *testing.T) {
	_, err := Find("", []string{t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "--quaver-dir") {
		t.Fatalf("err = %v", err)
	}
}

func TestFindRejectsUNCOverride(t *testing.T) {
	for _, p := range []string{"\x5c\x5cserver\x5cshare", `//server/share`} {
		_, err := Find(p, nil)
		if err == nil || !strings.Contains(err.Error(), "network") {
			t.Fatalf("Find(%q) err = %v", p, err)
		}
	}
}
