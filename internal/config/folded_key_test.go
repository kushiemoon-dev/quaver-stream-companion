package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/keys"
)

// foldedKeyFiles hold a key that encoding/json reads as a known key (it folds case and
// the long s and the Kelvin sign) and that sorts after the real one.
func foldedKeyFiles() []string {
	long, kelvin := string(rune(0x17f)), string(rune(0x212a))
	return []string{
		`{"key` + long + `": {"enabled": true}}`,
		`{"Keys": {"enabled": true}}`,
		`{"` + kelvin + `eys": {"enabled": true}}`,
		`{"keys": {"K4": ["Q","W","E","R"]}}`,
		`{"keys": {"` + kelvin + `4": ["Q","W","E","R"]}}`,
		`{"Port": 1, "keys": {"enabled": true}}`,
	}
}

func TestSaveIsNotShadowedByAFoldedKey(t *testing.T) {
	for _, old := range foldedKeyFiles() {
		p := filepath.Join(t.TempDir(), "config.json")
		if err := os.WriteFile(p, []byte(old), 0o644); err != nil {
			t.Fatal(err)
		}
		c := Defaults()
		c.Port = 9001
		c.Keys.Enabled = false
		c.Keys.K4 = []keys.Key{"A", "S", "K", "L"}
		if err := Save(p, c); err != nil {
			t.Fatal(err)
		}
		got, err := Load(p)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, Normalize(c)) {
			t.Errorf("old file %q: saved %+v, loaded %+v", old, Normalize(c), got)
		}
	}
}

func TestSaveKeepsUnknownKeysThatDoNotFoldToAKnownOne(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(p, []byte(`{"extra": 1, "keys": {"future": true}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Save(p, Defaults()); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"extra"`, `"future"`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("saved file lost %s: %s", want, data)
		}
	}
}

// FuzzSaveLoadRoundTrip: whatever the previous file held, Load after Save returns what was saved.
func FuzzSaveLoadRoundTrip(f *testing.F) {
	f.Add([]byte(`{"port":1,"extra":{"a":1},"overlay":{"theme":"x","future":true}}`), 8080, "fr", true)
	for _, old := range foldedKeyFiles() {
		f.Add([]byte(old), 7878, "en", false)
	}
	f.Fuzz(func(t *testing.T, old []byte, port int, lang string, enabled bool) {
		if !utf8.ValidString(lang) {
			t.Skip()
		}
		p := filepath.Join(t.TempDir(), "config.json")
		if err := os.WriteFile(p, old, 0o644); err != nil {
			t.Fatal(err)
		}
		c := Defaults()
		c.Port = port%65535 + 1
		if c.Port < 1 {
			c.Port = -c.Port + 1
		}
		c.Language, c.Keys.Enabled = lang, enabled
		if err := Save(p, c); err != nil {
			t.Fatal(err)
		}
		got, err := Load(p)
		if err != nil {
			t.Fatalf("saved file does not load: %v", err)
		}
		if !reflect.DeepEqual(got, Normalize(c)) {
			t.Fatalf("old %q: saved %+v, loaded %+v", old, Normalize(c), got)
		}
	})
}
