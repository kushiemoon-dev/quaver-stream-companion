package themes

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/kushiemoon-dev/quaver-stream-companion/web"
)

func write(t *testing.T, dir, name, css string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(css), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestUserDir(t *testing.T) {
	if got := UserDir(filepath.Join("a", "b", "config.json")); got != filepath.Join("a", "b", "themes") {
		t.Errorf("got %q", got)
	}
	if got := UserDir(""); got != "" {
		t.Errorf("empty config path gave %q", got)
	}
}

func TestMergeOrderSourcesAndOverride(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "zeta.css", ":root{--bg:#111;--fg:#eee;--accent:#f00}")
	write(t, dir, "alpha.css", ":root{--bg:#222}")
	write(t, dir, "nord.css", ":root{--bg:#010203;--fg:#fff;--accent:#0f0}")
	build := Builtin(web.FS)
	got := Merge(build, dir)
	want := append(slices.Clone(wantOrder), "alpha", "zeta")
	if ids := IDs(got); !slices.Equal(ids, want) {
		t.Fatalf("ids = %v", ids)
	}
	for _, th := range got {
		custom := th.ID == "nord" || th.ID == "alpha" || th.ID == "zeta"
		if (th.Source == SourceCustom) != custom {
			t.Errorf("%s source = %q", th.ID, th.Source)
		}
	}
	if got[6].ID != "nord" || got[6].Colors.BG != "#010203" {
		t.Errorf("nord not replaced in place: %+v", got[6])
	}
	if build[6].Source != SourceBuiltin {
		t.Error("Merge mutated the built-in list")
	}
}

func TestMergeIgnoresUnusableEntries(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "ok.css", ":root{--bg:#111}")
	write(t, dir, "big.css", strings.Repeat("a", 65<<10))
	write(t, dir, "Upper.css", ":root{}")
	write(t, dir, "a.b.css", ":root{}")
	write(t, dir, "notes.txt", "x")
	write(t, dir, "target.css", ":root{}")
	if err := os.Mkdir(filepath.Join(dir, "evil.css"), 0o700); err != nil {
		t.Fatal(err)
	}
	got := IDs(Merge(nil, dir))
	if !slices.Equal(got, []string{"ok", "target"}) {
		t.Fatalf("ids = %v", got)
	}
}

func TestMergeMissingFolderAndOversizedOverrideKeepsBuiltin(t *testing.T) {
	if got := IDs(Merge(Builtin(web.FS), filepath.Join(t.TempDir(), "none"))); !slices.Equal(got, wantOrder) {
		t.Fatalf("missing folder: %v", got)
	}
	dir := t.TempDir()
	write(t, dir, "nord.css", strings.Repeat("a", 65<<10))
	for _, th := range Merge(Builtin(web.FS), dir) {
		if th.Source != SourceBuiltin {
			t.Errorf("%s should stay built-in", th.ID)
		}
	}
}

func TestReadUserBounds(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "edge.css", strings.Repeat("a", MaxUserFile))
	if data, _, err := ReadUser(dir, "edge"); err != nil || len(data) != MaxUserFile {
		t.Fatalf("64 KiB file: %d %v", len(data), err)
	}
	write(t, dir, "over.css", strings.Repeat("a", MaxUserFile+1))
	for _, id := range []string{"over", "missing", "../x", "UP", "a.b", "x\x00", "a/b", `a\b`, ""} {
		if _, _, err := ReadUser(dir, id); err == nil {
			t.Errorf("%q should be refused", id)
		}
	}
	if _, _, err := ReadUser("", "edge"); err == nil {
		t.Error("empty folder should be refused")
	}
}

func TestMergeKeepsAtMostMaxUserThemes(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < MaxUserThemes+20; i++ {
		write(t, dir, fmt.Sprintf("t%03d.css", i), ":root{--bg:#111}")
	}
	got := IDs(Merge(nil, dir))
	if len(got) != MaxUserThemes || got[0] != "t000" || got[len(got)-1] != fmt.Sprintf("t%03d", MaxUserThemes-1) {
		t.Fatalf("got %d ids, first %q, last %q", len(got), got[0], got[len(got)-1])
	}
}

func TestReadUserRefusesDifferentCaseOnDisk(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "UP.css", ":root{}")
	if _, _, err := ReadUser(dir, "up"); err == nil {
		t.Error("up must not read UP.css")
	}
}

func TestMergeSkipsSymlinksButListsHardLinks(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "target.css", ":root{}")
	t.Run("symlink", func(t *testing.T) {
		if err := os.Symlink(filepath.Join(dir, "target.css"), filepath.Join(dir, "link.css")); err != nil {
			t.Skipf("symlinks are not available here: %v", err)
		}
		if got := IDs(Merge(nil, dir)); !slices.Equal(got, []string{"target"}) {
			t.Fatalf("ids = %v", got)
		}
	})
	t.Run("hardlink", func(t *testing.T) {
		if err := os.Link(filepath.Join(dir, "target.css"), filepath.Join(dir, "hard.css")); err != nil {
			t.Skipf("hard links are not available here: %v", err)
		}
		if got := IDs(Merge(nil, dir)); !slices.Equal(got, []string{"hard", "target"}) {
			t.Fatalf("ids = %v", got)
		}
	})
}
