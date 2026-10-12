package server

import (
	"bufio"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/api"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/bus"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/scores"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/state"
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
	h := api.New(api.Deps{ConfigPath: cfgPath, Themes: []string{"dark"}, Port: port})
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
		"/overlays/x/": false, "/themes/t.css": false, "/state": false, "/api/status": false,
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
