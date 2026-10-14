package server

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/api"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/bus"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/keys"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/scores"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/state"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/themes"
)

func newServer(t *testing.T) (*httptest.Server, *bus.Bus) {
	t.Helper()
	b := bus.New()
	snap := func() state.Snapshot {
		return state.Snapshot{Game: "menu", Map: &scores.Map{Title: "Song"}}
	}
	assets := fstest.MapFS{
		"overlays/x/index.html": {Data: []byte("<p>hello overlay</p>")},
		"themes/t.css":          {Data: []byte(":root{}")},
	}
	srv := httptest.NewServer(New(b, snap, assets))
	t.Cleanup(srv.Close)
	return srv, b
}

func TestStateEndpoint(t *testing.T) {
	srv, _ := newServer(t)
	resp, err := http.Get(srv.URL + "/state")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 || resp.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("status=%d cors=%q", resp.StatusCode, resp.Header.Get("Access-Control-Allow-Origin"))
	}
	var got state.Snapshot
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Game != "menu" || got.Map == nil || got.Map.Title != "Song" {
		t.Fatalf("snapshot = %+v", got)
	}
}

func TestStaticAssets(t *testing.T) {
	srv, _ := newServer(t)
	for path, want := range map[string]string{
		"/overlays/x/":  "hello overlay",
		"/themes/t.css": ":root{}",
	} {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 || !strings.Contains(string(body), want) {
			t.Fatalf("%s: status=%d body=%q", path, resp.StatusCode, body)
		}
	}
}

func TestStateRejectsNonGet(t *testing.T) {
	srv, _ := newServer(t)
	resp, err := http.Post(srv.URL+"/state", "text/plain", strings.NewReader("x"))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

// readEvent returns the name and data of the next SSE event.
func readEvent(t *testing.T, sc *bufio.Scanner) (string, string) {
	t.Helper()
	type result struct{ name, data string }
	done := make(chan result, 1)
	go func() {
		var r result
		for sc.Scan() {
			line := sc.Text()
			switch {
			case strings.HasPrefix(line, "event: "):
				r.name = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				r.data = strings.TrimPrefix(line, "data: ")
			case line == "" && r.name != "":
				done <- r
				return
			}
		}
	}()
	select {
	case r := <-done:
		return r.name, r.data
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for SSE event")
		return "", ""
	}
}

func TestEventsStreamSnapshotThenLiveEvents(t *testing.T) {
	srv, b := newServer(t)
	resp, err := http.Get(srv.URL + "/events")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("content-type = %q", ct)
	}
	sc := bufio.NewScanner(resp.Body)

	name, data := readEvent(t, sc)
	if name != "snapshot" {
		t.Fatalf("first event = %q", name)
	}
	var snap struct {
		V    int
		Type string
		Data state.Snapshot
	}
	if err := json.Unmarshal([]byte(data), &snap); err != nil {
		t.Fatal(err)
	}
	if snap.V != 1 || snap.Type != "snapshot" || snap.Data.Game != "menu" {
		t.Fatalf("snapshot envelope = %+v", snap)
	}

	b.Publish(bus.TypeMapStarted, scores.Map{Title: "Live"})
	name, data = readEvent(t, sc)
	if name != "map.started" || !strings.Contains(data, `"title":"Live"`) {
		t.Fatalf("event = %q %s", name, data)
	}
}

func newGuarded(t *testing.T) *httptest.Server {
	t.Helper()
	assets := fstest.MapFS{
		"overlays/x/index.html": {Data: []byte("<p>hello overlay</p>")},
		"themes/t.css":          {Data: []byte(":root{}")},
		"themes/fonts/f.ttf":    {Data: []byte("font")},
	}
	dash := fstest.MapFS{"index.html": {Data: []byte("<title>dash</title>")}}
	api := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "api:"+r.URL.Path) })
	srv := httptest.NewUnstartedServer(nil)
	port := srv.Listener.Addr().(*net.TCPAddr).Port
	snap := func() state.Snapshot { return state.Snapshot{Game: "menu"} }
	srv.Config.Handler = New(bus.New(), snap, assets, WithAPI(api, port), WithDashboard(dash))
	srv.Start()
	t.Cleanup(srv.Close)
	return srv
}

func getHost(t *testing.T, url, host string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest("GET", url, nil)
	req.Host = host
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}

func TestDashboardAndAPIMounted(t *testing.T) {
	srv := newGuarded(t)
	for path, want := range map[string]string{
		"/":             "<title>dash</title>",
		"/api/status":   "api:/api/status",
		"/state":        `"game":"menu"`,
		"/overlays/x/":  "hello overlay",
		"/themes/t.css": ":root{}",
	} {
		code, body := getHost(t, srv.URL+path, "")
		if code != 200 || !strings.Contains(body, want) {
			t.Errorf("%s: status=%d body=%q", path, code, body)
		}
	}
}

func TestGuardCoversWholeMux(t *testing.T) {
	srv := newGuarded(t)
	for _, path := range []string{"/", "/state", "/events", "/overlays/x/", "/themes/t.css", "/api/status", "/api/config"} {
		code, body := getHost(t, srv.URL+path, "evil.example")
		if code != http.StatusForbidden || strings.Contains(body, "hello overlay") || strings.Contains(body, "dash") {
			t.Errorf("%s: status=%d body=%q", path, code, body)
		}
	}
	_, port, _ := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	if code, _ := getHost(t, srv.URL+"/state", "localhost:"+port); code != 200 {
		t.Errorf("localhost host: %d", code)
	}
}

func TestWithoutOptionsNothingExtraIsMounted(t *testing.T) {
	srv, _ := newServer(t)
	for _, path := range []string{"/", "/api/status"} {
		if code, _ := getHost(t, srv.URL+path, ""); code != 404 {
			t.Errorf("%s: %d, want 404", path, code)
		}
	}
}

func TestAPIChunkedBodyOverLimitIs413AndWritesNothing(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.json")
	srv := httptest.NewUnstartedServer(nil)
	port := srv.Listener.Addr().(*net.TCPAddr).Port
	h := api.New(api.Deps{ConfigPath: cfgPath, Themes: func() []themes.Theme { return []themes.Theme{{ID: "dark", Source: themes.SourceBuiltin}} }, Port: port})
	srv.Config.Handler = New(bus.New(), func() state.Snapshot { return state.Snapshot{} },
		fstest.MapFS{}, WithAPI(h, port))
	srv.Start()
	t.Cleanup(srv.Close)

	body := io.MultiReader(strings.NewReader(`{"port":5555,"language":"`), strings.NewReader(strings.Repeat("a", 70<<10)), strings.NewReader(`"}`))
	req, _ := http.NewRequest("PUT", srv.URL+"/api/config", io.NopCloser(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Requested-With", "qsc")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if req.ContentLength > 0 || resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("content length %d, status %d, want chunked and 413", req.ContentLength, resp.StatusCode)
	}
	if _, err := os.Stat(cfgPath); !os.IsNotExist(err) {
		t.Fatalf("config file must not exist, stat err = %v", err)
	}
}

func TestDashboardHasContentSecurityPolicyOnly(t *testing.T) {
	srv := newGuarded(t)
	for path, want := range map[string]bool{
		"/": true, "/index.html": true, "/missing": true,
		"/themes/t.css": false, "/state": false, "/api/status": false,
	} {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		got := resp.Header.Get("Content-Security-Policy")
		if want && got != dashboardCSP {
			t.Errorf("%s: CSP = %q", path, got)
		}
		if !want && got != "" {
			t.Errorf("%s: unexpected CSP %q", path, got)
		}
	}
}

func TestDashboardCSPValue(t *testing.T) {
	for _, d := range []string{"default-src 'self'", "script-src 'self'", "style-src 'self'", "object-src 'none'", "base-uri 'none'", "frame-ancestors 'none'"} {
		if !strings.Contains(dashboardCSP, d) {
			t.Errorf("missing %q", d)
		}
	}
}

func TestOnlyThemesAssetsAllowCrossOrigin(t *testing.T) {
	srv := newGuarded(t)
	for path, want := range map[string]bool{
		"/themes/t.css": true, "/themes/fonts/f.ttf": true,
		"/api/themes": false, "/": false, "/overlays/x/": false,
	} {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		got := resp.Header.Get("Access-Control-Allow-Origin")
		if want && got != "*" || !want && got != "" {
			t.Errorf("%s: Access-Control-Allow-Origin = %q", path, got)
		}
	}
}

func newThemesServer(t *testing.T, dir string) *httptest.Server {
	t.Helper()
	assets := fstest.MapFS{
		"themes/t.css":        {Data: []byte(":root{--bg:#000}")},
		"themes/fonts/f.ttf":  {Data: []byte("font")},
		"overlays/x/index.js": {Data: []byte("x")},
	}
	dash := fstest.MapFS{"index.html": {Data: []byte("<html></html>")}}
	srv := httptest.NewUnstartedServer(nil)
	port := srv.Listener.Addr().(*net.TCPAddr).Port
	h := api.New(api.Deps{ConfigPath: filepath.Join(dir, "config.json"), Port: port})
	srv.Config.Handler = New(bus.New(), func() state.Snapshot { return state.Snapshot{} }, assets,
		WithAPI(h, port), WithDashboard(dash), WithThemesDir(filepath.Join(dir, "themes")))
	srv.Start()
	t.Cleanup(srv.Close)
	return srv
}

func TestCustomThemesAreServedWithoutRestartAndOverrideBuiltin(t *testing.T) {
	dir := t.TempDir()
	srv := newThemesServer(t, dir)
	if code, _ := getHost(t, srv.URL+"/themes/mine.css", ""); code != 404 {
		t.Fatalf("missing folder: %d", code)
	}
	if code, body := getHost(t, srv.URL+"/themes/t.css", ""); code != 200 || body != ":root{--bg:#000}" {
		t.Fatalf("built-in: %d %q", code, body)
	}
	themesDir := filepath.Join(dir, "themes")
	if err := os.Mkdir(themesDir, 0o700); err != nil {
		t.Fatal(err)
	}
	for name, css := range map[string]string{"mine.css": ":root{--bg:#abc}", "t.css": ":root{--bg:#fff}"} {
		if err := os.WriteFile(filepath.Join(themesDir, name), []byte(css), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if code, body := getHost(t, srv.URL+"/themes/mine.css", ""); code != 200 || body != ":root{--bg:#abc}" {
		t.Fatalf("new file: %d %q", code, body)
	}
	if code, body := getHost(t, srv.URL+"/themes/t.css", ""); code != 200 || body != ":root{--bg:#fff}" {
		t.Fatalf("override: %d %q", code, body)
	}
	if code, body := getHost(t, srv.URL+"/themes/fonts/f.ttf", ""); code != 200 || body != "font" {
		t.Fatalf("fonts: %d %q", code, body)
	}
	resp, err := http.Get(srv.URL + "/themes/mine.css")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.Header.Get("Access-Control-Allow-Origin") != "*" || resp.Header.Get("Content-Security-Policy") != "" {
		t.Errorf("headers: %v", resp.Header)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/css") {
		t.Errorf("content type %q", ct)
	}
	if cc := resp.Header.Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("custom theme Cache-Control = %q", cc)
	}
	for _, p := range []string{"/themes/mine.css", "/themes/t.css", "/themes/fonts/f.ttf", "/themes/missing.css"} {
		r, err := http.Get(srv.URL + p)
		if err != nil {
			t.Fatal(err)
		}
		r.Body.Close()
		if got := r.Header.Get("X-Content-Type-Options"); got != "nosniff" {
			t.Errorf("%s: X-Content-Type-Options = %q", p, got)
		}
	}
	if code, _ := getHost(t, srv.URL+"/themes/mine.css", "evil.example"); code != 403 {
		t.Errorf("host guard: %d", code)
	}
}

func TestCustomThemesRefuseTraversalAndUnsafeFiles(t *testing.T) {
	dir := t.TempDir()
	themesDir := filepath.Join(dir, "themes")
	if err := os.Mkdir(themesDir, 0o700); err != nil {
		t.Fatal(err)
	}
	secret := ":root{--bg:#bad}"
	files := map[string]string{
		filepath.Join(dir, "x.css"):         secret,
		filepath.Join(themesDir, "UP.css"):  secret,
		filepath.Join(themesDir, "a.b.css"): secret,
		filepath.Join(themesDir, "big.css"): strings.Repeat("a", 65<<10),
	}
	for p, c := range files {
		if err := os.WriteFile(p, []byte(c), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(themesDir, "evil.css"), 0o700); err != nil {
		t.Fatal(err)
	}
	srv := newThemesServer(t, dir)
	for _, p := range []string{
		"/themes/../x.css", "/themes/%2e%2e/x.css", "/themes/%2e%2e%2fx.css", "/themes/UP.css", "/themes/a.b.css",
		"/themes/x.css%00", "/themes/..%5cx.css", "/themes/%5c..%5cx.css", "/themes/big.css", "/themes/evil.css",
		"/themes//x.css", "/themes/sub/../../x.css",
	} {
		code, body := getHost(t, srv.URL+p, "")
		if code == 200 || strings.Contains(body, "#bad") || strings.HasPrefix(body, "aaaa") {
			t.Errorf("%s: %d leaked %.20q", p, code, body)
		}
	}
}

func TestCustomThemeSymlinkIsRefused(t *testing.T) {
	dir := t.TempDir()
	themesDir := filepath.Join(dir, "themes")
	if err := os.Mkdir(themesDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "x.css"), []byte(":root{--bg:#bad}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "x.css"), filepath.Join(themesDir, "link.css")); err != nil {
		t.Skipf("symlinks are not available here: %v", err)
	}
	srv := newThemesServer(t, dir)
	if code, body := getHost(t, srv.URL+"/themes/link.css", ""); code == 200 || strings.Contains(body, "#bad") {
		t.Errorf("symlink served: %d %q", code, body)
	}
}

// A hard link to a regular file is a regular file: serving it is intended.
func TestCustomThemeHardLinkIsServed(t *testing.T) {
	dir := t.TempDir()
	themesDir := filepath.Join(dir, "themes")
	if err := os.Mkdir(themesDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "x.css"), []byte(":root{--bg:#abc}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(filepath.Join(dir, "x.css"), filepath.Join(themesDir, "hard.css")); err != nil {
		t.Skipf("hard links are not available here: %v", err)
	}
	srv := newThemesServer(t, dir)
	if code, body := getHost(t, srv.URL+"/themes/hard.css", ""); code != 200 || body != ":root{--bg:#abc}" {
		t.Errorf("hard link: %d %q", code, body)
	}
}

func TestCustomThemeNeverServedUnderADifferentCaseName(t *testing.T) {
	dir := t.TempDir()
	themesDir := filepath.Join(dir, "themes")
	if err := os.Mkdir(themesDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(themesDir, "UP.css"), []byte(":root{--bg:#bad}"), 0o600); err != nil {
		t.Fatal(err)
	}
	srv := newThemesServer(t, dir)
	if code, body := getHost(t, srv.URL+"/themes/up.css", ""); code == 200 || strings.Contains(body, "#bad") {
		t.Errorf("up.css served the file UP.css: %d %q", code, body)
	}
}

func openEvents(t *testing.T, url string) *bufio.Scanner {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	sc := bufio.NewScanner(resp.Body)
	if name, _ := readEvent(t, sc); name != "snapshot" {
		t.Fatalf("first event = %q", name)
	}
	return sc
}

func TestKeysFramesOnlyReachClientsThatAskForThem(t *testing.T) {
	srv, b := newServer(t)
	plain := openEvents(t, srv.URL+"/events")
	optIn := openEvents(t, srv.URL+"/events?keys=1")
	other := openEvents(t, srv.URL+"/events?keys=0")

	for i := 0; i < 200; i++ {
		b.Publish(bus.TypeKeysFrame, map[string]int{"kps": i})
	}
	b.Publish(bus.TypeMapStarted, scores.Map{Title: "Live"})

	for _, sc := range []*bufio.Scanner{plain, other} {
		if name, data := readEvent(t, sc); name != "map.started" || !strings.Contains(data, `"title":"Live"`) {
			t.Fatalf("client without keys=1 got %q %s", name, data)
		}
	}
	if name, _ := readEvent(t, optIn); name != "keys.frame" {
		t.Fatalf("opt-in client first event = %q, want keys.frame", name)
	}
}

func TestOptInClientStillGetsOtherEvents(t *testing.T) {
	srv, b := newServer(t)
	sc := openEvents(t, srv.URL+"/events?keys=1")
	b.Publish(bus.TypeGameState, bus.GameState{State: "gameplay"})
	if name, _ := readEvent(t, sc); name != "game.state" {
		t.Fatalf("event = %q", name)
	}
}

func TestSnapshotKeepsKeysFieldForEveryClient(t *testing.T) {
	b := bus.New()
	snap := func() state.Snapshot { return state.Snapshot{Game: "menu"} }
	srv := httptest.NewServer(New(b, snap, fstest.MapFS{}))
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/events")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	_, data := readEvent(t, bufio.NewScanner(resp.Body))
	if !strings.Contains(data, `"keys":null`) {
		t.Fatalf("snapshot = %s", data)
	}
}

func getWithOrigin(t *testing.T, url, origin string) *http.Response {
	t.Helper()
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		t.Fatal(err)
	}
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func newKeysServer(t *testing.T) (*httptest.Server, *bus.Bus) {
	t.Helper()
	b := bus.New()
	snap := func() state.Snapshot {
		return state.Snapshot{Game: "gameplay", Keys: &keys.Frame{Layout: "4k", Lanes: 4, Down: []bool{true, false, false, false}, Presses: []int{1, 0, 0, 0}, KPS: 3, Peak: 4}}
	}
	srv := httptest.NewServer(New(b, snap, fstest.MapFS{}))
	t.Cleanup(srv.Close)
	return srv, b
}

func TestCrossOriginStateHasNoKeys(t *testing.T) {
	srv, _ := newKeysServer(t)
	host := strings.TrimPrefix(srv.URL, "http://")
	for _, c := range []struct {
		name, origin string
		wantKeys     bool
	}{
		{"no origin", "", true},
		{"same origin", "http://" + host, true},
		{"foreign origin", "http://evil.example", false},
		{"opaque origin", "null", false},
		{"other port", "http://127.0.0.1:1", false},
	} {
		raw, err := io.ReadAll(getWithOrigin(t, srv.URL+"/state", c.origin).Body)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]json.RawMessage
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if _, has := m["keys"]; has != c.wantKeys {
			t.Errorf("%s: keys present = %v, want %v (%s)", c.name, has, c.wantKeys, raw)
		}
		if string(m["game"]) != `"gameplay"` {
			t.Errorf("%s: other fields must stay: %s", c.name, raw)
		}
	}
}

func TestCrossOriginEventsKeysStreamIsForbidden(t *testing.T) {
	srv, _ := newKeysServer(t)
	host := strings.TrimPrefix(srv.URL, "http://")
	for _, origin := range []string{"http://evil.example", "null"} {
		resp := getWithOrigin(t, srv.URL+"/events?keys=1", origin)
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("origin %q: status = %d, want 403", origin, resp.StatusCode)
		}
	}
	for _, origin := range []string{"", "http://" + host} {
		resp := getWithOrigin(t, srv.URL+"/events?keys=1", origin)
		if resp.StatusCode != 200 {
			t.Fatalf("origin %q: status = %d", origin, resp.StatusCode)
		}
		if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
			t.Errorf("origin %q: keys stream sends Access-Control-Allow-Origin %q", origin, got)
		}
		_, data := readEvent(t, bufio.NewScanner(resp.Body))
		if !strings.Contains(data, `"kps":3`) {
			t.Errorf("origin %q: snapshot lost the keys: %s", origin, data)
		}
	}
}

func TestCrossOriginPlainEventsSnapshotHasNoKeys(t *testing.T) {
	srv, _ := newKeysServer(t)
	resp := getWithOrigin(t, srv.URL+"/events", "http://evil.example")
	if resp.StatusCode != 200 || resp.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("status=%d cors=%q", resp.StatusCode, resp.Header.Get("Access-Control-Allow-Origin"))
	}
	_, data := readEvent(t, bufio.NewScanner(resp.Body))
	if strings.Contains(data, `"keys"`) || !strings.Contains(data, `"game":"gameplay"`) {
		t.Fatalf("cross-origin snapshot = %s", data)
	}
}

func TestEventsCapsConcurrentSubscribers(t *testing.T) {
	srv, _ := newServer(t)
	for i := 0; i < maxEventClients; i++ {
		openEvents(t, srv.URL+"/events")
	}
	resp, err := http.Get(srv.URL + "/events")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("client %d: status = %d, want 503", maxEventClients+1, resp.StatusCode)
	}
}

type deadlineWriter struct {
	*httptest.ResponseRecorder
	mu        sync.Mutex
	deadlines []time.Time
}

func (d *deadlineWriter) SetWriteDeadline(t time.Time) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.deadlines = append(d.deadlines, t)
	return nil
}

func (d *deadlineWriter) count() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.deadlines)
}

func TestEventsRefreshesWriteDeadlineOnEveryWrite(t *testing.T) {
	b := bus.New()
	h := New(b, func() state.Snapshot { return state.Snapshot{Game: "menu"} }, fstest.MapFS{})
	ctx, cancel := context.WithCancel(context.Background())
	w := &deadlineWriter{ResponseRecorder: httptest.NewRecorder()}
	done := make(chan struct{})
	go func() {
		h.ServeHTTP(w, httptest.NewRequest("GET", "/events", nil).WithContext(ctx))
		close(done)
	}()
	waitFor(t, func() bool { return w.count() >= 1 })
	b.Publish(bus.TypeGameState, bus.GameState{State: "gameplay"})
	waitFor(t, func() bool { return w.count() >= 2 })
	cancel()
	<-done
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, d := range w.deadlines {
		if left := time.Until(d); left < 20*time.Second || left > 31*time.Second {
			t.Errorf("deadline is %v away, want about 30s", left)
		}
	}
}

func waitFor(t *testing.T, ok func() bool) {
	t.Helper()
	for i := 0; i < 200; i++ {
		if ok() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition not reached")
}

type stuckWriter struct {
	*httptest.ResponseRecorder
	release chan struct{}
	once    sync.Once
}

func (s *stuckWriter) Write(p []byte) (int, error) {
	s.once.Do(func() { <-s.release })
	return s.ResponseRecorder.Write(p)
}

func TestOverflowingSubscriberIsClosedSoItResyncs(t *testing.T) {
	b := bus.New()
	h := New(b, func() state.Snapshot { return state.Snapshot{Game: "menu"} }, fstest.MapFS{})
	w := &stuckWriter{ResponseRecorder: httptest.NewRecorder(), release: make(chan struct{})}
	done := make(chan struct{})
	go func() {
		h.ServeHTTP(w, httptest.NewRequest("GET", "/events", nil))
		close(done)
	}()
	waitFor(t, func() bool { return b.Subscribers() == 1 })
	for i := 0; i < 500; i++ {
		b.Publish(bus.TypeMapStarted, i)
	}
	close(w.release)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the stream stayed open after the queue overflowed")
	}
	if b.Dropped() == 0 {
		t.Error("the overflow was not counted")
	}
}
