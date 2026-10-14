package api

import (
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

func quitEnv(t *testing.T) (env, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	return newEnv(t, func(d *Deps) { d.Quit = func() { calls.Add(1) } }), &calls
}

func (e env) quit(method, host, body string, hdr map[string]string) int {
	h := map[string]string{"Content-Type": "application/json", "X-Requested-With": "qsc"}
	for k, v := range hdr {
		if v == "" {
			delete(h, k)
		} else {
			h[k] = v
		}
	}
	return do(e.h, method, host, "/api/quit", body, h).Code
}

func TestQuitAnswers202ThenStops(t *testing.T) {
	e, calls := quitEnv(t)
	w := do(e.h, "POST", "127.0.0.1:5555", "/api/quit", "", map[string]string{"Content-Type": "application/json", "X-Requested-With": "qsc"})
	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d", w.Code)
	}
	var body map[string]string
	decode(t, w, &body)
	if body["status"] != "stopping" || len(body) != 1 {
		t.Fatalf("body = %v", body)
	}
	if !w.Flushed {
		t.Error("the response must be flushed before the shutdown starts")
	}
	if calls.Load() != 1 {
		t.Fatalf("quit calls = %d, want 1", calls.Load())
	}
	if got := w.Header().Get("Content-Length"); got != strconv.Itoa(w.Body.Len()) {
		t.Errorf("Content-Length = %q, body is %d bytes", got, w.Body.Len())
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("quit must not send CORS headers")
	}
}

func TestQuitRepeatedCallsStopOnce(t *testing.T) {
	e, calls := quitEnv(t)
	for i := 0; i < 3; i++ {
		if got := e.quit("POST", "127.0.0.1:5555", "{}", nil); got != http.StatusAccepted {
			t.Fatalf("call %d: status = %d", i, got)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("quit calls = %d, want 1", calls.Load())
	}
}

func TestQuitGuards(t *testing.T) {
	cases := []struct {
		name   string
		method string
		host   string
		body   string
		hdr    map[string]string
		want   int
	}{
		{"wrong host", "POST", "evil.example", "{}", nil, 403},
		{"wrong port", "POST", "127.0.0.1:9", "{}", nil, 403},
		{"missing header", "POST", "127.0.0.1:5555", "{}", map[string]string{"X-Requested-With": ""}, 403},
		{"wrong content type", "POST", "127.0.0.1:5555", "{}", map[string]string{"Content-Type": "text/plain"}, 415},
		{"no content type", "POST", "127.0.0.1:5555", "{}", map[string]string{"Content-Type": ""}, 415},
		{"bad origin", "POST", "127.0.0.1:5555", "{}", map[string]string{"Origin": "http://evil.example"}, 403},
		{"huge body", "POST", "127.0.0.1:5555", strings.Repeat("a", maxBody+1), nil, 413},
		{"get", "GET", "127.0.0.1:5555", "", nil, 405},
		{"put", "PUT", "127.0.0.1:5555", "{}", nil, 405},
		{"delete", "DELETE", "127.0.0.1:5555", "{}", nil, 405},
	}
	for _, c := range cases {
		e, calls := quitEnv(t)
		if got := e.quit(c.method, c.host, c.body, c.hdr); got != c.want {
			t.Errorf("%s: status %d, want %d", c.name, got, c.want)
		}
		if calls.Load() != 0 {
			t.Errorf("%s: quit was called", c.name)
		}
	}
}

func TestQuitAcceptsSameOriginAndCharset(t *testing.T) {
	e, calls := quitEnv(t)
	hdr := map[string]string{"Origin": "http://127.0.0.1:5555", "Content-Type": "application/json; charset=utf-8"}
	if got := e.quit("POST", "127.0.0.1:5555", "", hdr); got != http.StatusAccepted || calls.Load() != 1 {
		t.Fatalf("status = %d, calls = %d", got, calls.Load())
	}
}

func TestQuitAsksTheClientToCloseTheConnection(t *testing.T) {
	e, _ := quitEnv(t)
	w := do(e.h, "POST", "127.0.0.1:5555", "/api/quit", "", map[string]string{"Content-Type": "application/json", "X-Requested-With": "qsc"})
	if w.Header().Get("Connection") != "close" {
		t.Fatalf("Connection = %q, want close", w.Header().Get("Connection"))
	}
}
