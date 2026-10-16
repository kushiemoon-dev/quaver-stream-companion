package server

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/api"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/bus"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/state"
	"github.com/kushiemoon-dev/quaver-stream-companion/web"
)

func newOverlaysServer(t *testing.T, dir string) *httptest.Server {
	t.Helper()
	assets := fstest.MapFS{
		"overlays/x/index.html":     {Data: []byte("builtin x")},
		"overlays/x/extra.js":       {Data: []byte("builtin js")},
		"overlays/common.js":        {Data: []byte("common")},
		"themes/t.css":              {Data: []byte(":root{}")},
		"dashboard/index.html":      {Data: []byte("<html></html>")},
		"overlays/x/sub/readme.txt": {Data: []byte("t")},
	}
	dash := fstest.MapFS{"index.html": {Data: []byte("<html></html>")}}
	srv := httptest.NewUnstartedServer(nil)
	port := srv.Listener.Addr().(*net.TCPAddr).Port
	h := api.New(api.Deps{ConfigPath: filepath.Join(dir, "config.json"), Port: port})
	srv.Config.Handler = New(bus.New(), func() state.Snapshot { return state.Snapshot{} }, assets,
		WithAPI(h, port), WithDashboard(dash), WithOverlaysDir(filepath.Join(dir, "overlays")))
	srv.Start()
	t.Cleanup(srv.Close)
	return srv
}

func writeOverlay(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, "overlays", filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func fetch(t *testing.T, url string) (*http.Response, string) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var sb strings.Builder
	buf := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(buf)
		sb.Write(buf[:n])
		if err != nil {
			break
		}
	}
	return resp, sb.String()
}

func TestCustomOverlayIsServedWithSandboxHeadersAndNoRestart(t *testing.T) {
	dir := t.TempDir()
	srv := newOverlaysServer(t, dir)
	if resp, _ := fetch(t, srv.URL+"/overlays/mine/"); resp.StatusCode != 404 {
		t.Fatalf("missing folder: %d", resp.StatusCode)
	}
	writeOverlay(t, dir, "mine/index.html", "<p>custom</p>")
	writeOverlay(t, dir, "mine/js/app.js", "1")
	writeOverlay(t, dir, "mine/img/a.png", "png")
	for path, ctype := range map[string]string{
		"/overlays/mine/": "text/html; charset=utf-8", "/overlays/mine/index.html": "text/html; charset=utf-8",
		"/overlays/mine/js/app.js": "text/javascript; charset=utf-8", "/overlays/mine/img/a.png": "image/png",
	} {
		resp, body := fetch(t, srv.URL+path)
		if resp.StatusCode != 200 || body == "" {
			t.Fatalf("%s: %d %q", path, resp.StatusCode, body)
		}
		want := map[string]string{
			"Content-Type": ctype, "X-Content-Type-Options": "nosniff", "Access-Control-Allow-Origin": "*",
			"Content-Security-Policy": "sandbox allow-scripts", "Cache-Control": "no-cache",
		}
		for k, v := range want {
			if got := resp.Header.Get(k); got != v {
				t.Errorf("%s: %s = %q, want %q", path, k, got, v)
			}
		}
	}
	writeOverlay(t, dir, "mine/index.html", "<p>changed</p>")
	if _, body := fetch(t, srv.URL+"/overlays/mine/"); body != "<p>changed</p>" {
		t.Errorf("edit not picked up: %q", body)
	}
}

func TestCustomOverlayWithoutSlashRedirects(t *testing.T) {
	dir := t.TempDir()
	srv := newOverlaysServer(t, dir)
	writeOverlay(t, dir, "mine/index.html", "ok")
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Get(srv.URL + "/overlays/mine")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusMovedPermanently || resp.Header.Get("Location") != "/overlays/mine/" {
		t.Errorf("%d %q", resp.StatusCode, resp.Header.Get("Location"))
	}
	if resp, _ := fetch(t, srv.URL+"/overlays/ghost"); resp.StatusCode != 404 {
		t.Errorf("ghost: %d", resp.StatusCode)
	}
}

func TestBuiltInOverlaysWinAndKeepTheirHeaders(t *testing.T) {
	dir := t.TempDir()
	writeOverlay(t, dir, "x/index.html", "custom x")
	writeOverlay(t, dir, "x/other.js", "custom js")
	srv := newOverlaysServer(t, dir)
	for path, want := range map[string]string{
		"/overlays/x/": "builtin x", "/overlays/x/index.html": "builtin x", "/overlays/x/extra.js": "builtin js", "/overlays/common.js": "common",
	} {
		resp, body := fetch(t, srv.URL+path)
		if resp.StatusCode != 200 || body != want {
			t.Errorf("%s: %d %q", path, resp.StatusCode, body)
		}
		if resp.Header.Get("Content-Security-Policy") != overlayCSP || resp.Header.Get("Access-Control-Allow-Origin") != "" {
			t.Errorf("%s: built-in got custom headers: %v", path, resp.Header)
		}
	}
	if resp, _ := fetch(t, srv.URL+"/overlays/x/other.js"); resp.StatusCode != 404 {
		t.Errorf("custom file leaked into a built-in id: %d", resp.StatusCode)
	}
}

func TestCustomOverlayRefusesTraversalAndUnsafeFiles(t *testing.T) {
	dir := t.TempDir()
	writeOverlay(t, dir, "mine/index.html", "ok")
	writeOverlay(t, dir, "mine/.hidden.html", "#bad")
	writeOverlay(t, dir, "mine/Up.html", "#bad")
	writeOverlay(t, dir, "mine/tool.exe", "#bad")
	writeOverlay(t, dir, "mine/sub/x.js", "#bad")
	writeOverlay(t, dir, "mine/big.js", strings.Repeat("a", 2<<20+1))
	if err := os.WriteFile(filepath.Join(dir, "secret.html"), []byte("#bad"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte("#bad"), 0o600); err != nil {
		t.Fatal(err)
	}
	srv := newOverlaysServer(t, dir)
	for _, p := range []string{
		"/overlays/mine/../secret.html", "/overlays/mine/%2e%2e/secret.html", "/overlays/mine/%2e%2e%2fsecret.html",
		"/overlays/mine/..%5csecret.html", "/overlays/mine/%5c..%5csecret.html", "/overlays/mine/index.html%00", "/overlays/mine/index.html%00.css",
		"/overlays/..%2f..%2fconfig.json", "/overlays/%2e%2e/config.json", "/overlays/mine/%2e%2e/%2e%2e/config.json",
		"/overlays/mine/.hidden.html", "/overlays/mine/up.html", "/overlays/mine/INDEX.HTML", "/overlays/mine/tool.exe", "/overlays/mine/big.js",
		"/overlays/mine/sub/", "/overlays/mine/sub", "/overlays/mine//index.html", "/overlays/Mine/index.html", "/overlays/mine/index.html::$DATA",
		"/overlays/mine/index.html:stream", "/overlays/mine/overlay.json", "/overlays/mine/.", "/overlays/mine/..",
	} {
		resp, body := fetch(t, srv.URL+p)
		if resp.StatusCode == 200 && strings.Contains(body, "#bad") || strings.HasPrefix(body, "aaaa") {
			t.Errorf("%s: %d leaked %.20q", p, resp.StatusCode, body)
		}
	}
	for _, p := range []string{"/overlays/mine/up.html", "/overlays/mine/INDEX.HTML", "/overlays/mine/sub/", "/overlays/mine/tool.exe", "/overlays/mine/.hidden.html", "/overlays/Mine/index.html"} {
		if resp, _ := fetch(t, srv.URL+p); resp.StatusCode != 404 {
			t.Errorf("%s: %d, want 404", p, resp.StatusCode)
		}
	}
	if resp, body := fetch(t, srv.URL+"/overlays/mine/Up.html"); resp.StatusCode != 200 || body != "#bad" {
		t.Errorf("exact-case name must be served: %d", resp.StatusCode)
	}
}

func TestCustomOverlaySymlinkIsRefused(t *testing.T) {
	dir := t.TempDir()
	writeOverlay(t, dir, "mine/index.html", "ok")
	if err := os.WriteFile(filepath.Join(dir, "secret.js"), []byte("#bad"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "secret.js"), filepath.Join(dir, "overlays", "mine", "link.js")); err != nil {
		t.Skipf("symlinks are not available here: %v", err)
	}
	srv := newOverlaysServer(t, dir)
	if resp, body := fetch(t, srv.URL+"/overlays/mine/link.js"); resp.StatusCode == 200 || strings.Contains(body, "#bad") {
		t.Errorf("symlink served: %d %q", resp.StatusCode, body)
	}
}

func TestCustomOverlayHardLinkIsServed(t *testing.T) {
	dir := t.TempDir()
	writeOverlay(t, dir, "mine/index.html", "ok")
	if err := os.WriteFile(filepath.Join(dir, "src.js"), []byte("linked"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(filepath.Join(dir, "src.js"), filepath.Join(dir, "overlays", "mine", "hard.js")); err != nil {
		t.Skipf("hard links are not available here: %v", err)
	}
	srv := newOverlaysServer(t, dir)
	if resp, body := fetch(t, srv.URL+"/overlays/mine/hard.js"); resp.StatusCode != 200 || body != "linked" {
		t.Errorf("hard link: %d %q", resp.StatusCode, body)
	}
}

func TestCustomOverlayRespectsHostGuardAndDashboardCSP(t *testing.T) {
	dir := t.TempDir()
	writeOverlay(t, dir, "mine/index.html", "ok")
	srv := newOverlaysServer(t, dir)
	if code, _ := getHost(t, srv.URL+"/overlays/mine/", "evil.example"); code != 403 {
		t.Errorf("host guard: %d", code)
	}
	resp, _ := fetch(t, srv.URL+"/")
	if resp.Header.Get("Content-Security-Policy") != dashboardCSP {
		t.Errorf("dashboard CSP = %q", resp.Header.Get("Content-Security-Policy"))
	}
}

func TestOverlaysDirThatIsAFileNeverBreaksServing(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "overlays"), []byte("not a folder"), 0o600); err != nil {
		t.Fatal(err)
	}
	srv := newOverlaysServer(t, dir)
	if resp, _ := fetch(t, srv.URL+"/overlays/mine/"); resp.StatusCode != 404 {
		t.Errorf("custom: %d", resp.StatusCode)
	}
	if resp, body := fetch(t, srv.URL+"/overlays/x/"); resp.StatusCode != 200 || body != "builtin x" {
		t.Errorf("built-in: %d %q", resp.StatusCode, body)
	}
}

const builtinOverlayCSP = "default-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'"

func TestBuiltInOverlaysSendTheirCSPAndEveryReferencedFileLoads(t *testing.T) {
	srv := httptest.NewServer(New(bus.New(), func() state.Snapshot { return state.Snapshot{} }, web.FS, WithOverlaysDir(t.TempDir())))
	defer srv.Close()
	ref := regexp.MustCompile(`(?:src|href)="(/[^"]+)"`)
	for _, name := range []string{"now-playing", "result", "history", "state", "alert", "keys", "curve", "session"} {
		resp, err := http.Get(srv.URL + "/overlays/" + name + "/")
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 || resp.Header.Get("Content-Security-Policy") != builtinOverlayCSP {
			t.Errorf("%s: status %d, CSP %q", name, resp.StatusCode, resp.Header.Get("Content-Security-Policy"))
		}
		for _, m := range ref.FindAllStringSubmatch(string(body), -1) {
			r, err := http.Get(srv.URL + m[1])
			if err != nil {
				t.Fatal(err)
			}
			r.Body.Close()
			if r.StatusCode != 200 {
				t.Errorf("%s: %s answers %d", name, m[1], r.StatusCode)
			}
		}
	}
	r, err := http.Get(srv.URL + "/themes/fonts/VT323-Regular.ttf")
	if err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	if r.StatusCode != 200 {
		t.Errorf("theme font answers %d", r.StatusCode)
	}
}

func TestCustomOverlaysKeepTheirSandboxCSP(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "mine"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "mine", "index.html"), []byte("<p>x</p>"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(New(bus.New(), func() state.Snapshot { return state.Snapshot{} }, web.FS, WithOverlaysDir(dir)))
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/overlays/mine/")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if got := resp.Header.Get("Content-Security-Policy"); got != "sandbox allow-scripts" {
		t.Errorf("custom overlay CSP = %q", got)
	}
}
