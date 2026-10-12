package paths

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestIsUNCSpellings(t *testing.T) {
	win := runtime.GOOS == "windows"
	cases := []struct {
		in   string
		want bool
	}{
		{`\host\share`, true},
		{`//host/share`, true},
		{`\/host\share`, true},
		{`/\host/share`, true},
		{`\?\UNC\h\s`, true},
		{`\.\pipe\x`, true},
		{`\??\UNC\h\s`, win},
		{`\host\share`, win},
		{`/host/share`, win},
		{`/home/user/Quaver`, win},
		{`C:\Games\Quaver`, false},
		{`G:/SteamLibrary/steamapps/common/Quaver`, false},
		{`Quaver`, false},
		{`..\Quaver`, false},
		{``, false},
	}
	for _, c := range cases {
		if got := IsUNC(c.in); got != c.want {
			t.Errorf("IsUNC(%q) = %v, want %v on %s", c.in, got, c.want, runtime.GOOS)
		}
	}
}

func TestIsQuaverDirRejectsUNCWithoutStat(t *testing.T) {
	start := time.Now()
	for _, p := range []string{`\invalid-host-xyz\share`, `//invalid-host-xyz/share`, `\/invalid-host-xyz\share`, `/\invalid-host-xyz/share`, `\?\UNC\invalid-host-xyz\share`} {
		if IsQuaverDir(p) {
			t.Errorf("IsQuaverDir(%q) = true", p)
		}
	}
	if d := time.Since(start); d > time.Second {
		t.Fatalf("took %v, a stat was probably attempted", d)
	}
}

func TestFindSkipsUNCLibraries(t *testing.T) {
	root := t.TempDir()
	vdf := "\"libraryfolders\"\n{\n\t\"1\"\n\t{\n\t\t\"path\"\t\t\"\\\\invalid-host-xyz\\share\"\n\t}\n\t\"2\"\n\t{\n\t\t\"path\"\t\t\"//invalid-host-xyz/share\"\n\t}\n}\n"
	if err := os.MkdirAll(filepath.Join(root, "steamapps"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "steamapps", "libraryfolders.vdf"), []byte(vdf), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, l := range libraries(root) {
		if IsUNC(l) {
			t.Errorf("libraries returned %q", l)
		}
	}
	start := time.Now()
	_, err := Find("", []string{root, `\invalid-host-xyz\steam`})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("err = %v", err)
	}
	if strings.Contains(err.Error(), "invalid-host-xyz") {
		t.Fatalf("UNC path was tried: %v", err)
	}
	if time.Since(start) > time.Second {
		t.Fatal("Find was slow")
	}
}

func TestFindOverrideRejectsUNCSpellings(t *testing.T) {
	for _, p := range []string{`\/host\share`, `/\host/share`} {
		if _, err := Find(p, nil); err == nil || !strings.Contains(err.Error(), "network path") {
			t.Errorf("Find(%q) err = %v", p, err)
		}
	}
}
