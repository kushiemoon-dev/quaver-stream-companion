package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/keys"
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
		{"partial", `{"port": 9000}`, Config{Port: 9000, Overlay: Overlay{Theme: "neutral-light"}, Keys: keys.Defaults(), WatchQuaverProcess: true}},
		{"zero port", `{"port": 0, "language": "fr"}`, Config{Port: DefaultPort, Language: "fr", Overlay: Overlay{Theme: "neutral-light"}, Keys: keys.Defaults(), WatchQuaverProcess: true}},
		{"empty theme", `{"overlay": {"theme": "", "accent": "ff00aa"}}`, Config{Port: DefaultPort, Overlay: Overlay{Theme: "neutral-light", Accent: "ff00aa"}, Keys: keys.Defaults(), WatchQuaverProcess: true}},
		{"unknown fields", `{"port": 9001, "extra": true, "overlay": {"x": 1}}`, Config{Port: 9001, Overlay: Overlay{Theme: "neutral-light"}, Keys: keys.Defaults(), WatchQuaverProcess: true}},
		{"full", `{"quaver_dir":"/q","port":8000,"language":"de","overlay":{"theme":"dark","accent":"fff"}}`, Config{QuaverDir: "/q", Port: 8000, Language: "de", Overlay: Overlay{Theme: "dark", Accent: "fff"}, Keys: keys.Defaults(), WatchQuaverProcess: true}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Load(write(t, c.content))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Fatalf("got %+v, want %+v", got, c.want)
			}
		})
	}
}

func TestLoadMissingFileGivesDefaults(t *testing.T) {
	got, err := Load(filepath.Join(t.TempDir(), "nope", "config.json"))
	if err != nil || !reflect.DeepEqual(got, Defaults()) {
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
	first := Config{Port: 9100, Language: "en", Overlay: Overlay{Theme: "neutral-light", Accent: "abc"}, Keys: keys.Defaults()}
	if err := Save(p, first); err != nil {
		t.Fatal(err)
	}
	second := Config{Port: 9200, Overlay: Overlay{Theme: "neutral-light"}, Keys: keys.Defaults()}
	if err := Save(p, second); err != nil {
		t.Fatal(err)
	}
	got, err := Load(p)
	if err != nil || !reflect.DeepEqual(got, second) {
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
	keep := Config{Port: 9, Overlay: Overlay{Theme: "dark"}, Keys: keys.Defaults()}
	if !reflect.DeepEqual(Normalize(keep), keep) {
		t.Fatal("set fields must be kept")
	}
}

func TestDefaultsKeepSamplingOff(t *testing.T) {
	if k := Defaults().Keys; k.Enabled || !reflect.DeepEqual(k, keys.Defaults()) {
		t.Fatalf("keys defaults = %+v", k)
	}
}

func TestLoadKeysSection(t *testing.T) {
	got, err := Load(write(t, `{"keys": {"enabled": true, "k4": ["Q","W","E","R"]}}`))
	if err != nil {
		t.Fatal(err)
	}
	want := keys.Defaults()
	want.Enabled, want.K4 = true, []keys.Key{"Q", "W", "E", "R"}
	if !reflect.DeepEqual(got.Keys, want) {
		t.Fatalf("keys = %+v, want %+v", got.Keys, want)
	}
}

func TestKeysSaveRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	c := Defaults()
	c.Keys = keys.Config{Enabled: true, Layout: "7k", K4: []keys.Key{"F1", "F2", "F3", "F4"}, K7: []keys.Key{"Q", "W", "E", "SPACE", "I", "O", "P"}}.Normalized()
	if err := Save(p, c); err != nil {
		t.Fatal(err)
	}
	got, err := Load(p)
	if err != nil || !reflect.DeepEqual(got, c) {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestValidateKeysFields(t *testing.T) {
	cases := []struct {
		name  string
		edit  func(*Config)
		field string
	}{
		{"layout", func(c *Config) { c.Keys.Layout = "11k" }, "keys.layout"},
		{"bad name", func(c *Config) { c.Keys.K4[0] = "ENTER" }, "keys.k4"},
		{"wrong count", func(c *Config) { c.Keys.K7 = c.Keys.K7[:5] }, "keys.k7"},
		{"duplicate", func(c *Config) { c.Keys.K4[1] = "A" }, "keys.k4"},
	}
	for _, tc := range cases {
		c := Defaults()
		tc.edit(&c)
		var fe *FieldError
		if err := c.Validate([]string{"neutral-light"}); !errors.As(err, &fe) || fe.Field != tc.field {
			t.Errorf("%s: err = %v, want field %q", tc.name, err, tc.field)
		}
	}
}

func TestSaveWhileAnotherGoroutineLoads(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	if err := Save(p, Defaults()); err != nil {
		t.Fatal(err)
	}
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			default:
				Load(p)
				time.Sleep(time.Millisecond)
			}
		}
	}()
	defer func() { close(stop); <-done }()
	for i := 0; i < 50; i++ {
		if err := Save(p, Defaults()); err != nil {
			t.Fatalf("save %d: %v", i, err)
		}
	}
}

func readFileObject(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("saved file is not JSON: %v\n%s", err, data)
	}
	return m
}

func TestSaveKeepsKeysThisVersionDoesNotKnow(t *testing.T) {
	p := write(t, `{"port": 9000, "stream_deck": {"slot": 3, "label": "x"}, "overlay": {"theme": "neutral-light", "font_scale": 1.5}, "keys": {"enabled": false, "haptics": true}}`)
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	c.Port = 9100
	c.Overlay.Accent = "ff8800"
	if err := Save(p, c); err != nil {
		t.Fatal(err)
	}

	got := readFileObject(t, p)
	if got["port"] != float64(9100) {
		t.Fatalf("port = %v", got["port"])
	}
	deck, _ := got["stream_deck"].(map[string]any)
	if deck["slot"] != float64(3) || deck["label"] != "x" {
		t.Fatalf("stream_deck = %v", got["stream_deck"])
	}
	if ov, _ := got["overlay"].(map[string]any); ov["font_scale"] != 1.5 || ov["accent"] != "ff8800" {
		t.Fatalf("overlay = %v", got["overlay"])
	}
	if ks, _ := got["keys"].(map[string]any); ks["haptics"] != true {
		t.Fatalf("keys = %v", got["keys"])
	}
}

func TestSaveWritesTheVersionAndNeverLowersIt(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	if err := Save(p, Defaults()); err != nil {
		t.Fatal(err)
	}
	if v := readFileObject(t, p)["version"]; v != float64(1) {
		t.Fatalf("version = %v, want 1", v)
	}

	newer := write(t, `{"version": 2, "port": 9000, "shiny": true}`)
	c, err := Load(newer)
	if err != nil {
		t.Fatalf("a newer file must still load: %v", err)
	}
	if err := Save(newer, c); err != nil {
		t.Fatal(err)
	}
	got := readFileObject(t, newer)
	if got["version"] != float64(2) || got["shiny"] != true {
		t.Fatalf("saved = %v", got)
	}
}

func TestSaveOverACorruptFileStillWorks(t *testing.T) {
	p := write(t, `{not json`)
	if err := Save(p, Defaults()); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(p); err != nil {
		t.Fatal(err)
	}
}

func TestSaveFlushesTheTempFileBeforeTheRename(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	var flushed []string
	old := syncFile
	syncFile = func(f *os.File) error {
		if _, err := os.Stat(p); err == nil {
			t.Error("the target was replaced before the temp file was flushed")
		}
		flushed = append(flushed, f.Name())
		return f.Sync()
	}
	t.Cleanup(func() { syncFile = old })

	if err := Save(p, Defaults()); err != nil {
		t.Fatal(err)
	}
	if len(flushed) != 1 || !strings.HasSuffix(flushed[0], ".tmp") {
		t.Fatalf("flushed = %v", flushed)
	}
}

func TestSaveReportsAFailedFlush(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	old := syncFile
	syncFile = func(*os.File) error { return errors.New("disk full") }
	t.Cleanup(func() { syncFile = old })

	if err := Save(p, Defaults()); err == nil {
		t.Fatal("a flush error must fail the save")
	}
	if _, err := os.Stat(p); err == nil {
		t.Fatal("no config file may appear after a failed flush")
	}
	if left, _ := filepath.Glob(filepath.Join(filepath.Dir(p), ".config-*.tmp")); len(left) != 0 {
		t.Fatalf("temp files left: %v", left)
	}
}
