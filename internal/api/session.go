package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
)

// sessionReset answers POST /api/session/reset: the body is empty or an empty JSON object.
func (a api) sessionReset(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeError(w, http.StatusRequestEntityTooLarge, "request body too large", "")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error(), "")
		return
	}
	if body = bytes.TrimSpace(body); len(body) > 0 {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(body, &fields); err != nil || len(fields) > 0 {
			writeError(w, http.StatusBadRequest, "the body must be an empty JSON object", "")
			return
		}
	}
	if a.d.ResetSession == nil {
		writeError(w, http.StatusServiceUnavailable, "the session statistics are not available", "")
		return
	}
	a.d.ResetSession()
	slog.Info("session statistics reset")
	writeJSON(w, http.StatusOK, map[string]string{"status": "reset"})
}
