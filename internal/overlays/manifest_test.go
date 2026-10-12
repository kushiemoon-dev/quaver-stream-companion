package overlays

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func one(t *testing.T, manifestBody string) Info {
	t.Helper()
	dir := t.TempDir()
	put(t, dir, "mine/index.html", "ok")
	if manifestBody != "-" {
		put(t, dir, "mine/overlay.json", manifestBody)
	}
	got := List(dir, none)
	if len(got) != 1 {
		t.Fatalf("listing = %+v", got)
	}
	return got[0]
}

func TestManifestDefaults(t *testing.T) {
	want := Info{ID: "mine", Name: "mine", Width: 500, Height: 200}
	for name, body := range map[string]string{
		"absent": "-", "empty": "", "not json": "{nope", "array": "[]", "null": "null", "trailing data": `{"name":"x"} {}`,
		"trailing garbage": `{"name":"x"} x`, "wrong types": `{"name":5,"width":"wide"}`, "huge number": `{"width":1e999}`,
		"oversize": `{"name":"x","pad":"` + strings.Repeat("a", 8<<10) + `"}`,
	} {
		if got := one(t, body); got != want {
			t.Errorf("%s: %+v", name, got)
		}
	}
}

func TestManifestValuesAreCleanedAndClamped(t *testing.T) {
	got := one(t, `{"name":"  My overlay  ","description":"Shows things","width":640,"height":120.4}`)
	if got != (Info{ID: "mine", Name: "My overlay", Description: "Shows things", Width: 640, Height: 120}) {
		t.Errorf("valid: %+v", got)
	}
	for body, want := range map[string][2]int{
		`{"width":-5,"height":0}`:         {100, 40},
		`{"width":99999,"height":99999}`:  {3840, 2160},
		`{"width":1e300,"height":-1e300}`: {3840, 40},
		`{"width":null}`:                  {500, 200},
		`{"height":90}`:                   {500, 90},
	} {
		if g := one(t, body); g.Width != want[0] || g.Height != want[1] {
			t.Errorf("%s: %dx%d", body, g.Width, g.Height)
		}
	}
	long := one(t, `{"name":"`+strings.Repeat("n", 100)+`","description":"`+strings.Repeat("d", 500)+`"}`)
	if len(long.Name) != 40 || len(long.Description) != 160 {
		t.Errorf("caps: %d %d", len(long.Name), len(long.Description))
	}
	wide := one(t, `{"name":"`+strings.Repeat(string(rune(0xe9)), 100)+`"}`)
	if len([]rune(wide.Name)) != 40 {
		t.Errorf("rune cap: %d", len([]rune(wide.Name)))
	}
	bidi := string(rune(0x202e))
	hostile := one(t, `{"name":"a\u0000b\nc`+bidi+`<script>","description":"\u0007\r\n  "}`)
	if strings.ContainsAny(hostile.Name, "\x00\n"+bidi) || hostile.Description != "" {
		t.Errorf("hostile: %q %q", hostile.Name, hostile.Description)
	}
	if one(t, `{"name":"\u0000\n"}`).Name != "mine" {
		t.Error("blank name must fall back to the id")
	}
}

func TestManifestSymlinkIsIgnored(t *testing.T) {
	dir := t.TempDir()
	put(t, dir, "mine/index.html", "ok")
	put(t, dir, "real.json", `{"name":"Linked"}`)
	if err := os.Symlink(filepath.Join(dir, "real.json"), filepath.Join(dir, "mine", "overlay.json")); err != nil {
		t.Skipf("symlinks are not available here: %v", err)
	}
	if got := List(dir, none); len(got) != 1 || got[0].Name != "mine" {
		t.Errorf("listing = %+v", got)
	}
}

func TestListSortsSkipsBuiltinsAndMissingIndex(t *testing.T) {
	dir := t.TempDir()
	for _, id := range []string{"zeta", "alpha", "now-playing", "mid"} {
		put(t, dir, id+"/index.html", "ok")
	}
	put(t, dir, "noindex/overlay.json", `{"name":"x"}`)
	put(t, dir, "Upper/index.html", "ok")
	put(t, dir, "file.html", "ok")
	got := List(dir, func(id string) bool { return id == "now-playing" })
	var ids []string
	for _, i := range got {
		ids = append(ids, i.ID)
	}
	if strings.Join(ids, ",") != "alpha,mid,zeta" {
		t.Errorf("ids = %v", ids)
	}
}

func TestListMissingFolderAndBound(t *testing.T) {
	if got := List(filepath.Join(t.TempDir(), "absent"), none); got == nil || len(got) != 0 {
		t.Errorf("missing folder = %#v", got)
	}
	if got := List("", none); got == nil || len(got) != 0 {
		t.Errorf("empty dir = %#v", got)
	}
	dir := t.TempDir()
	for i := 0; i < MaxListed+20; i++ {
		put(t, dir, fmt.Sprintf("o%03d/index.html", i), "ok")
	}
	got := List(dir, none)
	if len(got) != MaxListed || got[0].ID != "o000" || got[MaxListed-1].ID != fmt.Sprintf("o%03d", MaxListed-1) {
		t.Errorf("bound: %d entries", len(got))
	}
}
