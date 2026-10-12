package themes

import (
	"errors"
	"io/fs"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/kushiemoon-dev/quaver-stream-companion/web"
)

var wantOrder = []string{"neutral-light", "neutral-dark", "catppuccin-mocha", "catppuccin-latte", "tokyo-night", "dracula", "nord", "gruvbox", "rose-pine", "pixel"}

func TestBuiltinOrderAndColors(t *testing.T) {
	got := Builtin(web.FS)
	if ids := IDs(got); !slices.Equal(ids, wantOrder) {
		t.Fatalf("ids = %v", ids)
	}
	for _, th := range got {
		if th.Source != SourceBuiltin {
			t.Errorf("%s source = %q", th.ID, th.Source)
		}
		for name, c := range map[string]string{"bg": th.Colors.BG, "fg": th.Colors.FG, "accent": th.Colors.Accent} {
			if !validHex(c) {
				t.Errorf("%s %s = %q", th.ID, name, c)
			}
		}
	}
	want := map[string]Colors{
		"nord":          {BG: "#2e3440", FG: "#eceff4", Accent: "#88c0d0"},
		"neutral-light": {BG: "#ffffff", FG: "#1c1c1e", Accent: "#3b6fe0"},
	}
	for _, th := range got {
		if w, ok := want[th.ID]; ok && th.Colors != w {
			t.Errorf("%s colors = %+v", th.ID, th.Colors)
		}
	}
	if got[0].Colors.BG == got[1].Colors.BG {
		t.Errorf("light and dark share a background: %+v %+v", got[0].Colors, got[1].Colors)
	}
}

func TestParseColorsIsStrict(t *testing.T) {
	cases := []struct {
		name, css string
		want      Colors
	}{
		{"short and long", ":root { --bg: #FFF; --fg:#112233 ; --accent: #abcdef; }", Colors{"#FFF", "#112233", "#abcdef"}},
		{"not hex", ":root { --bg: red; --fg: rgb(1 2 3); --accent: #12345; }", Colors{}},
		{"alpha rejected", ":root { --bg: #11223344; --fg: #1234; --accent: var(--bg); }", Colors{}},
		{"outside root ignored", "body { --bg: #111; } :root { --fg: #222; }", Colors{FG: "#222"}},
		{"no root", "body { color: #111; }", Colors{}},
		{"brace in comment", ":root { /* } */ --bg: #111; --fg: #222; }", Colors{BG: "#111", FG: "#222"}},
		{"commented property", ":root { /* --bg: #f00; */ --fg: #222; }", Colors{FG: "#222"}},
		{"commented root before", "/* :root { --bg: #f00; } */ :root { --fg: #222; }", Colors{FG: "#222"}},
		{"url declaration", ":root { --fg: url(x); --bg: #111; }", Colors{BG: "#111"}},
		{"injection", ":root { --bg: #111; background: url(x); --fg: #222\"><script>; --accent: #333; }", Colors{BG: "#111", Accent: "#333"}},
	}
	for _, c := range cases {
		if got := ParseColors(c.css); got != c.want {
			t.Errorf("%s: %+v, want %+v", c.name, got, c.want)
		}
	}
}

type failingFS struct {
	fs.FS
	bad string
}

func (f failingFS) Open(name string) (fs.File, error) {
	if name == f.bad {
		return nil, errors.New("denied")
	}
	return f.FS.Open(name)
}

func TestUnreadableOrColorlessThemeKeepsItsEntry(t *testing.T) {
	base := fstest.MapFS{
		"themes/zeta.css":  {Data: []byte("body{}")},
		"themes/nord.css":  {Data: []byte(":root{--bg:#000;--fg:#fff;--accent:#f00}")},
		"themes/alpha.css": {Data: []byte(":root{--bg:#111;}")},
		"themes/notes.md":  {},
	}
	got := Builtin(failingFS{FS: base, bad: "themes/alpha.css"})
	if ids := IDs(got); !slices.Equal(ids, []string{"nord", "alpha", "zeta"}) {
		t.Fatalf("ids = %v", ids)
	}
	if got[0].Colors.Accent != "#f00" || got[1].Colors != (Colors{}) || got[2].Colors != (Colors{}) {
		t.Fatalf("colors = %+v", got)
	}
}

func TestBuiltinIgnoresOddNamesAndKeepsDirectoriesColorless(t *testing.T) {
	fsys := fstest.MapFS{
		"themes/ok-1.css":       {Data: []byte(":root{--bg:#000;}")},
		"themes/Weird_Name.css": {Data: []byte(":root{--bg:#000;}")},
		"themes/a.b.css":        {Data: []byte(":root{--bg:#000;}")},
		"themes/UPPER.css":      {Data: []byte(":root{--bg:#000;}")},
		"themes/dir.css/x.txt":  {Data: []byte("x")},
	}
	got := Builtin(fsys)
	if ids := IDs(got); !slices.Equal(ids, []string{"dir", "ok-1"}) {
		t.Fatalf("ids = %v", ids)
	}
	if got[0].Colors != (Colors{}) {
		t.Fatalf("directory colors = %+v", got[0].Colors)
	}
}
