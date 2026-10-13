package server

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/state"
)

// crossOrigin reports whether r carries an Origin header that does not match
// the Host it was sent to. A request without Origin (OBS, curl, a same-origin
// EventSource) is not cross-origin; an opaque "null" origin is.
func crossOrigin(r *http.Request) bool {
	origins := r.Header.Values("Origin")
	if len(origins) == 0 {
		return false
	}
	if len(origins) > 1 {
		return true
	}
	u, err := url.Parse(origins[0])
	if err != nil || u.Host == "" {
		return true
	}
	return !strings.EqualFold(u.Host, r.Host)
}

// publicSnapshot returns the snapshot to send to r: cross-origin readers get it without the keys field.
func publicSnapshot(r *http.Request, s state.Snapshot) any {
	if !crossOrigin(r) {
		return s
	}
	raw, err := json.Marshal(s)
	if err != nil {
		return map[string]json.RawMessage{}
	}
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil {
		return map[string]json.RawMessage{}
	}
	delete(m, "keys")
	return m
}
