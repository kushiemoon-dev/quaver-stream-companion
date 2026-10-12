package overlays

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestDirectoriesCountTowardTheEntryLimit(t *testing.T) {
	dir := t.TempDir()
	put(t, dir, "ok/index.html", "ok")
	put(t, dir, "dirs/index.html", "ok")
	for i := 0; i < MaxFiles; i++ {
		if err := os.MkdirAll(filepath.Join(dir, "dirs", fmt.Sprintf("d%d", i)), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Read(dir, "ok", ""); err != nil {
		t.Fatalf("small overlay refused: %v", err)
	}
	if _, err := Read(dir, "dirs", ""); err == nil {
		t.Error("an overlay full of empty folders must be refused")
	}
	if got := List(dir, none); len(got) != 1 || got[0].ID != "ok" {
		t.Errorf("listing = %+v", got)
	}
}

func TestManyNestedEmptyDirectoriesAreBounded(t *testing.T) {
	dir := t.TempDir()
	put(t, dir, "deep/index.html", "ok")
	for i := 0; i < 60; i++ {
		for j := 0; j < 60; j++ {
			if err := os.MkdirAll(filepath.Join(dir, "deep", fmt.Sprintf("a%d", i), fmt.Sprintf("b%d", j)), 0o700); err != nil {
				t.Fatal(err)
			}
		}
	}
	start := time.Now()
	if _, err := Read(dir, "deep", ""); err == nil {
		t.Error("overlay with thousands of folders served")
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Errorf("lookup took %v", d)
	}
}

func junction(t *testing.T, link, target string) {
	t.Helper()
	if runtime.GOOS != "windows" {
		t.Skip("junctions are a Windows feature")
	}
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput(); err != nil {
		t.Skipf("cannot create a junction: %v: %s", err, out)
	}
}

func TestJunctionsAreRefused(t *testing.T) {
	dir, outside := t.TempDir(), t.TempDir()
	put(t, outside, "x.js", "bad")
	put(t, outside, "index.html", "bad")
	put(t, dir, "mine/index.html", "ok")
	junction(t, filepath.Join(dir, "mine", "jump"), outside)
	junction(t, filepath.Join(dir, "alias"), filepath.Join(dir, "mine"))
	junction(t, filepath.Join(dir, "away"), outside)
	for _, c := range [][2]string{{"mine", "jump/x.js"}, {"alias", "index.html"}, {"away", "index.html"}} {
		if f, err := Read(dir, c[0], c[1]); err == nil {
			t.Errorf("%v served %q through a junction", c, f.Data)
		}
	}
	if got := List(dir, none); len(got) != 1 || got[0].ID != "mine" {
		t.Errorf("listing = %+v", got)
	}
}
