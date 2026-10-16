package api

import (
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

func resetEnv(t *testing.T) (env, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	return newEnv(t, func(d *Deps) { d.ResetSession = func() { calls.Add(1) } }), &calls
}

func (e env) reset(method, host, body string, hdr map[string]string) int {
	h := map[string]string{"Content-Type": "application/json", "X-Requested-With": "qsc"}
	for k, v := range hdr {
		if v == "" {
			delete(h, k)
		} else {
			h[k] = v
		}
	}
	return do(e.h, method, host, "/api/session/reset", body, h).Code
}

func TestSessionResetAcceptsAnEmptyObjectOrNoBody(t *testing.T) {
	for _, body := range []string{"{}", "", " {} \n"} {
		e, calls := resetEnv(t)
		if got := e.reset("POST", "127.0.0.1:5555", body, nil); got != http.StatusOK || calls.Load() != 1 {
			t.Errorf("body %q: status %d, calls %d", body, got, calls.Load())
		}
	}
}

func TestSessionResetAnswersJSON(t *testing.T) {
	e, _ := resetEnv(t)
	w := do(e.h, "POST", "127.0.0.1:5555", "/api/session/reset", "{}", map[string]string{"Content-Type": "application/json", "X-Requested-With": "qsc"})
	var body map[string]string
	decode(t, w, &body)
	if body["status"] != "reset" || w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("body %v headers %v", body, w.Header())
	}
}

func TestSessionResetGuards(t *testing.T) {
	cases := []struct {
		name, method, host, body string
		hdr                      map[string]string
		want                     int
	}{
		{"wrong host", "POST", "evil.example", "{}", nil, 403},
		{"missing header", "POST", "127.0.0.1:5555", "{}", map[string]string{"X-Requested-With": ""}, 403},
		{"cross origin", "POST", "127.0.0.1:5555", "{}", map[string]string{"Origin": "http://evil.example"}, 403},
		{"wrong content type", "POST", "127.0.0.1:5555", "{}", map[string]string{"Content-Type": "text/plain"}, 415},
		{"huge body", "POST", "127.0.0.1:5555", strings.Repeat("a", maxBody+1), nil, 413},
		{"get", "GET", "127.0.0.1:5555", "", nil, 405},
		{"put", "PUT", "127.0.0.1:5555", "{}", nil, 405},
		{"not json", "POST", "127.0.0.1:5555", "nope", nil, 400},
		{"array", "POST", "127.0.0.1:5555", "[]", nil, 400},
		{"fields", "POST", "127.0.0.1:5555", `{"a":1}`, nil, 400},
	}
	for _, c := range cases {
		e, calls := resetEnv(t)
		if got := e.reset(c.method, c.host, c.body, c.hdr); got != c.want {
			t.Errorf("%s: status %d, want %d", c.name, got, c.want)
		}
		if calls.Load() != 0 {
			t.Errorf("%s: the session was reset", c.name)
		}
	}
}

func TestSessionResetWithoutASessionStore(t *testing.T) {
	e := newEnv(t)
	if got := e.reset("POST", "127.0.0.1:5555", "{}", nil); got != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", got)
	}
}
