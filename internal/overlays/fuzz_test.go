package overlays

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

// FuzzDescribe: any overlay.json gives an id, a clean bounded name and description and in-range size.
func FuzzDescribe(f *testing.F) {
	for _, s := range []string{
		`{"name":"My HUD","description":"d","width":800,"height":300}`,
		`{"width":1e308,"height":-1e308}`, `{"width":"x"}`, `{} {}`, `null`, `{"name":"\u0000​�  "}`,
		`{"width":1e400}`, `{"name":"` + strings.Repeat("é", 100) + `"}`,
	} {
		f.Add([]byte(s))
	}
	root := f.TempDir()
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > maxManifest {
			data = data[:maxManifest]
		}
		if err := os.WriteFile(filepath.Join(root, manifest), data, 0o644); err != nil {
			t.Fatal(err)
		}
		info := describe(root, "my-overlay")
		if info.ID != "my-overlay" || info.Name == "" {
			t.Fatalf("id/name %+v", info)
		}
		if info.Width < minWidth || info.Width > maxWidth || info.Height < minHeight || info.Height > maxHeight {
			t.Fatalf("size out of range %+v", info)
		}
		for _, s := range []struct {
			v   string
			max int
		}{{info.Name, maxName}, {info.Description, maxDesc}} {
			if utf8.RuneCountInString(s.v) > s.max || !utf8.ValidString(s.v) || s.v != strings.TrimSpace(s.v) {
				t.Fatalf("text %q not clean", s.v)
			}
			for _, r := range s.v {
				if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == utf8.RuneError {
					t.Fatalf("text %q keeps %U", s.v, r)
				}
			}
		}
	})
}

// FuzzClean is idempotent.
func FuzzClean(f *testing.F) {
	f.Add(" a​b ", 3)
	f.Add("\xff\xfe x", 1)
	f.Fuzz(func(t *testing.T, s string, max int) {
		max = max%300 + 1
		if max < 1 {
			max = -max + 1
		}
		once := clean(s, max)
		if twice := clean(once, max); twice != once {
			t.Fatalf("clean not idempotent: %q -> %q -> %q", s, once, twice)
		}
	})
}

// FuzzRead: a requested path only ever returns one of the overlay's own whitelisted, non-hidden files.
func FuzzRead(f *testing.F) {
	dir := f.TempDir()
	files := map[string]string{
		"ov/index.html":      "INDEX",
		"ov/sub/a.css":       "CSS",
		"ov/sub/deep/b.js":   "JS",
		"ov/.hidden.js":      "HIDDEN",
		"ov/notes.exe":       "EXE",
		"secret.txt":         "SECRET",
		"other/index.html":   "OTHER",
		"ov/sub/deep/x/y.js": "TOO-DEEP",
	}
	for p, c := range files {
		full := filepath.Join(dir, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			f.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(c), 0o644); err != nil {
			f.Fatal(err)
		}
	}
	allowed := map[string]string{"": "INDEX", "index.html": "INDEX", "sub/a.css": "CSS", "sub/deep/b.js": "JS"}
	for _, s := range []string{"", "index.html", "../secret.txt", "sub/../../secret.txt", "INDEX.HTML", "sub\\a.css", "sub/deep/x/y.js", ".hidden.js", "notes.exe", "sub//a.css", "index.html.", "index.html::$DATA"} {
		f.Add("ov", s)
	}
	f.Add("other", "")
	f.Add("..", "secret.txt")
	f.Fuzz(func(t *testing.T, id, rel string) {
		file, err := Read(dir, id, rel)
		if err != nil {
			return
		}
		got := string(file.Data)
		if id == "other" && got == "OTHER" && (rel == "" || rel == "index.html") {
			return
		}
		if want, ok := allowed[rel]; id != "ov" || !ok || got != want {
			t.Fatalf("Read(%q, %q) returned %q", id, rel, got)
		}
		if file.Type == "" {
			t.Fatalf("no content type for %q", rel)
		}
	})
}
