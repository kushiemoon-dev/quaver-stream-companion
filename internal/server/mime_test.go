package server

import (
	"io/fs"
	"mime"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/bus"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/state"
	"github.com/kushiemoon-dev/quaver-stream-companion/web"
)

func realHandler(t *testing.T, themesDir string) http.Handler {
	t.Helper()
	dash, err := fs.Sub(web.FS, "dashboard")
	if err != nil {
		t.Fatal(err)
	}
	return New(bus.New(), func() state.Snapshot { return state.Snapshot{} }, web.FS,
		WithDashboard(dash), WithThemesDir(themesDir))
}

func contentType(t *testing.T, h http.Handler, path string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	if rec.Code != 200 {
		t.Fatalf("%s: status %d", path, rec.Code)
	}
	return rec.Header().Get("Content-Type")
}

func TestStaticFilesHaveFixedContentTypes(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "mine.css"), []byte(":root{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	h := realHandler(t, dir)
	for path, want := range map[string]string{
		"/themes/pixel.css":               "text/css; charset=utf-8",
		"/themes/fonts/VT323-Regular.ttf": "font/ttf",
		"/overlays/common.js":             "text/javascript; charset=utf-8",
		"/overlays/now-playing/":          "text/html; charset=utf-8",
		"/overlays/now-playing/page.js":   "text/javascript; charset=utf-8",
		"/overlays/result/page.js":        "text/javascript; charset=utf-8",
		"/overlays/history/page.js":       "text/javascript; charset=utf-8",
		"/overlays/state/page.js":         "text/javascript; charset=utf-8",
		"/overlays/alert/page.js":         "text/javascript; charset=utf-8",
		"/overlays/keys/page.js":          "text/javascript; charset=utf-8",
		"/overlays/curve/page.js":         "text/javascript; charset=utf-8",
		"/overlays/session/page.js":       "text/javascript; charset=utf-8",
		"/":                               "text/html; charset=utf-8",
		"/app.js":                         "text/javascript; charset=utf-8",
		"/favicon.svg":                    "image/svg+xml",
		"/themes/mine.css":                "text/css; charset=utf-8",
	} {
		if got := contentType(t, h, path); got != want {
			t.Errorf("%s: Content-Type %q, want %q", path, got, want)
		}
	}
}

func TestContentTypesSurviveOSOverride(t *testing.T) {
	t.Cleanup(registerMimeTypes)
	if err := mime.AddExtensionType(".css", "text/plain"); err != nil {
		t.Fatal(err)
	}
	registerMimeTypes()
	if got := contentType(t, realHandler(t, t.TempDir()), "/themes/pixel.css"); got != "text/css; charset=utf-8" {
		t.Fatalf("Content-Type %q", got)
	}
}
