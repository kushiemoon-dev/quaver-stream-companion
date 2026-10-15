package keys

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestDefaultsAreOffAndComplete(t *testing.T) {
	d := Defaults()
	if d.Enabled || d.Layout != "auto" {
		t.Fatalf("defaults = %+v", d)
	}
	if !reflect.DeepEqual(d.K4, []Key{"A", "S", "K", "L"}) || !reflect.DeepEqual(d.K7, []Key{"A", "S", "D", "SPACE", "J", "K", "L"}) {
		t.Fatalf("defaults = %+v", d)
	}
	if f, m := d.Check(); f != "" {
		t.Fatalf("defaults invalid: %s %s", f, m)
	}
}

func TestDefaultsReturnFreshSlices(t *testing.T) {
	a := Defaults()
	a.K4[0] = "Z"
	if Defaults().K4[0] != "A" {
		t.Fatal("defaults share their slices")
	}
}

func TestNormalizedFillsEmptyFieldsOnly(t *testing.T) {
	got := Config{Enabled: true, K4: []Key{"Q", "W", "E", "R"}}.Normalized()
	want := Defaults()
	want.Enabled, want.K4 = true, []Key{"Q", "W", "E", "R"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
}

func TestCheckFieldErrors(t *testing.T) {
	cases := []struct {
		name  string
		edit  func(*Config)
		field string
	}{
		{"layout", func(c *Config) { c.Layout = "11k" }, "keys.layout"},
		{"k4 too short", func(c *Config) { c.K4 = []Key{"A", "S", "K"} }, "keys.k4"},
		{"k4 too long", func(c *Config) { c.K4 = []Key{"A", "S", "K", "L", "M"} }, "keys.k4"},
		{"k4 unknown name", func(c *Config) { c.K4[1] = "ENTER" }, "keys.k4"},
		{"k4 lowercase", func(c *Config) { c.K4[0] = "a" }, "keys.k4"},
		{"k4 duplicate", func(c *Config) { c.K4[3] = "A" }, "keys.k4"},
		{"k7 too short", func(c *Config) { c.K7 = c.K7[:6] }, "keys.k7"},
		{"k7 unknown name", func(c *Config) { c.K7[2] = "F13" }, "keys.k7"},
		{"k7 duplicate", func(c *Config) { c.K7[6] = "A" }, "keys.k7"},
	}
	for _, tc := range cases {
		c := Defaults()
		tc.edit(&c)
		if f, m := c.Check(); f != tc.field || m == "" {
			t.Errorf("%s: field = %q msg = %q, want field %q", tc.name, f, m, tc.field)
		}
	}
}

func TestCheckAcceptsEveryWhitelistedName(t *testing.T) {
	names := []Key{"A", "Z", "0", "9", "SPACE", "LEFT", "RIGHT", "UP", "DOWN", "SHIFT_L", "SHIFT_R", "CTRL_L", "CTRL_R", "ALT_L", "ALT_R", ";", ",", ".", "/", "'", "F1", "F12"}
	for _, n := range names {
		if !Valid(n) {
			t.Errorf("%q should be valid", n)
		}
	}
	for _, n := range []Key{"", "ENTER", "F0", "F13", "a", "AA", "\\", "TAB"} {
		if Valid(n) {
			t.Errorf("%q should be invalid", n)
		}
	}
}

func TestSameLaneKeyMayAppearInBothLayouts(t *testing.T) {
	if f, _ := Defaults().Check(); f != "" {
		t.Fatal("default layouts share A, S, K, L and must be valid")
	}
}

func TestJSONRoundTrip(t *testing.T) {
	c := Config{Enabled: true, Layout: "7k", Scratch: "CTRL_L", K4: []Key{"F1", "F2", "F3", "F4"}, K7: []Key{"A", "S", "D", "SPACE", "J", "K", "L"}}
	raw, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"enabled":true`) || !strings.Contains(string(raw), `"k4":["F1"`) {
		t.Fatalf("json = %s", raw)
	}
	var back Config
	if err := json.Unmarshal(raw, &back); err != nil || !reflect.DeepEqual(back, c) {
		t.Fatalf("back = %+v err = %v", back, err)
	}
}

func TestExplicitEmptyListIsKeptAndRejected(t *testing.T) {
	var c Config
	if err := json.Unmarshal([]byte(`{"enabled": true, "k4": [], "k7": []}`), &c); err != nil {
		t.Fatal(err)
	}
	n := c.Normalized()
	if n.K4 == nil || len(n.K4) != 0 || n.K7 == nil || len(n.K7) != 0 {
		t.Fatalf("explicit empty lists were replaced: %+v", n)
	}
	if f, _ := n.Check(); f != "keys.k4" {
		t.Fatalf("field = %q", f)
	}
	n.K4 = Defaults().K4
	if f, _ := n.Check(); f != "keys.k7" {
		t.Fatalf("field = %q", f)
	}
}

func TestAbsentListsTakeDefaults(t *testing.T) {
	var c Config
	if err := json.Unmarshal([]byte(`{"enabled": true, "k4": null}`), &c); err != nil {
		t.Fatal(err)
	}
	n := c.Normalized()
	if !reflect.DeepEqual(n.K4, Defaults().K4) || !reflect.DeepEqual(n.K7, Defaults().K7) {
		t.Fatalf("got %+v", n)
	}
}
