package paths

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// bs is built from its code point so no editor can collapse doubled backslashes in the cases below.
var bs = string(rune(92))

// uncPath joins parts with backslashes after a leading double backslash.
func uncPath(parts ...string) string { return bs + bs + strings.Join(parts, bs) }

func TestIsUNCSpellings(t *testing.T) {
	win := runtime.GOOS == "windows"
	cases := []struct {
		in   string
		want bool
	}{
		{uncPath("host", "share"), true},
		{`//host/share`, true},
		{`//`, true},
		{bs + bs, true},
		{bs + `/`, true},
		{bs + `/host` + bs + `share`, true},
		{`/` + bs + `host/share`, true},
		{uncPath("?", "UNC", "h", "s"), true},
		{uncPath(".", "pipe", "x"), true},
		{bs + `??` + bs + `UNC` + bs + `h` + bs + `s`, win},
		{bs + `host` + bs + `share`, win},
		{`/host/share`, win},
		{`/home/user/Quaver`, win},
		{`C:` + bs + `Games` + bs + `Quaver`, false},
		{`D:/SteamLibrary/steamapps/common/Quaver`, false},
		{`Quaver`, false},
		{`..` + bs + `Quaver`, false},
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
	for _, p := range []string{
		uncPath("invalid-host-xyz", "share"),
		`//invalid-host-xyz/share`,
		bs + `/invalid-host-xyz` + bs + `share`,
		`/` + bs + `invalid-host-xyz/share`,
		uncPath("?", "UNC", "invalid-host-xyz", "share"),
	} {
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
	writeVDF(t, root, uncPath("invalid-host-xyz", "share"), `//invalid-host-xyz/share`)
	if err := os.MkdirAll(filepath.Join(root, "steamapps"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, l := range libraries(root) {
		if IsUNC(l) {
			t.Errorf("libraries returned %q", l)
		}
	}
	start := time.Now()
	_, err := Find("", []string{root, uncPath("invalid-host-xyz", "steam")})
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
	for _, p := range []string{
		bs + `/host` + bs + `share`,
		`/` + bs + `host/share`,
		uncPath("host", "share"),
	} {
		if _, err := Find(p, nil); err == nil || !strings.Contains(err.Error(), "network path") {
			t.Errorf("Find(%q) err = %v", p, err)
		}
	}
}
