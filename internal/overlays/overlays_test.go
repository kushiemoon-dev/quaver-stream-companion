package overlays

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func put(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func none(string) bool { return false }

func TestDir(t *testing.T) {
	if got := Dir(filepath.Join("a", "b", "config.json")); got != filepath.Join("a", "b", "overlays") {
		t.Errorf("got %q", got)
	}
	for _, p := range []string{"", `\\host\share\config.json`, "//host/share/config.json"} {
		if got := Dir(p); got != "" {
			t.Errorf("Dir(%q) = %q", p, got)
		}
	}
}

func TestIsBuiltin(t *testing.T) {
	assets := fstest.MapFS{"overlays/now-playing/index.html": {Data: []byte("x")}, "overlays/common.js": {Data: []byte("x")}}
	if !IsBuiltin(assets, "now-playing") || IsBuiltin(assets, "mine") || IsBuiltin(assets, "common.js") {
		t.Error("built-in detection is wrong")
	}
}

func TestReadServesIndexAndNestedFilesWithFixedTypes(t *testing.T) {
	dir := t.TempDir()
	put(t, dir, "mine/index.html", "<p>hi</p>")
	put(t, dir, "mine/css/a.css", "a{}")
	put(t, dir, "mine/a/b/c.js", "1")
	for rel, want := range map[string]string{"": "text/html; charset=utf-8", "index.html": "text/html; charset=utf-8", "css/a.css": "text/css; charset=utf-8", "a/b/c.js": "text/javascript; charset=utf-8"} {
		f, err := Read(dir, "mine", rel)
		if err != nil || f.Type != want || len(f.Data) == 0 {
			t.Errorf("%q: %v type=%q", rel, err, f.Type)
		}
	}
}

func TestReadRefusesUnsafeRequests(t *testing.T) {
	dir := t.TempDir()
	put(t, dir, "mine/index.html", "ok")
	put(t, dir, "secret.html", "bad")
	put(t, dir, "mine/.hidden.html", "bad")
	put(t, dir, "mine/.git/x.html", "bad")
	put(t, dir, "mine/Up.html", "bad")
	put(t, dir, "mine/a/b/c/d.js", "too deep")
	put(t, dir, "mine/tool.exe", "bad")
	put(t, dir, "mine/noext", "bad")
	put(t, dir, "mine/s p.html", "bad")
	put(t, dir, "mine/sub/x.js", "ok")
	bad := []string{
		"../secret.html", "..", "a/../../secret.html", "%2e%2e/secret.html", `..\secret.html`, `sub\x.js`, "x.html\x00", "index.html\x00.css",
		"INDEX.HTML", "index.HTML", "up.html", ".hidden.html", ".git/x.html", "a/b/c/d.js", "tool.exe", "noext", "s p.html",
		"index.html:stream", "index.html::$DATA", "index.html.", "index.html ", "sub", "sub/", "/index.html", "//index.html", "sub//x.js", "./index.html", "~1.html",
	}
	for _, rel := range bad {
		if f, err := Read(dir, "mine", rel); err == nil {
			t.Errorf("%q served %q", rel, f.Data)
		}
	}
	for _, id := range []string{"", "..", "Mine", "mine/..", "mi.ne", "mine\x00", "secret.html", `mine\`} {
		if _, err := Read(dir, id, "index.html"); err == nil {
			t.Errorf("id %q served", id)
		}
	}
	if _, err := Read("", "mine", ""); err == nil {
		t.Error("empty dir served")
	}
}

func TestReadSizeLimit(t *testing.T) {
	dir := t.TempDir()
	put(t, dir, "mine/index.html", "ok")
	put(t, dir, "mine/edge.js", strings.Repeat("a", MaxFile))
	put(t, dir, "mine/big.js", strings.Repeat("a", MaxFile+1))
	if f, err := Read(dir, "mine", "edge.js"); err != nil || len(f.Data) != MaxFile {
		t.Errorf("edge file: %v", err)
	}
	if _, err := Read(dir, "mine", "big.js"); err == nil {
		t.Error("oversized file served")
	}
}

func TestReadFileCountLimit(t *testing.T) {
	dir := t.TempDir()
	put(t, dir, "ok/index.html", "ok")
	for i := 0; i < MaxFiles-2; i++ {
		put(t, dir, fmt.Sprintf("ok/sub/f%d.txt", i), "x")
	}
	put(t, dir, "many/index.html", "ok")
	for i := 0; i < MaxFiles; i++ {
		put(t, dir, fmt.Sprintf("many/sub/f%d.txt", i), "x")
	}
	if _, err := Read(dir, "ok", ""); err != nil {
		t.Errorf("overlay with %d files refused: %v", MaxFiles, err)
	}
	if _, err := Read(dir, "many", ""); err == nil {
		t.Errorf("overlay with %d files served", MaxFiles+1)
	}
	if got := List(dir, none); len(got) != 1 || got[0].ID != "ok" {
		t.Errorf("listing = %+v", got)
	}
}

func TestReadIndexIsNotADirectoryListing(t *testing.T) {
	dir := t.TempDir()
	put(t, dir, "mine/sub/x.js", "x")
	if _, err := Read(dir, "mine", ""); err == nil {
		t.Error("missing index.html must not list the folder")
	}
	if _, err := Read(dir, "mine", "sub"); err == nil {
		t.Error("directory served")
	}
}

func TestReadRefusesSymlinks(t *testing.T) {
	dir, outside := t.TempDir(), t.TempDir()
	put(t, outside, "x.js", "bad")
	put(t, dir, "mine/index.html", "ok")
	if err := os.Symlink(filepath.Join(outside, "x.js"), filepath.Join(dir, "mine", "link.js")); err != nil {
		t.Skipf("symlinks are not available here: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "mine", "dirlink")); err != nil {
		t.Skipf("symlinks are not available here: %v", err)
	}
	if err := os.Symlink(filepath.Join(dir, "mine"), filepath.Join(dir, "alias")); err != nil {
		t.Skipf("symlinks are not available here: %v", err)
	}
	for _, c := range [][2]string{{"mine", "link.js"}, {"mine", "dirlink/x.js"}, {"alias", "index.html"}} {
		if f, err := Read(dir, c[0], c[1]); err == nil {
			t.Errorf("%v served %q", c, f.Data)
		}
	}
	if got := List(dir, none); len(got) != 1 || got[0].ID != "mine" {
		t.Errorf("listing = %+v", got)
	}
}

func TestReadServesHardLinks(t *testing.T) {
	dir := t.TempDir()
	put(t, dir, "src.html", "linked")
	if err := os.MkdirAll(filepath.Join(dir, "mine"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(filepath.Join(dir, "src.html"), filepath.Join(dir, "mine", "index.html")); err != nil {
		t.Skipf("hard links are not available here: %v", err)
	}
	if f, err := Read(dir, "mine", ""); err != nil || string(f.Data) != "linked" {
		t.Errorf("hard link: %v", err)
	}
}

func TestReadExactNameOnly(t *testing.T) {
	dir := t.TempDir()
	put(t, dir, "mine/index.html", "ok")
	put(t, dir, "mine/Sub/X.JS", "ok")
	if _, err := Read(dir, "mine", "sub/X.JS"); err == nil {
		t.Error("directory served under another case")
	}
	if _, err := Read(dir, "mine", "Sub/x.js"); err == nil {
		t.Error("file served under another case")
	}
	if _, err := Read(dir, "mine", "Sub/X.JS"); err != nil {
		t.Errorf("exact name refused: %v", err)
	}
	put(t, dir, "Other/index.html", "bad")
	if _, err := Read(dir, "other", ""); err == nil {
		t.Error("folder Other served as other")
	}
}

func TestReadSeesNewFoldersWithoutRestart(t *testing.T) {
	dir := t.TempDir()
	if _, err := Read(dir, "late", ""); err == nil {
		t.Fatal("served before it exists")
	}
	put(t, dir, "late/index.html", "now")
	if f, err := Read(dir, "late", ""); err != nil || string(f.Data) != "now" {
		t.Errorf("new folder: %v", err)
	}
}
