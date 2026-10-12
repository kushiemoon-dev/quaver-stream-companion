package api

import (
	"fmt"
	"mime"
	"net/http"
	"strings"
)

// maxBody is the largest request body accepted on mutating requests.
const maxBody = 64 << 10

// GuardHost rejects requests whose Host is not this server (DNS rebinding) and
// applies the extra checks required on mutating requests.
func GuardHost(port int, next http.Handler) http.Handler {
	local := fmt.Sprintf("127.0.0.1:%d", port)
	named := fmt.Sprintf("localhost:%d", port)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != local && !strings.EqualFold(r.Host, named) {
			writeError(w, http.StatusForbidden, "forbidden host", "")
			return
		}
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
		default:
			if status, msg := checkMutation(r); status != 0 {
				writeError(w, status, msg, "")
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, maxBody)
		}
		next.ServeHTTP(w, r)
	})
}

func checkMutation(r *http.Request) (int, string) {
	if r.Header.Get("X-Requested-With") != "qsc" {
		return http.StatusForbidden, "missing X-Requested-With header"
	}
	if origins := r.Header.Values("Origin"); len(origins) > 0 && (len(origins) > 1 || origins[0] != "http://"+r.Host) {
		return http.StatusForbidden, "forbidden origin"
	}
	if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "application/json" {
		return http.StatusUnsupportedMediaType, "Content-Type must be application/json"
	}
	if r.ContentLength > maxBody {
		return http.StatusRequestEntityTooLarge, "request body too large"
	}
	return 0, ""
}
