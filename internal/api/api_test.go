package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/config"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/scores"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/state"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/testutil"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/themes"
	"github.com/kushiemoon-dev/quaver-stream-companion/web"
)

type env struct {
	h       http.Handler
	cfgPath string
	quaver  string
}

type opt func(*Deps)

func newEnv(t *testing.T, opts ...opt) env {
	t.Helper()
	q := testutil.NewQuaverDir(t)
	cfgPath := filepath.Join(t.TempDir(), "sub", "config.json")
	d := Deps{
		Version:  "test-1",
		Started:  time.Now().Add(-5 * time.Second),
		Snapshot: func() state.Snapshot { return state.Snapshot{Game: "menu", Map: &scores.Map{Title: "Song"}} },
		History: func(ctx context.Context, limit int) ([]scores.Result, error) {
			return nil, nil
		},
		ConfigPath: cfgPath,
		Effective:  func() string { return q.Path },
		Detect:     func() string { return q.Path },
		Themes:     func() []themes.Theme { return testThemes },
		Port:       testPort,
	}
	for _, o := range opts {
		o(&d)
	}
	return env{h: GuardHost(testPort, New(d)), cfgPath: cfgPath, quaver: q.Path}
}

func (e env) get(path string) *httptest.ResponseRecorder {
	return do(e.h, "GET", "127.0.0.1:5555", path, "", nil)
}

func (e env) put(body string, hdr map[string]string) *httptest.ResponseRecorder {
	h := map[string]string{"Content-Type": "application/json", "X-Requested-With": "qsc"}
	for k, v := range hdr {
		if v == "" {
			delete(h, k)
		} else {
			h[k] = v
		}
	}
	return do(e.h, "PUT", "127.0.0.1:5555", "/api/config", body, h)
}

func decode(t *testing.T, w *httptest.ResponseRecorder, v any) {
	t.Helper()
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q (body %q)", ct, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), v); err != nil {
		t.Fatalf("bad JSON %q: %v", w.Body.String(), err)
	}
}

func (e env) noFile(t *testing.T) {
	t.Helper()
	if _, err := os.Stat(e.cfgPath); !os.IsNotExist(err) {
		t.Fatalf("config file must not exist, stat err = %v", err)
	}
	if entries, _ := os.ReadDir(filepath.Dir(e.cfgPath)); len(entries) > 0 {
		t.Fatalf("config folder must stay empty, has %d entries", len(entries))
	}
}

func jsonPath(p string) string { return strings.ReplaceAll(p, `\`, `\\`) }

func TestStatus(t *testing.T) {
	e := newEnv(t)
	w := e.get("/api/status")
	if w.Code != 200 {
		t.Fatalf("code %d: %s", w.Code, w.Body.String())
	}
	var got struct {
		Version   string         `json:"version"`
		Game      string         `json:"game"`
		Map       *scores.Map    `json:"map"`
		LastScore *scores.Result `json:"last_score"`
		Checks    []struct {
			ID string `json:"id"`
			OK bool   `json:"ok"`
		} `json:"checks"`
		ConfigPath string `json:"config_path"`
		UptimeS    int64  `json:"uptime_s"`
	}
	decode(t, w, &got)
	if got.Version != "test-1" || got.Game != "menu" || got.Map == nil || got.ConfigPath != e.cfgPath || got.UptimeS < 5 {
		t.Fatalf("status = %+v", got)
	}
	if len(got.Checks) != 4 {
		t.Fatalf("checks = %+v", got.Checks)
	}
	for i, id := range []string{"quaver_dir", "log", "database", "port"} {
		if got.Checks[i].ID != id || !got.Checks[i].OK {
			t.Errorf("check %d = %+v, want %s ok", i, got.Checks[i], id)
		}
	}
}

func TestConfigGetEffective(t *testing.T) {
	e := newEnv(t)
	if err := config.Save(e.cfgPath, config.Config{Port: testPort, Overlay: config.Overlay{Theme: "dark"}}); err != nil {
		t.Fatal(err)
	}
	w := e.get("/api/config")
	var got struct {
		Config    config.Config `json:"config"`
		Effective struct {
			QuaverDir string `json:"quaver_dir"`
			Port      int    `json:"port"`
		} `json:"effective"`
		RestartRequired bool `json:"restart_required"`
	}
	decode(t, w, &got)
	if w.Code != 200 || got.Effective.QuaverDir != e.quaver || got.Effective.Port != testPort || got.RestartRequired {
		t.Fatalf("code %d got %+v", w.Code, got)
	}
	if got.Config.QuaverDir != "" || got.Config.Port != testPort || got.Config.Overlay.Theme != "dark" {
		t.Fatalf("config = %+v", got.Config)
	}
}

func TestConfigGetNoFileShowsDefaultsAndRestart(t *testing.T) {
	e := newEnv(t)
	var got struct {
		Config          config.Config `json:"config"`
		RestartRequired bool          `json:"restart_required"`
	}
	decode(t, e.get("/api/config"), &got)
	if got.Config.Port != config.DefaultPort || !got.RestartRequired {
		t.Fatalf("got %+v", got)
	}
}

func TestConfigGetBrokenFile(t *testing.T) {
	e := newEnv(t)
	os.MkdirAll(filepath.Dir(e.cfgPath), 0o755)
	os.WriteFile(e.cfgPath, []byte("{nope"), 0o644)
	w := e.get("/api/config")
	var er errBody
	decode(t, w, &er)
	if w.Code != 500 || er.Error != "could not read the config file" {
		t.Fatalf("code %d body %s", w.Code, w.Body.String())
	}
}

func TestPutValidWritesFile(t *testing.T) {
	e := newEnv(t)
	body := `{"quaver_dir":"` + jsonPath(e.quaver) + `","port":5555,"language":"fr","overlay":{"theme":"dark","accent":"a855f7"}}`
	w := e.put(body, nil)
	var got struct {
		Config          config.Config `json:"config"`
		RestartRequired bool          `json:"restart_required"`
	}
	decode(t, w, &got)
	if w.Code != 200 || got.Config.Language != "fr" || got.RestartRequired {
		t.Fatalf("code %d got %+v", w.Code, got)
	}
	saved, err := config.Load(e.cfgPath)
	if err != nil || saved.Language != "fr" || saved.Overlay.Theme != "dark" || saved.QuaverDir != e.quaver {
		t.Fatalf("saved = %+v err %v", saved, err)
	}
}

func TestPutRestartRequired(t *testing.T) {
	var got struct {
		RestartRequired bool `json:"restart_required"`
	}

	e := newEnv(t)
	w := e.put(`{"port":5555,"language":"de","overlay":{"theme":"dark"}}`, nil)
	decode(t, w, &got)
	if w.Code != 200 || got.RestartRequired {
		t.Fatalf("no change: code %d %s", w.Code, w.Body.String())
	}

	w = e.put(`{"port":6000,"overlay":{"theme":"dark"}}`, nil)
	decode(t, w, &got)
	if w.Code != 200 || !got.RestartRequired {
		t.Fatalf("port change: code %d %s", w.Code, w.Body.String())
	}

	other := testutil.NewQuaverDir(t).Path
	w = e.put(`{"port":5555,"quaver_dir":"`+jsonPath(other)+`"}`, nil)
	decode(t, w, &got)
	if w.Code != 200 || !got.RestartRequired {
		t.Fatalf("dir change: code %d %s", w.Code, w.Body.String())
	}

	w = e.put(`{"port":5555,"quaver_dir":"`+jsonPath(e.quaver)+`"}`, nil)
	decode(t, w, &got)
	if w.Code != 200 || got.RestartRequired {
		t.Fatalf("dir equal to the one in use: code %d %s", w.Code, w.Body.String())
	}
}

type cfgResp struct {
	Config          config.Config `json:"config"`
	RestartRequired bool          `json:"restart_required"`
	Overridden      *[]string     `json:"overridden"`
}

func TestOverriddenByFlags(t *testing.T) {
	other := testutil.NewQuaverDir(t).Path
	otherBody := `"quaver_dir":"` + jsonPath(other) + `"`
	cases := []struct {
		name string
		opt  opt
		body string
		want []string
		rr   bool
	}{
		{"none", func(d *Deps) {}, `{"port":5555}`, []string{}, false},
		{"port flag", func(d *Deps) { d.PortFlag = true }, `{"port":6000}`, []string{"port"}, false},
		{"dir flag", func(d *Deps) { d.QuaverDirFlag = true }, `{"port":5555,` + otherBody + `}`, []string{"quaver_dir"}, false},
		{"both", func(d *Deps) { d.PortFlag, d.QuaverDirFlag = true, true }, `{"port":6000,` + otherBody + `}`, []string{"port", "quaver_dir"}, false},
		{"port flag, dir changes", func(d *Deps) { d.PortFlag = true }, `{"port":6000,` + otherBody + `}`, []string{"port"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t, c.opt)
			var put cfgResp
			w := e.put(c.body, nil)
			decode(t, w, &put)
			if w.Code != 200 || put.RestartRequired != c.rr || put.Overridden == nil || !slices.Equal(*put.Overridden, c.want) {
				t.Fatalf("PUT: code %d %s", w.Code, w.Body.String())
			}
			var get cfgResp
			decode(t, e.get("/api/config"), &get)
			if get.RestartRequired != c.rr || get.Overridden == nil || !slices.Equal(*get.Overridden, c.want) {
				t.Fatalf("GET: %+v", get)
			}
		})
	}
}

func TestRestartRequiredEmptyDirUsesDetected(t *testing.T) {
	other := testutil.NewQuaverDir(t).Path
	e := newEnv(t, func(d *Deps) { d.Detect = func() string { return other } })
	var got cfgResp
	decode(t, e.put(`{"port":5555}`, nil), &got)
	if !got.RestartRequired {
		t.Fatal("detected folder differs from the one in use: restart required")
	}
}

func TestPutAppliesLoadDefaults(t *testing.T) {
	e := newEnv(t)
	var got cfgResp
	w := e.put(`{"port":5555}`, nil)
	decode(t, w, &got)
	if w.Code != 200 || got.Config.Overlay.Theme != "neutral-light" {
		t.Fatalf("code %d %s", w.Code, w.Body.String())
	}
	raw, _ := os.ReadFile(e.cfgPath)
	if !strings.Contains(string(raw), `"neutral-light"`) {
		t.Fatalf("file = %s", raw)
	}
	w = e.put(`{"overlay":{"theme":"dark"}}`, nil)
	decode(t, w, &got)
	if w.Code != 200 || got.Config.Port != config.DefaultPort {
		t.Fatalf("missing port: code %d %s", w.Code, w.Body.String())
	}
}

func TestConfigGetBrokenFileLeaksNothing(t *testing.T) {
	e := newEnv(t)
	os.MkdirAll(filepath.Dir(e.cfgPath), 0o755)
	os.WriteFile(e.cfgPath, []byte("{nope"), 0o644)
	if body := e.get("/api/config").Body.String(); strings.Contains(body, "config.json") || strings.Contains(body, "invalid character") {
		t.Fatalf("leaks details: %s", body)
	}
}

func TestPutInvalidIsRejectedAndWritesNothing(t *testing.T) {
	big := `{"port":5555,"language":"` + strings.Repeat("a", maxBody) + `"}`
	cases := []struct {
		name, body string
		hdr        map[string]string
		code       int
		field      string
	}{
		{"port 70000", `{"port":70000}`, nil, 400, "port"},
		{"port negative", `{"port":-1}`, nil, 400, "port"},
		{"quaver_dir missing", `{"port":5555,"quaver_dir":"Z:\\no\\such\\dir"}`, nil, 400, "quaver_dir"},
		{"theme traversal", `{"port":5555,"overlay":{"theme":"../x"}}`, nil, 400, "overlay.theme"},
		{"accent", `{"port":5555,"overlay":{"theme":"dark","accent":"zzz"}}`, nil, 400, "overlay.accent"},
		{"language", `{"port":5555,"language":"xx"}`, nil, 400, "language"},
		{"broken json", `{"port":`, nil, 400, ""},
		{"trailing data", `{"port":5555} {"port":1}`, nil, 400, ""},
		{"not an object", `[1]`, nil, 400, ""},
		{"wrong type", `{"port":"5555"}`, nil, 400, ""},
		{"empty body", ``, nil, 400, ""},
		{"wrong content type", `{"port":5555}`, map[string]string{"Content-Type": "text/plain"}, 415, ""},
		{"no requested-with", `{"port":5555}`, map[string]string{"X-Requested-With": ""}, 403, ""},
		{"foreign origin", `{"port":5555}`, map[string]string{"Origin": "http://evil.example"}, 403, ""},
		{"body over 64 KB", big, nil, 413, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t)
			w := e.put(c.body, c.hdr)
			if w.Code != c.code {
				t.Fatalf("code %d, want %d: %s", w.Code, c.code, w.Body.String())
			}
			var er errBody
			decode(t, w, &er)
			if er.Error == "" || er.Field != c.field {
				t.Fatalf("body = %+v, want field %q", er, c.field)
			}
			e.noFile(t)
		})
	}
}

func TestPutKeepsPreviousFileOnInvalid(t *testing.T) {
	e := newEnv(t)
	if w := e.put(`{"port":5555,"language":"de","overlay":{"theme":"dark"}}`, nil); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	before, _ := os.ReadFile(e.cfgPath)
	if w := e.put(`{"port":70000}`, nil); w.Code != 400 {
		t.Fatal(w.Code)
	}
	after, _ := os.ReadFile(e.cfgPath)
	if string(before) != string(after) {
		t.Fatal("file changed by a rejected PUT")
	}
}

func TestHistory(t *testing.T) {
	var gotLimit int
	e := newEnv(t, func(d *Deps) {
		d.History = func(ctx context.Context, limit int) ([]scores.Result, error) {
			gotLimit = limit
			return []scores.Result{{
				Score:     scores.Score{ID: 7, Grade: "S", Accuracy: 97.5, MaxCombo: 100, PlayedAt: "2026-01-01T10:00:00+01:00", PlayedAtRaw: "01/01/2026 10:00", Rate: 1.2},
				Map:       scores.Map{Title: "T", Artist: "A", DifficultyName: "Hard", Difficulty: 12.3, Keys: 4},
				IsPB:      true,
				FullCombo: true,
				LocalRank: 2, LocalTotal: 9, IsLocalBest: true,
			}}, nil
		}
	})
	w := e.get("/api/history?limit=3")
	var got []map[string]any
	decode(t, w, &got)
	if w.Code != 200 || gotLimit != 3 || len(got) != 1 {
		t.Fatalf("code %d limit %d body %s", w.Code, gotLimit, w.Body.String())
	}
	row := got[0]
	for _, k := range []string{"id", "played_at", "played_at_raw", "title", "artist", "difficulty_name", "difficulty", "keys", "rate", "mods", "grade", "accuracy", "max_combo", "judgments", "failed", "paused", "is_pb", "full_combo"} {
		if _, ok := row[k]; !ok {
			t.Errorf("missing key %q", k)
		}
	}
	if row["local_rank"] != 2.0 || row["local_total"] != 9.0 || row["is_local_best"] != true {
		t.Errorf("local ranking fields = %v %v %v", row["local_rank"], row["local_total"], row["is_local_best"])
	}
	if row["played_at"] != "2026-01-01T10:00:00+01:00" || row["played_at_raw"] != "01/01/2026 10:00" {
		t.Errorf("times = %v %v", row["played_at"], row["played_at_raw"])
	}
	if row["mods"] == nil || row["title"] != "T" || row["is_pb"] != true {
		t.Errorf("row = %v", row)
	}

	e.get("/api/history")
	if gotLimit != 50 {
		t.Errorf("default limit = %d, want 50", gotLimit)
	}
}

func TestHistoryReportsAutoplay(t *testing.T) {
	e := newEnv(t, func(d *Deps) {
		d.History = func(ctx context.Context, limit int) ([]scores.Result, error) {
			return []scores.Result{{Autoplay: true}, {}}, nil
		}
	})
	var got []map[string]any
	decode(t, e.get("/api/history"), &got)
	if len(got) != 2 || got[0]["autoplay"] != true || got[1]["autoplay"] != false {
		t.Fatalf("autoplay flags = %v", got)
	}
}

func TestHistoryEmptyIsArray(t *testing.T) {
	e := newEnv(t)
	w := e.get("/api/history")
	if w.Code != 200 || strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("code %d body %q", w.Code, w.Body.String())
	}
}

func TestHistoryBadLimit(t *testing.T) {
	e := newEnv(t)
	for _, q := range []string{"0", "201", "-1", "abc"} {
		w := e.get("/api/history?limit=" + q)
		var er errBody
		decode(t, w, &er)
		if w.Code != 400 || er.Field != "limit" {
			t.Errorf("limit=%s: code %d body %+v", q, w.Code, er)
		}
	}
}

func TestHistoryDatabaseError(t *testing.T) {
	e := newEnv(t, func(d *Deps) {
		d.History = func(ctx context.Context, limit int) ([]scores.Result, error) {
			return nil, errors.New("disk on fire: C:\\secret\\quaver.db")
		}
	})
	w := e.get("/api/history")
	var er errBody
	decode(t, w, &er)
	if w.Code != 500 || er.Error == "" || strings.Contains(w.Body.String(), "secret") {
		t.Fatalf("code %d body %s", w.Code, w.Body.String())
	}
}

func TestHistoryNilFunc(t *testing.T) {
	e := newEnv(t, func(d *Deps) { d.History = nil })
	w := e.get("/api/history")
	if w.Code != 500 {
		t.Fatalf("code %d", w.Code)
	}
}

func TestMethodsAndUnknownPaths(t *testing.T) {
	e := newEnv(t)
	hdr := map[string]string{"Content-Type": "application/json", "X-Requested-With": "qsc"}
	for _, c := range []struct {
		method, path string
		want         int
	}{
		{"POST", "/api/status", 405}, {"PUT", "/api/history", 405}, {"DELETE", "/api/config", 405},
		{"POST", "/api/themes", 405}, {"GET", "/api/config/x", 404}, {"GET", "/api/nope", 404},
	} {
		w := do(e.h, c.method, "127.0.0.1:5555", c.path, "{}", hdr)
		var er errBody
		decode(t, w, &er)
		if w.Code != c.want || er.Error == "" {
			t.Errorf("%s %s: code %d body %s", c.method, c.path, w.Code, w.Body.String())
		}
	}
}

func TestRestartRequiredIgnoresSpellingOfSameFolder(t *testing.T) {
	e := newEnv(t)
	spellings := []string{filepath.ToSlash(e.quaver), e.quaver + "/", filepath.Join(e.quaver, "..", filepath.Base(e.quaver))}
	if runtime.GOOS == "windows" {
		spellings = append(spellings, strings.ToUpper(e.quaver), strings.ToLower(e.quaver))
	}
	for _, dir := range spellings {
		body, _ := json.Marshal(map[string]any{"quaver_dir": dir, "port": 5555})
		var got cfgResp
		decode(t, e.put(string(body), nil), &got)
		if got.RestartRequired {
			t.Errorf("%q is the folder already in use: no restart needed", dir)
		}
	}
}

var testThemes = []themes.Theme{
	{ID: "neutral-light", Source: themes.SourceBuiltin, Colors: themes.Colors{BG: "#fff", FG: "#111111", Accent: "#a855f7"}},
	{ID: "dark", Source: themes.SourceBuiltin},
	{ID: "mine", Source: themes.SourceCustom, Colors: themes.Colors{BG: "#000", FG: "#fff", Accent: "#f00"}},
}

func TestThemes(t *testing.T) {
	e := newEnv(t)
	var got []themes.Theme
	w := e.get("/api/themes")
	decode(t, w, &got)
	if w.Code != 200 || !reflect.DeepEqual(got, testThemes) {
		t.Fatalf("code %d got %+v", w.Code, got)
	}
	if body := w.Body.String(); !strings.Contains(body, `"colors":{"bg":"","fg":"","accent":""}`) || !strings.Contains(body, `"source":"custom"`) {
		t.Fatalf("body %s", body)
	}
}

func TestThemesEmptyIsAnArray(t *testing.T) {
	e := newEnv(t, func(d *Deps) { d.Themes = nil })
	if w := e.get("/api/themes"); strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("body %q", w.Body.String())
	}
}

func TestPutAcceptsEveryBuiltinThemeAndRejectsTraversal(t *testing.T) {
	built := themes.Builtin(web.FS)
	if len(built) != 11 {
		t.Fatalf("built-in themes = %d", len(built))
	}
	e := newEnv(t, func(d *Deps) { d.Themes = func() []themes.Theme { return built } })
	for _, th := range built {
		if w := e.put(`{"port":5555,"overlay":{"theme":"`+th.ID+`"}}`, nil); w.Code != 200 {
			t.Errorf("%s: code %d %s", th.ID, w.Code, w.Body)
		}
	}
	w := e.put(`{"port":5555,"overlay":{"theme":"../x"}}`, nil)
	if w.Code != 400 || !strings.Contains(w.Body.String(), "overlay.theme") {
		t.Fatalf("code %d %s", w.Code, w.Body)
	}
}

func TestCustomThemesAreListedLiveAndAcceptedByValidation(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "themes")
	live := func() []themes.Theme { return themes.Merge(testThemes, dir) }
	e := newEnv(t, func(d *Deps) { d.Themes = live; d.ThemesDir = dir })
	if w := e.put(`{"port":5555,"overlay":{"theme":"fresh"}}`, nil); w.Code != 400 {
		t.Fatalf("unknown theme: %d %s", w.Code, w.Body)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "fresh.css"), []byte(":root{--bg:#123456}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if w := e.get("/api/themes"); !strings.Contains(w.Body.String(), `"id":"fresh","source":"custom","colors":{"bg":"#123456"`) {
		t.Fatalf("themes %s", w.Body)
	}
	if w := e.put(`{"port":5555,"overlay":{"theme":"fresh"}}`, nil); w.Code != 200 {
		t.Fatalf("custom theme: %d %s", w.Code, w.Body)
	}
	if w := e.put(`{"port":5555,"overlay":{"theme":"../x"}}`, nil); w.Code != 400 {
		t.Fatalf("traversal: %d %s", w.Code, w.Body)
	}
}

func TestGetConfigReturnsThemesDir(t *testing.T) {
	e := newEnv(t, func(d *Deps) { d.ThemesDir = "/some/dir/themes" })
	var got struct {
		ThemesDir string `json:"themes_dir"`
	}
	if err := json.Unmarshal(e.get("/api/config").Body.Bytes(), &got); err != nil || got.ThemesDir != "/some/dir/themes" {
		t.Fatalf("got %+v %v", got, err)
	}
}

func TestPutKeepsMissingSavedThemeButRejectsChangedUnknownTheme(t *testing.T) {
	e := newEnv(t)
	if err := os.MkdirAll(filepath.Dir(e.cfgPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(e.cfgPath, []byte(`{"port":5555,"overlay":{"theme":"gone"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if w := e.put(`{"port":5555,"language":"fr","overlay":{"theme":"gone"}}`, nil); w.Code != 200 {
		t.Fatalf("unchanged missing theme: %d %s", w.Code, w.Body)
	}
	for _, id := range []string{"other-gone", "../x"} {
		w := e.put(`{"port":5555,"overlay":{"theme":"`+id+`"}}`, nil)
		if w.Code != 400 || !strings.Contains(w.Body.String(), "overlay.theme") {
			t.Fatalf("%s: %d %s", id, w.Code, w.Body)
		}
	}
}

func TestOverlaysListing(t *testing.T) {
	dir := t.TempDir()
	e := newEnv(t, func(d *Deps) {
		d.OverlaysDir = dir
		d.IsBuiltinOverlay = func(id string) bool { return id == "now-playing" }
	})
	type listing struct {
		Dir    string `json:"dir"`
		Custom []struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Description string `json:"description"`
			Width       int    `json:"width"`
			Height      int    `json:"height"`
		} `json:"custom"`
	}
	var got listing
	w := e.get("/api/overlays")
	decode(t, w, &got)
	if w.Code != 200 || got.Dir != dir || got.Custom == nil || len(got.Custom) != 0 || !strings.Contains(w.Body.String(), `"custom":[]`) {
		t.Fatalf("empty folder: %d %s", w.Code, w.Body.String())
	}
	for id, manifest := range map[string]string{"zeta": "", "alpha": `{"name":"Alpha","description":"d","width":640,"height":100}`, "now-playing": ""} {
		if err := os.MkdirAll(filepath.Join(dir, id), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, id, "index.html"), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		if manifest != "" {
			if err := os.WriteFile(filepath.Join(dir, id, "overlay.json"), []byte(manifest), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	got = listing{}
	decode(t, e.get("/api/overlays"), &got)
	if len(got.Custom) != 2 || got.Custom[0].ID != "alpha" || got.Custom[0].Name != "Alpha" || got.Custom[0].Width != 640 ||
		got.Custom[1].ID != "zeta" || got.Custom[1].Name != "zeta" || got.Custom[1].Width != 500 || got.Custom[1].Height != 200 {
		t.Fatalf("listing = %+v", got.Custom)
	}
	if w := do(e.h, "POST", "127.0.0.1:5555", "/api/overlays", "{}", map[string]string{"Content-Type": "application/json", "X-Requested-With": "qsc"}); w.Code != 405 {
		t.Errorf("POST: %d", w.Code)
	}
}

func TestOverlaysListingMissingFolderAndNoDir(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "absent")
	e := newEnv(t, func(d *Deps) { d.OverlaysDir = missing })
	var got struct {
		Dir    string `json:"dir"`
		Custom []any  `json:"custom"`
	}
	decode(t, e.get("/api/overlays"), &got)
	if got.Dir != missing || got.Custom == nil || len(got.Custom) != 0 {
		t.Errorf("missing folder: %+v", got)
	}
	e = newEnv(t)
	w := e.get("/api/overlays")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"custom":[]`) {
		t.Errorf("no deps: %d %s", w.Code, w.Body.String())
	}
}

func TestDegradedAppliesQuaverDirLive(t *testing.T) {
	other := testutil.NewQuaverDir(t).Path
	degraded := true
	e := newEnv(t, func(d *Deps) {
		d.Effective = func() string { return "" }
		d.Degraded = func() bool { return degraded }
	})
	var got cfgResp
	decode(t, e.put(`{"port":5555,"quaver_dir":"`+jsonPath(other)+`"}`, nil), &got)
	if got.RestartRequired {
		t.Fatal("a folder saved while degraded is picked up live")
	}
	decode(t, e.put(`{"port":6000,"quaver_dir":"`+jsonPath(other)+`"}`, nil), &got)
	if !got.RestartRequired {
		t.Fatal("a port change still needs a restart")
	}
	degraded = false
	decode(t, e.put(`{"port":5555,"quaver_dir":"`+jsonPath(other)+`"}`, nil), &got)
	if !got.RestartRequired {
		t.Fatal("once running, the folder change needs a restart")
	}
}

func TestHistoryAndTimelineAnswer503WhileUnavailable(t *testing.T) {
	e := newEnv(t, func(d *Deps) {
		d.History = func(context.Context, int) ([]scores.Result, error) { return nil, ErrUnavailable }
		d.Timeline = func(context.Context, int64) (scores.Timeline, error) { return scores.Timeline{}, ErrUnavailable }
	})
	for _, p := range []string{"/api/history", "/api/scores/1/timeline"} {
		w := e.get(p)
		if w.Code != 503 || !strings.Contains(w.Body.String(), "not available yet") {
			t.Errorf("%s = %d %s", p, w.Code, w.Body.String())
		}
	}
}

func TestHistoryAndTimelineAnswer503WhenQuaverIsWriting(t *testing.T) {
	busy := fmt.Errorf("list recent scores: %w", scores.ErrBusy)
	e := newEnv(t, func(d *Deps) {
		d.History = func(context.Context, int) ([]scores.Result, error) { return nil, busy }
		d.Timeline = func(context.Context, int64) (scores.Timeline, error) { return scores.Timeline{}, busy }
	})
	for _, p := range []string{"/api/history", "/api/scores/1/timeline"} {
		w := e.get(p)
		if w.Code != 503 || !strings.Contains(w.Body.String(), "Quaver is writing the database, try again") {
			t.Errorf("%s = %d %s", p, w.Code, w.Body.String())
		}
	}
}

func TestPutConfigRejectsABodyWithoutSettings(t *testing.T) {
	e := newEnv(t)
	before, _ := os.ReadFile(e.cfgPath)
	for _, body := range []string{`{}`, `{"config":{"port":9000},"language":"de"}`, `{"config":{"port":9000}}`, `{"foo":1}`, `[]`, `null`} {
		w := e.put(body, nil)
		var er errBody
		decode(t, w, &er)
		if w.Code != 400 || er.Error == "" {
			t.Errorf("%s: code %d body %s", body, w.Code, w.Body.String())
		}
	}
	after, _ := os.ReadFile(e.cfgPath)
	if string(before) != string(after) {
		t.Fatal("a rejected body must not touch the config file")
	}
	if w := e.put(`{"language":"de"}`, nil); w.Code != 200 {
		t.Fatalf("a document with a known key is saved as a whole, got %d %s", w.Code, w.Body.String())
	}
}

func TestPutProcessSettingIsAcceptedAndOmittedMeansOn(t *testing.T) {
	e := newEnv(t)
	var got cfgResp
	w := e.put(`{"port":5555,"watch_quaver_process":false}`, nil)
	decode(t, w, &got)
	if w.Code != 200 || got.Config.WatchQuaverProcess || got.RestartRequired {
		t.Fatalf("code %d %s", w.Code, w.Body.String())
	}
	if saved, err := config.Load(e.cfgPath); err != nil || saved.WatchQuaverProcess {
		t.Fatalf("saved = %+v, %v", saved, err)
	}
	w = e.put(`{"port":5555}`, nil)
	decode(t, w, &got)
	if w.Code != 200 || !got.Config.WatchQuaverProcess {
		t.Fatalf("omitted key must mean on: code %d %s", w.Code, w.Body.String())
	}
	if w = e.put(`{"watch_quaver_process":"no"}`, nil); w.Code != 400 {
		t.Fatalf("a non boolean must be rejected, code %d", w.Code)
	}
}
