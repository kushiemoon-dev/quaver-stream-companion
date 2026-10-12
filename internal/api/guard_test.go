package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testPort = 5555

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
}

func do(h http.Handler, method, host, target, body string, hdr map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, target, strings.NewReader(body))
	r.Host = host
	for k, v := range hdr {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestGuardHost(t *testing.T) {
	h := GuardHost(testPort, okHandler())
	cases := map[string]int{
		"127.0.0.1:5555":      http.StatusNoContent,
		"localhost:5555":      http.StatusNoContent,
		"LOCALHOST:5555":      http.StatusNoContent,
		"evil.example:5555":   http.StatusForbidden,
		"evil.example":        http.StatusForbidden,
		"127.0.0.1":           http.StatusForbidden,
		"127.0.0.1:5556":      http.StatusForbidden,
		"localhost.evil:5555": http.StatusForbidden,
		"":                    http.StatusForbidden,
	}
	for host, want := range cases {
		for _, path := range []string{"/", "/api/status", "/state"} {
			if got := do(h, "GET", host, path, "", nil).Code; got != want {
				t.Errorf("Host %q %s: status %d, want %d", host, path, got, want)
			}
		}
	}
}

func TestGuardForeignHostBodyIsJSON(t *testing.T) {
	w := do(GuardHost(testPort, okHandler()), "GET", "evil.example", "/api/status", "", nil)
	if w.Code != 403 || !strings.Contains(w.Body.String(), `"error"`) || w.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("code=%d ct=%q body=%q", w.Code, w.Header().Get("Content-Type"), w.Body.String())
	}
}

func TestGuardMutations(t *testing.T) {
	h := GuardHost(testPort, okHandler())
	host := "127.0.0.1:5555"
	good := map[string]string{"Content-Type": "application/json", "X-Requested-With": "qsc"}
	with := func(k, v string) map[string]string {
		m := map[string]string{}
		for a, b := range good {
			m[a] = b
		}
		if v == "" {
			delete(m, k)
		} else {
			m[k] = v
		}
		return m
	}
	for _, method := range []string{"PUT", "POST", "DELETE", "PATCH", "FOO"} {
		cases := []struct {
			name string
			hdr  map[string]string
			body string
			want int
		}{
			{"ok", good, "{}", 204},
			{"ok with charset", with("Content-Type", "application/json; charset=utf-8"), "{}", 204},
			{"ok same origin", with("Origin", "http://"+host), "{}", 204},
			{"missing requested-with", with("X-Requested-With", ""), "{}", 403},
			{"wrong requested-with", with("X-Requested-With", "x"), "{}", 403},
			{"wrong content type", with("Content-Type", "text/plain"), "{}", 415},
			{"form content type", with("Content-Type", "application/x-www-form-urlencoded"), "{}", 415},
			{"missing content type", with("Content-Type", ""), "{}", 415},
			{"foreign origin", with("Origin", "http://evil.example"), "{}", 403},
			{"null origin", with("Origin", "null"), "{}", 403},
			{"https origin", with("Origin", "https://"+host), "{}", 403},
			{"huge body", good, strings.Repeat("a", maxBody+1), 413},
		}
		for _, c := range cases {
			if got := do(h, method, host, "/api/config", c.body, c.hdr).Code; got != c.want {
				t.Errorf("%s %s: status %d, want %d", method, c.name, got, c.want)
			}
		}
	}
}

func TestGuardLimitsStreamedBody(t *testing.T) {
	var total int
	var readErr error
	h := GuardHost(testPort, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 4096)
		for readErr == nil {
			var n int
			n, readErr = r.Body.Read(buf)
			total += n
		}
	}))
	r := httptest.NewRequest("PUT", "/api/config", strings.NewReader(strings.Repeat("a", maxBody+5)))
	r.ContentLength = -1
	r.Host = "127.0.0.1:5555"
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Requested-With", "qsc")
	h.ServeHTTP(httptest.NewRecorder(), r)
	if total > maxBody || readErr == nil || readErr.Error() == "EOF" {
		t.Fatalf("read %d bytes, error = %v; want a size limit error", total, readErr)
	}
}

func TestGuardLeavesReadsAlone(t *testing.T) {
	w := do(GuardHost(testPort, okHandler()), "GET", "localhost:5555", "/", "", map[string]string{"Origin": "http://evil.example"})
	if w.Code != 204 {
		t.Fatalf("GET with foreign Origin: %d", w.Code)
	}
}
