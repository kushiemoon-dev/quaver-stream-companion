package server

import (
	"bytes"
	"io/fs"
	"net/http"
	"strings"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/overlays"
)

// WithOverlaysDir serves the custom overlays of dir on /overlays/<id>/. Built-in overlays always win.
func WithOverlaysDir(dir string) Option {
	return func(s *settings) { s.overlaysDir = dir }
}

// customOverlays answers /overlays/<id>/... from the custom folder for ids that are not built in, and defers to next otherwise.
// A custom overlay runs in a sandbox with an opaque origin: it can read the public routes but cannot call the write API.
func customOverlays(dir string, assets fs.FS, next http.Handler) http.Handler {
	if dir == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rest, _ := strings.CutPrefix(r.URL.Path, "/overlays/")
		id, rel, slash := strings.Cut(rest, "/")
		if !overlays.ID.MatchString(id) || overlays.IsBuiltin(assets, id) {
			next.ServeHTTP(w, r)
			return
		}
		if strings.HasSuffix(rel, "/") {
			http.NotFound(w, r)
			return
		}
		f, err := overlays.Read(dir, id, rel)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if !slash {
			http.Redirect(w, r, r.URL.Path+"/", http.StatusMovedPermanently)
			return
		}
		h := w.Header()
		h.Set("Content-Type", f.Type)
		h.Set("Cache-Control", "no-cache")
		h.Set("Access-Control-Allow-Origin", "*")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Content-Security-Policy", "sandbox allow-scripts")
		http.ServeContent(w, r, "", f.ModTime, bytes.NewReader(f.Data))
	})
}
