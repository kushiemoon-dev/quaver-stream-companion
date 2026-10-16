package api

import (
	"net/http"
	"slices"
	"strings"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/textout"
)

const textPrefix = "/api/text/"

// text answers GET /api/text/{file} with the same string the text file holds, whether or not the files are written.
func (a api) text(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, textPrefix)
	if !slices.Contains(textout.Names, name) {
		writeError(w, http.StatusNotFound, "not found", "")
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		writeError(w, http.StatusMethodNotAllowed, "method not allowed", "")
		return
	}
	snap := a.d.Snapshot()
	h := w.Header()
	h.Set("Content-Type", "text/plain; charset=utf-8")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Content-Type-Options", "nosniff")
	if r.Method == http.MethodGet {
		_, _ = w.Write([]byte(textout.Texts(snap.Map, snap.LastScore)[name]))
	}
}
