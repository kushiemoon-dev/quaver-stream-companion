package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/scores"
)

const (
	scoresPrefix    = "/api/scores/"
	timelineSuffix  = "/timeline"
	timelineMissing = "no judgment timeline for this score"
)

// scoreID extracts the id from /api/scores/{id}/timeline.
func scoreID(path string) (int64, bool) {
	rest, ok := strings.CutPrefix(path, scoresPrefix)
	if !ok {
		return 0, false
	}
	raw, ok := strings.CutSuffix(rest, timelineSuffix)
	if !ok || raw == "" || strings.TrimLeft(raw, "0123456789") != "" {
		return 0, false
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	return id, err == nil && id > 0
}

func (a api) timeline(w http.ResponseWriter, r *http.Request) {
	id, ok := scoreID(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "not found", "")
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed", "")
		return
	}
	if a.d.Timeline == nil {
		writeError(w, http.StatusInternalServerError, "judgment timelines are unavailable", "")
		return
	}
	tl, err := a.d.Timeline(r.Context(), id)
	switch {
	case errors.Is(err, scores.ErrNotFound):
		writeError(w, http.StatusNotFound, "score not found", "")
	case errors.Is(err, scores.ErrNoTimeline):
		writeError(w, http.StatusNotFound, timelineMissing, "")
	case errors.Is(err, ErrUnavailable):
		writeError(w, http.StatusServiceUnavailable, err.Error(), "")
	case scores.IsBusy(err):
		writeError(w, http.StatusServiceUnavailable, busyMessage, "")
	case err != nil:
		writeError(w, http.StatusInternalServerError, "could not read the judgment timeline", "")
	default:
		writeJSON(w, http.StatusOK, tl)
	}
}
