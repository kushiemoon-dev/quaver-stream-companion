package api

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func diagEnv(t *testing.T) env {
	return newEnv(t, func(d *Deps) {
		d.Diagnostics = func(context.Context) string { return "REPORT BODY" }
	})
}

func TestDiagnosticsSameOriginGet(t *testing.T) {
	e := diagEnv(t)
	for name, hdr := range map[string]map[string]string{
		"no origin headers":  nil,
		"same origin":        {"Origin": "http://127.0.0.1:5555"},
		"fetch same-origin":  {"Sec-Fetch-Site": "same-origin"},
		"fetch direct visit": {"Sec-Fetch-Site": "none"},
	} {
		w := do(e.h, "GET", "127.0.0.1:5555", "/api/diagnostics", "", hdr)
		var body struct {
			Report string `json:"report"`
		}
		if w.Code != http.StatusOK {
			t.Fatalf("%s: status %d", name, w.Code)
		}
		decode(t, w, &body)
		if body.Report != "REPORT BODY" {
			t.Fatalf("%s: report %q", name, body.Report)
		}
		if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Fatalf("%s: CORS header %q must not be set", name, got)
		}
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("%s: must not be cached", name)
		}
	}
}

func TestDiagnosticsRefusesCrossOriginAndForeignHosts(t *testing.T) {
	e := diagEnv(t)
	for name, hdr := range map[string]map[string]string{
		"other origin":     {"Origin": "http://evil.example"},
		"other port":       {"Origin": "http://127.0.0.1:5556"},
		"opaque origin":    {"Origin": "null"},
		"fetch cross-site": {"Sec-Fetch-Site": "cross-site"},
		"fetch same-site":  {"Sec-Fetch-Site": "same-site"},
	} {
		w := do(e.h, "GET", "127.0.0.1:5555", "/api/diagnostics", "", hdr)
		if w.Code != http.StatusForbidden || strings.Contains(w.Body.String(), "REPORT BODY") || w.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Fatalf("%s: status %d body %q", name, w.Code, w.Body.String())
		}
	}
	if w := do(e.h, "GET", "evil.example:5555", "/api/diagnostics", "", nil); w.Code != http.StatusForbidden {
		t.Fatalf("foreign host: status %d", w.Code)
	}
}

func TestDiagnosticsOnlyAcceptsGet(t *testing.T) {
	e := diagEnv(t)
	hdr := map[string]string{"Content-Type": "application/json", "X-Requested-With": "qsc"}
	if w := do(e.h, "POST", "127.0.0.1:5555", "/api/diagnostics", "{}", hdr); w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST: status %d", w.Code)
	}
}

func TestConfigResponseNamesTheFoldersAndVersion(t *testing.T) {
	e := newEnv(t, func(d *Deps) { d.OverlaysDir = "/o"; d.ThemesDir = "/t" })
	var body struct {
		ConfigPath  string `json:"config_path"`
		OverlaysDir string `json:"overlays_dir"`
		ThemesDir   string `json:"themes_dir"`
		Version     string `json:"version"`
	}
	decode(t, e.get("/api/config"), &body)
	if body.ConfigPath != e.cfgPath || body.OverlaysDir != "/o" || body.ThemesDir != "/t" || body.Version != "test-1" {
		t.Fatalf("body = %+v", body)
	}
}
