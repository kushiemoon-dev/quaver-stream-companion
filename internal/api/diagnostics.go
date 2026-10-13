package api

import (
	"net/http"
	"net/url"
	"strings"
)

// diagnostics answers the support report to pages served by qsc only: no CORS, and a request
// that a browser marks as coming from another origin or site is refused.
func (a api) diagnostics(w http.ResponseWriter, r *http.Request) {
	if foreignRequest(r) || a.d.Diagnostics == nil {
		writeError(w, http.StatusForbidden, "the diagnostics report is only available to the dashboard", "")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]string{"report": a.d.Diagnostics(r.Context())})
}

func foreignRequest(r *http.Request) bool {
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
		return true
	}
	origins := r.Header.Values("Origin")
	if len(origins) == 0 {
		return false
	}
	if len(origins) > 1 {
		return true
	}
	u, err := url.Parse(origins[0])
	return err != nil || u.Host == "" || !strings.EqualFold(u.Host, r.Host)
}
