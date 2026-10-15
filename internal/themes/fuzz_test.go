package themes

import (
	"os"
	"path/filepath"
	"testing"
)

// FuzzParseColors: every color read is "" or a valid hex color.
func FuzzParseColors(f *testing.F) {
	for _, s := range []string{
		":root { --bg: #fff; --fg: #000000; --accent: #f0f; }",
		":root{--bg:#fff;--fg:#000;--accent:#f00}",
		"/* :root { --bg: #111 } */ :root { --bg: red }",
		":root { --bg: #fff", ":root{}", "",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, css string) {
		c := ParseColors(css)
		for _, v := range []string{c.BG, c.FG, c.Accent} {
			if v != "" && !validHex(v) {
				t.Fatalf("ParseColors(%q) gave %q", css, v)
			}
		}
	})
}

// FuzzReadUser: only <dir>/<id>.css with that exact name is ever returned.
func FuzzReadUser(f *testing.F) {
	dir := f.TempDir()
	for name, c := range map[string]string{"mine.css": "MINE", "Upper.css": "UPPER", "x.txt": "TXT"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(c), 0o644); err != nil {
			f.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(dir), "outside.css"), []byte("OUT"), 0o644); err != nil {
		f.Fatal(err)
	}
	for _, s := range []string{"mine", "MINE", "upper", "Upper", "x", "../outside", "mine.css", ""} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, id string) {
		data, _, err := ReadUser(dir, id)
		if err != nil {
			return
		}
		if id != "mine" || string(data) != "MINE" {
			t.Fatalf("ReadUser(%q) = %q", id, data)
		}
	})
}
