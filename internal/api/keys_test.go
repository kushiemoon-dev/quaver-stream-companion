package api

import (
	"strings"
	"testing"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/config"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/keys"
)

func withKeys(st keys.Status, c keys.Config) opt {
	return func(d *Deps) {
		d.Keys = func() keys.Status { return st }
		d.KeysConfig = c
	}
}

func statusKeys(t *testing.T, e env) (keys.Status, string) {
	t.Helper()
	w := e.get("/api/status")
	var got struct {
		Keys *keys.Status `json:"keys"`
	}
	decode(t, w, &got)
	if got.Keys == nil {
		t.Fatalf("status has no keys item: %s", w.Body.String())
	}
	return *got.Keys, w.Body.String()
}

func TestStatusReportsKeysItem(t *testing.T) {
	c := keys.Defaults()
	c.Enabled, c.Layout = true, "7k"
	e := newEnv(t, withKeys(keys.Status{Enabled: true, Supported: true, Layout: "7k"}, c))
	got, _ := statusKeys(t, e)
	if got != (keys.Status{Enabled: true, Supported: true, Layout: "7k"}) {
		t.Fatalf("keys = %+v", got)
	}
}

func TestStatusKeysItemWhenNotWired(t *testing.T) {
	got, _ := statusKeys(t, newEnv(t))
	if got.Enabled || got.Supported || got.Layout != "auto" {
		t.Fatalf("keys = %+v", got)
	}
}

func TestStatusNeverContainsKeyNames(t *testing.T) {
	c := keys.Defaults()
	c.Enabled = true
	c.K4 = []keys.Key{"F9", "F10", "F11", "F12"}
	e := newEnv(t, withKeys(keys.Status{Enabled: true, Supported: true, Layout: "auto"}, c))
	_, body := statusKeys(t, e)
	for _, name := range []string{"F9", "F10", "F11", "F12", "SPACE"} {
		if strings.Contains(body, name) {
			t.Fatalf("status leaks %q: %s", name, body)
		}
	}
}

func TestStatusAddsKeysCheckOnlyWhenEnabled(t *testing.T) {
	ids := func(e env) []string {
		var got struct {
			Checks []struct {
				ID string `json:"id"`
				OK bool   `json:"ok"`
			} `json:"checks"`
		}
		decode(t, e.get("/api/status"), &got)
		var out []string
		for _, c := range got.Checks {
			out = append(out, c.ID)
		}
		return out
	}
	if got := ids(newEnv(t)); len(got) != 4 {
		t.Fatalf("disabled: checks = %v", got)
	}
	c := keys.Defaults()
	c.Enabled = true
	got := ids(newEnv(t, withKeys(keys.Status{Enabled: true, Supported: true, Layout: "auto"}, c)))
	if len(got) != 5 || got[4] != "keys" {
		t.Fatalf("enabled: checks = %v", got)
	}
}

func TestPutKeysRestartRequiredFollowsRunningSettings(t *testing.T) {
	run := keys.Defaults()
	run.Enabled = true
	e := newEnv(t, withKeys(keys.Status{Enabled: true, Supported: true, Layout: "auto"}, run))
	base := `"quaver_dir":"` + jsonPath(e.quaver) + `","port":5555`
	put := func(keysJSON string) bool {
		t.Helper()
		w := e.put(`{`+base+`,"keys":`+keysJSON+`}`, nil)
		var got struct {
			RestartRequired bool `json:"restart_required"`
		}
		decode(t, w, &got)
		if w.Code != 200 {
			t.Fatalf("code %d: %s", w.Code, w.Body.String())
		}
		return got.RestartRequired
	}
	if put(`{"enabled":true}`) {
		t.Fatal("same keys settings (defaults filled in) must not need a restart")
	}
	if !put(`{"enabled":false}`) {
		t.Fatal("turning sampling off needs a restart")
	}
	if !put(`{"enabled":true,"layout":"7k"}`) {
		t.Fatal("changing the layout needs a restart")
	}
	if !put(`{"enabled":true,"k4":["Q","W","E","R"]}`) {
		t.Fatal("changing the keys needs a restart")
	}
}

func TestPutWithoutKeysSectionSavesDefaults(t *testing.T) {
	e := newEnv(t)
	w := e.put(`{"port":5555}`, nil)
	var got struct {
		Config config.Config `json:"config"`
	}
	decode(t, w, &got)
	if w.Code != 200 || got.Config.Keys.Enabled || len(got.Config.Keys.K4) != 4 || len(got.Config.Keys.K7) != 7 || got.Config.Keys.Layout != "auto" {
		t.Fatalf("code %d: %s", w.Code, w.Body.String())
	}
	saved, err := config.Load(e.cfgPath)
	if err != nil || saved.Keys.Enabled || saved.Keys.Layout != "auto" {
		t.Fatalf("saved = %+v err = %v", saved.Keys, err)
	}
}

func TestPutRejectsInvalidKeysWithFieldAndWritesNothing(t *testing.T) {
	cases := []struct{ name, keys, field string }{
		{"layout", `{"layout":"5k"}`, "keys.layout"},
		{"unknown name", `{"k4":["A","S","K","ENTER"]}`, "keys.k4"},
		{"wrong count", `{"k4":["A","S","K"]}`, "keys.k4"},
		{"duplicate", `{"k7":["A","S","D","D","J","K","L"]}`, "keys.k7"},
		{"k7 wrong count", `{"k7":["A","S","D","J"]}`, "keys.k7"},
		{"wrong type", `{"enabled":"yes"}`, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t)
			w := e.put(`{"port":5555,"keys":`+c.keys+`}`, nil)
			var er errBody
			decode(t, w, &er)
			if w.Code != 400 || er.Field != c.field || er.Error == "" {
				t.Fatalf("code %d body %+v", w.Code, er)
			}
			e.noFile(t)
		})
	}
}

func TestConfigRoundTripKeepsKeys(t *testing.T) {
	e := newEnv(t)
	body := `{"port":5555,"keys":{"enabled":true,"layout":"4k","k4":["Q","W","E","R"],"k7":["A","S","D","SPACE","J","K","L"]}}`
	if w := e.put(body, nil); w.Code != 200 {
		t.Fatalf("put: %d %s", w.Code, w.Body.String())
	}
	var cfg struct {
		Config config.Config `json:"config"`
	}
	decode(t, e.get("/api/config"), &cfg)
	k := cfg.Config.Keys
	if !k.Enabled || k.Layout != "4k" || k.K4[0] != "Q" {
		t.Fatalf("keys = %+v", k)
	}
}

func TestPutExplicitEmptyKeyListsAreRejected(t *testing.T) {
	for field, body := range map[string]string{
		"keys.k4": `{"port":5555,"keys":{"k4":[]}}`,
		"keys.k7": `{"port":5555,"keys":{"k7":[]}}`,
	} {
		e := newEnv(t)
		w := e.put(body, nil)
		var er errBody
		decode(t, w, &er)
		if w.Code != 400 || er.Field != field || !strings.Contains(er.Error, "exactly") {
			t.Fatalf("%s: code %d body %+v", field, w.Code, er)
		}
		e.noFile(t)
	}
}

func TestPutAbsentOrNullKeyListsTakeDefaults(t *testing.T) {
	e := newEnv(t)
	w := e.put(`{"port":5555,"keys":{"enabled":false,"k4":null}}`, nil)
	var got struct {
		Config config.Config `json:"config"`
	}
	decode(t, w, &got)
	if w.Code != 200 || len(got.Config.Keys.K4) != 4 || len(got.Config.Keys.K7) != 7 {
		t.Fatalf("code %d: %s", w.Code, w.Body.String())
	}
}

func TestRestartRequiredOnlyWhenKeysChangeAndSamplingIsOnBeforeOrAfter(t *testing.T) {
	e := newEnv(t, withKeys(keys.Status{Layout: "auto"}, keys.Defaults()))
	base := `"quaver_dir":"` + jsonPath(e.quaver) + `","port":5555`
	restart := func(keysJSON string) bool {
		t.Helper()
		w := e.put(`{`+base+`,"keys":`+keysJSON+`}`, nil)
		var got struct {
			RestartRequired bool `json:"restart_required"`
		}
		decode(t, w, &got)
		if w.Code != 200 {
			t.Fatalf("code %d: %s", w.Code, w.Body.String())
		}
		return got.RestartRequired
	}
	if restart(`{"enabled":false,"layout":"7k","k4":["Q","W","E","R"]}`) {
		t.Fatal("editing the keys while sampling stays off needs no restart")
	}
	if !restart(`{"enabled":true}`) {
		t.Fatal("turning sampling on needs a restart")
	}
}
