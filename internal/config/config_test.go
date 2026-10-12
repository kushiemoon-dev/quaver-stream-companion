package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/testutil"
)

func write(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestDefaults(t *testing.T) {
	d := Defaults()
	if d.Port != DefaultPort || d.Overlay.Theme != "neutral-light" || d.QuaverDir != "" || d.Language != "" || d.Overlay.Accent != "" {
		t.Fatalf("defaults = %+v", d)
	}
}

func TestLoad(t *testing.T) {
	cases := []struct {
		name, content string
		want          Config
	}{
		{"empty file", "", Defaults()},
		{"empty object", "{}", Defaults()},
		{"partial", `{"port": 9000}`, Config{Port: 9000, Overlay: Overlay{Theme: "neutral-light"}}},
		{"zero port", `{"port": 0, "language": "fr"}`, Config{Port: DefaultPort, Language: "fr", Overlay: Overlay{Theme: "neutral-light"}}},
		{"empty theme", `{"overlay": {"theme": "", "accent": "ff00aa"}}`, Config{Port: DefaultPort, Overlay: Overlay{Theme: "neutral-light", Accent: "ff00aa"}}},
		{"unknown fields", `{"port": 9001, "extra": true, "overlay": {"x": 1}}`, Config{Port: 9001, Overlay: Overlay{Theme: "neutral-light"}}},
		{"full", `{"quaver_dir":"/q","port":8000,"language":"de","overlay":{"theme":"dark","accent":"fff"}}`, Config{QuaverDir: "/q", Port: 8000, Language: "de", Overlay: Overlay{Theme: "dark", Accent: "fff"}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Load(write(t, c.content))
			if err != nil {
				t.Fatal(err)
			}
			if got != c.want {
				t.Fatalf("got %+v, want %+v", got, c.want)
			}
		})
	}
}

func TestLoadMissingFileGivesDefaults(t *testing.T) {
	got, err := Load(filepath.Join(t.TempDir(), "nope", "config.json"))
	if err != nil || got != Defaults() {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestLoadInvalidJSONMentionsPath(t *testing.T) {
	p := write(t, `{"port": `)
	_, err := Load(p)
	if err == nil || !strings.Contains(err.Error(), p) {
		t.Fatalf("err = %v", err)
	}
}

func TestSaveRoundTripOverwriteAndNoTempLeft(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sub", "deeper")
	p := filepath.Join(dir, "config.json")
	first := Config{Port: 9100, Language: "en", Overlay: Overlay{Theme: "neutral-light", Accent: "abc"}}
	if err := Save(p, first); err != nil {
		t.Fatal(err)
	}
	second := Config{Port: 9200, Overlay: Overlay{Theme: "neutral-light"}}
	if err := Save(p, second); err != nil {
		t.Fatal(err)
	}
	got, err := Load(p)
	if err != nil || got != second {
		t.Fatalf("got %+v, %v", got, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "config.json" {
		t.Fatalf("dir entries = %v", entries)
	}
}

func TestValidate(t *testing.T) {
	q := testutil.NewQuaverDir(t)
	themes := []string{"neutral-light", "dark"}
	ok := func() Config { c := Defaults(); return c }
	cases := []struct {
		name  string
		edit  func(*Config)
		field string
	}{
		{"defaults", func(c *Config) {}, ""},
		{"port min", func(c *Config) { c.Port = 1 }, ""},
		{"port max", func(c *Config) { c.Port = 65535 }, ""},
		{"port zero", func(c *Config) { c.Port = 0 }, "port"},
		{"port high", func(c *Config) { c.Port = 65536 }, "port"},
		{"port negative", func(c *Config) { c.Port = -1 }, "port"},
		{"dir quaver", func(c *Config) { c.QuaverDir = q.Path }, ""},
		{"dir missing", func(c *Config) { c.QuaverDir = filepath.Join(q.Path, "nope") }, "quaver_dir"},
		{"dir without exe", func(c *Config) { c.QuaverDir = t.TempDir() }, "quaver_dir"},
		{"dir unc backslash", func(c *Config) { c.QuaverDir = `\\server\share` }, "quaver_dir"},
		{"dir unc slash", func(c *Config) { c.QuaverDir = "//server/share" }, "quaver_dir"},
		{"dir unc mixed", func(c *Config) { c.QuaverDir = `\/server\share` }, "quaver_dir"},
		{"dir unc mixed 2", func(c *Config) { c.QuaverDir = `/\server/share` }, "quaver_dir"},
		{"dir unc extended", func(c *Config) { c.QuaverDir = `\\?\UNC\server\share` }, "quaver_dir"},
		{"dir nt unc", func(c *Config) { c.QuaverDir = `\??\UNC\server\share` }, "quaver_dir"},
		{"lang en", func(c *Config) { c.Language = "en" }, ""},
		{"lang fr", func(c *Config) { c.Language = "fr" }, ""},
		{"lang de", func(c *Config) { c.Language = "de" }, ""},
		{"lang es", func(c *Config) { c.Language = "es" }, "language"},
		{"theme known", func(c *Config) { c.Overlay.Theme = "dark" }, ""},
		{"theme empty", func(c *Config) { c.Overlay.Theme = "" }, ""},
		{"theme unknown", func(c *Config) { c.Overlay.Theme = "nope" }, "overlay.theme"},
		{"theme traversal", func(c *Config) { c.Overlay.Theme = "../x" }, "overlay.theme"},
		{"accent 3", func(c *Config) { c.Overlay.Accent = "fff" }, ""},
		{"accent 4", func(c *Config) { c.Overlay.Accent = "fff8" }, ""},
		{"accent 6", func(c *Config) { c.Overlay.Accent = "A1b2C3" }, ""},
		{"accent 8", func(c *Config) { c.Overlay.Accent = "a1b2c3d4" }, ""},
		{"accent zzz", func(c *Config) { c.Overlay.Accent = "zzz" }, "overlay.accent"},
		{"accent hash", func(c *Config) { c.Overlay.Accent = "#fff" }, "overlay.accent"},
		{"accent 5", func(c *Config) { c.Overlay.Accent = "fffff" }, "overlay.accent"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg := ok()
			c.edit(&cfg)
			err := cfg.Validate(themes)
			if c.field == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			var fe *FieldError
			if !errors.As(err, &fe) || fe.Field != c.field || fe.Error() == "" {
				t.Fatalf("err = %v, want field %q", err, c.field)
			}
		})
	}
}

func TestLoadStripsUTF8BOM(t *testing.T) {
	p := write(t, "\xEF\xBB\xBF"+`{"port": 9000, "language": "fr"}`)
	c, err := Load(p)
	if err != nil || c.Port != 9000 || c.Language != "fr" {
		t.Fatalf("c = %+v err = %v", c, err)
	}
	if c, err := Load(write(t, "\xEF\xBB\xBF")); err != nil || c.Port != DefaultPort {
		t.Fatalf("BOM only: c = %+v err = %v", c, err)
	}
}

func TestNormalize(t *testing.T) {
	got := Normalize(Config{Language: "fr"})
	if got.Port != DefaultPort || got.Overlay.Theme != "neutral-light" || got.Language != "fr" {
		t.Fatalf("got %+v", got)
	}
	keep := Config{Port: 9, Overlay: Overlay{Theme: "dark"}}
	if Normalize(keep) != keep {
		t.Fatal("set fields must be kept")
	}
}
