package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
)

var fuzzThemes = []string{"neutral-light", "neutral-dark"}

// FuzzLoad: any file content either fails to load or gives a config that Normalize leaves unchanged,
// and Validate never panics on it.
func FuzzLoad(f *testing.F) {
	for _, s := range []string{"", "\xEF\xBB\xBF{}", `{"port":0}`, `{"keys":{"k4":[]}}`, `{"overlay":null}`, `[1]`, `{"port":1e3}`} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		p := filepath.Join(t.TempDir(), "config.json")
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
		c, err := Load(p)
		if err != nil {
			return
		}
		if n := Normalize(c); !reflect.DeepEqual(n, c) {
			t.Fatalf("Load result not normalized: %+v vs %+v", c, n)
		}
		c.QuaverDir = "" // never stat an arbitrary path
		_ = c.Validate(fuzzThemes)
	})
}

// FuzzMergeObjects: every key of next ends up with next's value (deep merged for objects),
// every key of old missing from next is kept unchanged.
func FuzzMergeObjects(f *testing.F) {
	f.Add([]byte(`{"a":1,"o":{"x":1,"y":2}}`), []byte(`{"o":{"x":3},"b":true}`))
	f.Add([]byte(`{"o":"s"}`), []byte(`{"o":{"x":1}}`))
	f.Add([]byte(`{"o":{"x":null}}`), []byte(`{"o":null}`))
	f.Add([]byte(`{"port":null}`), []byte(`{"port":7878}`))
	f.Add([]byte(`{"o":{"x":1}}`), []byte(`{"o":null}`))
	f.Add([]byte(`{"O":"s","keyX":1}`), []byte(`{"o":{},"keyx":2}`))
	f.Fuzz(func(t *testing.T, a, b []byte) {
		var old, next object
		if json.Unmarshal(a, &old) != nil || json.Unmarshal(b, &next) != nil || next == nil {
			return
		}
		out := mergeObjects(old, next)
		for k, v := range old {
			_, inNext := next[k]
			if foldsToAnother(k, next) {
				if _, kept := out[k]; kept {
					t.Fatalf("old key %q folds to a key of next but was kept", k)
				}
				continue
			}
			if !inNext && string(out[k]) != string(v) {
				t.Fatalf("old key %q changed: %s -> %s", k, v, out[k])
			}
		}
		for k, v := range next {
			got, ok := out[k]
			if !ok {
				t.Fatalf("next key %q lost", k)
			}
			var nv object
			if json.Unmarshal(v, &nv) == nil && nv != nil {
				var gv object
				if json.Unmarshal(got, &gv) != nil {
					t.Fatalf("object key %q is no longer an object: %s", k, got)
				}
				for nk, nvv := range nv {
					var x, y any
					if json.Unmarshal(gv[nk], &x) != nil || json.Unmarshal(nvv, &y) != nil {
						t.Fatalf("nested %q.%q unreadable", k, nk)
					}
					if !reflect.DeepEqual(x, y) && !isObject(nvv) {
						t.Fatalf("nested %q.%q = %s, want %s", k, nk, gv[nk], nvv)
					}
				}
				continue
			}
			if string(got) != string(v) {
				t.Fatalf("key %q = %s, want %s", k, got, v)
			}
		}
	})
}

func isObject(raw json.RawMessage) bool {
	var o object
	return json.Unmarshal(raw, &o) == nil && o != nil
}

// FuzzValidate never panics and agrees with the documented accent rule.
func FuzzValidate(f *testing.F) {
	f.Add(7878, "en", "neutral-light", "fff", "auto", true)
	f.Add(0, "xx", "nope", "#fff", "11k", false)
	f.Add(7878, "en", "neutral-dark", "#abc", "10k", true)
	f.Add(7878, "de", "", "", "01k", true)
	f.Add(7878, "fr", "", "", "0k", true)
	f.Fuzz(func(t *testing.T, port int, lang, theme, accent, layout string, enabled bool) {
		c := Defaults()
		c.Port, c.Language, c.Overlay.Theme, c.Overlay.Accent = port, lang, theme, accent
		c.Keys.Layout, c.Keys.Enabled = layout, enabled
		err := c.Validate(fuzzThemes)
		valid := port >= 1 && port <= 65535 &&
			(lang == "" || lang == "en" || lang == "fr" || lang == "de") &&
			(theme == "" || theme == "neutral-light" || theme == "neutral-dark") &&
			(accent == "" || accentRe.MatchString(accent)) &&
			validLayoutName(layout)
		if (err == nil) != valid {
			t.Fatalf("Validate(%+v) = %v, want valid=%v", c, err, valid)
		}
		if err != nil {
			if _, ok := err.(*FieldError); !ok {
				t.Fatalf("error %T is not a *FieldError", err)
			}
		}
	})
}

func validLayoutName(l string) bool {
	if l == "auto" {
		return true
	}
	for n := 1; n <= 10; n++ {
		if l == strconv.Itoa(n)+"k" {
			return true
		}
	}
	return false
}
