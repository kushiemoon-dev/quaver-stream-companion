package themes

import (
	"regexp"
	"strings"
	"testing"
)

// Minified CSS has no space after the semicolon: every declaration must still be read.
func TestParseColorsMinifiedCSS(t *testing.T) {
	got := ParseColors(":root{--bg:#fff;--fg:#000;--accent:#f00}")
	want := Colors{BG: "#fff", FG: "#000", Accent: "#f00"}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

// refColors is a reference reader: split the first :root block on ";" and read each declaration.
func refColors(css string) Colors {
	var c Colors
	block := rootBlock.FindStringSubmatch(comment.ReplaceAllString(css, " "))
	if block == nil {
		return c
	}
	decl := regexp.MustCompile(`^--(bg|fg|accent)\s*:\s*(.*)$`)
	for _, d := range strings.Split(block[1], ";") {
		s := strings.TrimLeft(d, " \t\n\r\f")
		m := decl.FindStringSubmatch(s)
		if m == nil {
			continue
		}
		v := strings.TrimRight(m[2], " \t\n\r\f")
		if !validHex(v) {
			v = ""
		}
		switch m[1] {
		case "bg":
			c.BG = v
		case "fg":
			c.FG = v
		case "accent":
			c.Accent = v
		}
	}
	return c
}

// FuzzParseColorsMatchesReference checks ParseColors against a reference reader on declaration lists.
func FuzzParseColorsMatchesReference(f *testing.F) {
	f.Add("#fff", "#000", "#f00", " ", "\n")
	f.Fuzz(func(t *testing.T, bg, fg, accent, s1, s2 string) {
		for _, v := range []string{bg, fg, accent} {
			if !validHex(v) {
				return
			}
		}
		for _, s := range []string{s1, s2} {
			if strings.Trim(s, " \t\n") != "" {
				return
			}
		}
		css := ":root{--bg:" + bg + ";" + s1 + "--fg:" + fg + ";" + s2 + "--accent:" + accent + "}"
		if got, want := ParseColors(css), refColors(css); got != want {
			t.Fatalf("ParseColors(%q) = %+v, want %+v", css, got, want)
		}
	})
}
