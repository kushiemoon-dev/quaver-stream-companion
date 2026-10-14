package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/testutil"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/themes"
)

func TestAvailableThemesAreTheBuiltInOnes(t *testing.T) {
	if got := themes.IDs(availableThemes("")); !slices.Equal(got, []string{"neutral-light", "neutral-dark", "catppuccin-mocha", "catppuccin-latte", "tokyo-night", "dracula", "nord", "gruvbox", "rose-pine", "pixel", "neon-cosy"}) {
		t.Fatalf("themes = %v", got)
	}
}

func TestUnknownThemeInConfigIsRejected(t *testing.T) {
	cfg := writeConfig(t, `{"overlay": {"theme": "../x"}}`)
	var out, errw bytes.Buffer
	if code := run(context.Background(), []string{"--config", cfg}, &out, &errw); code != 1 {
		t.Fatalf("code = %d", code)
	}
	if !strings.Contains(errw.String(), "overlay.theme") {
		t.Fatalf("stderr = %q", errw.String())
	}
}

func TestMissingConfigFolderWarns(t *testing.T) {
	for _, k := range []string{"APPDATA", "XDG_CONFIG_HOME", "HOME", "USERPROFILE"} {
		t.Setenv(k, "")
	}
	var errw bytes.Buffer
	o, code := parseFlags("qsc", true, nil, &errw)
	if code != -1 || o.port != 7878 {
		t.Fatalf("o = %+v code = %d", o, code)
	}
	if got := errw.String(); !strings.HasPrefix(got, "warning:") || strings.Count(got, "\n") != 1 {
		t.Fatalf("stderr = %q", got)
	}
}

func TestOpenFlagOnlyForServe(t *testing.T) {
	var out, errw bytes.Buffer
	if code := run(context.Background(), []string{"doctor", "--open"}, &out, &errw); code != 2 {
		t.Fatalf("doctor --open: code = %d", code)
	}
}

type liveServer struct {
	port    int
	cfgPath string
	quaver  string
	stop    func()
}

func startServer(t *testing.T, extra ...string) liveServer {
	t.Helper()
	d := testutil.NewQuaverDir(t)
	port := freePort(t)
	cfgPath := filepath.Join(t.TempDir(), "config.json")
	args := append([]string{"--quaver-dir", d.Path, "--port", fmt.Sprint(port), "--config", cfgPath}, extra...)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	var out, errw bytes.Buffer
	go func() { done <- run(ctx, args, &out, &errw) }()
	var once sync.Once
	stop := func() {
		once.Do(func() {
			cancel()
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				t.Error("server did not stop")
			}
		})
	}
	t.Cleanup(stop)
	url := fmt.Sprintf("http://127.0.0.1:%d/state", port)
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); time.Sleep(30 * time.Millisecond) {
		if resp, err := http.Get(url); err == nil {
			resp.Body.Close()
			return liveServer{port, cfgPath, d.Path, stop}
		}
	}
	t.Fatalf("server not up (stderr: %s)", errw.String())
	return liveServer{}
}

func (s liveServer) req(t *testing.T, method, path, host, body string, hdr map[string]string) (int, string) {
	t.Helper()
	r, _ := http.NewRequest(method, fmt.Sprintf("http://127.0.0.1:%d%s", s.port, path), strings.NewReader(body))
	if host != "" {
		r.Host = host
	}
	for k, v := range hdr {
		r.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func TestServeMountsDashboardAndAPI(t *testing.T) {
	s := startServer(t)
	if code, body := s.req(t, "GET", "/", "", "", nil); code != 200 || !strings.Contains(body, "QuaverStreamCompanion") {
		t.Fatalf("/: %d %q", code, body)
	}
	code, body := s.req(t, "GET", "/api/status", "", "", nil)
	var st struct {
		Checks []struct {
			ID string `json:"id"`
			OK bool   `json:"ok"`
		} `json:"checks"`
	}
	if code != 200 || json.Unmarshal([]byte(body), &st) != nil || len(st.Checks) != 4 {
		t.Fatalf("/api/status: %d %q", code, body)
	}
	for _, c := range st.Checks {
		if !c.OK {
			t.Errorf("check %s not ok (the port check must see our own server)", c.ID)
		}
	}
	if code, body := s.req(t, "GET", "/api/history", "", "", nil); code != 200 || strings.TrimSpace(body) != "[]" {
		t.Fatalf("/api/history: %d %q", code, body)
	}
	code, body = s.req(t, "GET", "/api/config", "", "", nil)
	var cfg struct {
		Effective struct {
			QuaverDir string `json:"quaver_dir"`
		} `json:"effective"`
	}
	if code != 200 || json.Unmarshal([]byte(body), &cfg) != nil || cfg.Effective.QuaverDir != s.quaver {
		t.Fatalf("/api/config: %d %q", code, body)
	}
	if code, body := s.req(t, "GET", "/api/themes", "", "", nil); code != 200 || !strings.Contains(body, "neutral-light") {
		t.Fatalf("/api/themes: %d %q", code, body)
	}
}

func TestServeRejectsForeignHostEverywhere(t *testing.T) {
	s := startServer(t)
	for _, p := range []string{"/", "/api/status", "/state", "/events", "/overlays/now-playing/", "/themes/neutral-light.css"} {
		if code, _ := s.req(t, "GET", p, "evil.example", "", nil); code != http.StatusForbidden {
			t.Errorf("%s with foreign Host: %d", p, code)
		}
	}
}

func TestServePutConfigEndToEnd(t *testing.T) {
	s := startServer(t)
	hdr := map[string]string{"Content-Type": "application/json", "X-Requested-With": "qsc"}
	if code, body := s.req(t, "PUT", "/api/config", "", `{"port":70000}`, hdr); code != 400 {
		t.Fatalf("invalid: %d %q", code, body)
	}
	if _, err := os.Stat(s.cfgPath); !os.IsNotExist(err) {
		t.Fatalf("config written by a rejected PUT: %v", err)
	}
	body := fmt.Sprintf(`{"quaver_dir":"%s","port":%d,"language":"de","overlay":{"theme":"neutral-light"}}`, jsonPath(s.quaver), s.port)
	code, resp := s.req(t, "PUT", "/api/config", "", body, hdr)
	if code != 200 || !strings.Contains(resp, `"restart_required":false`) {
		t.Fatalf("valid: %d %q", code, resp)
	}
	if data, err := os.ReadFile(s.cfgPath); err != nil || !strings.Contains(string(data), `"language": "de"`) {
		t.Fatalf("file = %q err %v", data, err)
	}
	if code, _ := s.req(t, "PUT", "/api/config", "", body, map[string]string{"Content-Type": "application/json"}); code != 403 {
		t.Fatalf("missing X-Requested-With: %d", code)
	}
}

type recorder struct {
	mu   sync.Mutex
	urls []string
}

func (r *recorder) open(url string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.urls = append(r.urls, url)
	return nil
}

func (r *recorder) calls() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.urls)
}

func swapBrowser(t *testing.T, f func(string) error) {
	t.Helper()
	t.Cleanup(openBrowser.set(f))
}

func TestOpenFlagCallsBrowserOnce(t *testing.T) {
	rec := &recorder{}
	swapBrowser(t, rec.open)
	s := startServer(t, "--open")
	want := fmt.Sprintf("http://127.0.0.1:%d/", s.port)
	if got := rec.calls(); !slices.Equal(got, []string{want}) {
		t.Fatalf("calls = %v, want [%s]", got, want)
	}
}

func TestNoBrowserWithoutOpenFlag(t *testing.T) {
	rec := &recorder{}
	swapBrowser(t, rec.open)
	s := startServer(t)
	s.stop()
	if got := rec.calls(); len(got) != 0 {
		t.Fatalf("browser called: %v", got)
	}
}

func TestBrowserFailureIsReportedNotFatal(t *testing.T) {
	swapBrowser(t, func(string) error { return fmt.Errorf("no browser") })
	d := testutil.NewQuaverDir(t)
	port := freePort(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	var out, errw bytes.Buffer
	cfg := filepath.Join(t.TempDir(), "c.json")
	go func() {
		done <- run(ctx, []string{"--open", "--quaver-dir", d.Path, "--port", fmt.Sprint(port), "--config", cfg}, &out, &errw)
	}()
	url := fmt.Sprintf("http://127.0.0.1:%d/state", port)
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); time.Sleep(30 * time.Millisecond) {
		if resp, err := http.Get(url); err == nil {
			resp.Body.Close()
			break
		}
	}
	cancel()
	if code := <-done; code != 0 {
		t.Fatalf("code = %d", code)
	}
	if !strings.Contains(errw.String(), "no browser") {
		t.Fatalf("stderr = %q", errw.String())
	}
}

func TestServeLoadsCustomOverlaysFromTheConfigFolder(t *testing.T) {
	s := startServer(t)
	dir := filepath.Join(filepath.Dir(s.cfgPath), "overlays", "mine")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<p>mine</p>"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(filepath.Dir(s.cfgPath), "overlays", "now-playing"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(s.cfgPath), "overlays", "now-playing", "index.html"), []byte("hijack"), 0o600); err != nil {
		t.Fatal(err)
	}
	if code, body := s.req(t, "GET", "/overlays/mine/", "", "", nil); code != 200 || body != "<p>mine</p>" {
		t.Fatalf("custom overlay: %d %q", code, body)
	}
	if code, body := s.req(t, "GET", "/overlays/now-playing/", "", "", nil); code != 200 || strings.Contains(body, "hijack") {
		t.Fatalf("built-in overlay: %d %.40q", code, body)
	}
	code, body := s.req(t, "GET", "/api/overlays", "", "", nil)
	var got struct {
		Dir    string `json:"dir"`
		Custom []struct {
			ID string `json:"id"`
		} `json:"custom"`
	}
	if code != 200 || json.Unmarshal([]byte(body), &got) != nil || got.Dir != filepath.Join(filepath.Dir(s.cfgPath), "overlays") ||
		len(got.Custom) != 1 || got.Custom[0].ID != "mine" {
		t.Fatalf("/api/overlays: %d %q", code, body)
	}
	if code, _ := s.req(t, "GET", "/overlays/mine/", "evil.example", "", nil); code != http.StatusForbidden {
		t.Errorf("foreign Host: %d", code)
	}
}

func TestServeStartsWhenTheOverlaysFolderIsBroken(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	cfgDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(cfgDir, "overlays"), []byte("a file, not a folder"), 0o600); err != nil {
		t.Fatal(err)
	}
	port := freePort(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	var out, errw bytes.Buffer
	go func() {
		done <- run(ctx, []string{"--quaver-dir", d.Path, "--port", fmt.Sprint(port), "--config", filepath.Join(cfgDir, "config.json")}, &out, &errw)
	}()
	defer func() {
		cancel()
		<-done
	}()
	url := fmt.Sprintf("http://127.0.0.1:%d/api/overlays", port)
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); time.Sleep(30 * time.Millisecond) {
		resp, err := http.Get(url)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 || !strings.Contains(string(body), `"custom":[]`) {
			t.Fatalf("%d %s", resp.StatusCode, body)
		}
		return
	}
	t.Fatalf("server not up (stderr: %s)", errw.String())
}
