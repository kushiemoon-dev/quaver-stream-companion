package paths

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsQuaverDirExecutables(t *testing.T) {
	cases := []struct {
		name  string
		files []string
		want  bool
	}{
		{"windows build", []string{"Quaver.exe"}, true},
		{"native linux build", []string{"Quaver"}, true},
		{"both", []string{"Quaver", "Quaver.exe"}, true},
		{"neither", []string{"quaver.db"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, f := range c.files {
				touch(t, filepath.Join(dir, f))
			}
			if got := IsQuaverDir(dir); got != c.want {
				t.Fatalf("IsQuaverDir = %v, want %v", got, c.want)
			}
		})
	}
}

func TestIsQuaverDirRefusesFolderNamedQuaver(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "Quaver"), 0o755); err != nil {
		t.Fatal(err)
	}
	if IsQuaverDir(dir) {
		t.Fatal("a sub folder named Quaver is not the game executable")
	}
}

func TestFindFindsNativeLinuxInstall(t *testing.T) {
	root := t.TempDir()
	want := filepath.Join(root, "steamapps", "common", "Quaver")
	touch(t, filepath.Join(want, "Quaver"))
	got, err := Find("", []string{root})
	if err != nil || got != want {
		t.Fatalf("Find = %q, %v", got, err)
	}
}

func TestFindErrorsNameBothExecutables(t *testing.T) {
	_, err := Find(t.TempDir(), nil)
	if err == nil || !strings.Contains(err.Error(), "Quaver or Quaver.exe") {
		t.Fatalf("err = %v", err)
	}
}
