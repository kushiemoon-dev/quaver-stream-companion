package main

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/testutil"
)

type stoppable struct {
	port int
	cfg  string
	done chan int
	errw *bytes.Buffer
}

func startStoppable(t *testing.T) stoppable {
	t.Helper()
	d := testutil.NewQuaverDir(t)
	port := freePort(t)
	cfg := filepath.Join(t.TempDir(), "config.json")
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	s := stoppable{port: port, cfg: cfg, done: make(chan int, 1), errw: new(bytes.Buffer)}
	var out bytes.Buffer
	args := []string{"--quaver-dir", d.Path, "--port", fmt.Sprint(port), "--config", cfg}
	go func() { s.done <- run(ctx, args, &out, s.errw) }()
	url := fmt.Sprintf("http://127.0.0.1:%d/state", port)
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); time.Sleep(30 * time.Millisecond) {
		if resp, err := http.Get(url); err == nil {
			resp.Body.Close()
			return s
		}
	}
	t.Fatalf("server not up (stderr: %s)", s.errw.String())
	return s
}

func (s stoppable) waitExit(t *testing.T) {
	t.Helper()
	select {
	case code := <-s.done:
		if code != 0 {
			t.Fatalf("serve exit code = %d (stderr: %s)", code, s.errw.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not exit")
	}
}

func runStop(args ...string) (int, string, string) {
	var out, errw bytes.Buffer
	code := run(context.Background(), append([]string{"stop"}, args...), &out, &errw)
	return code, out.String(), errw.String()
}

func TestStopStopsARunningServer(t *testing.T) {
	s := startStoppable(t)
	code, out, errw := runStop("--port", fmt.Sprint(s.port), "--config", s.cfg)
	if code != 0 || strings.TrimSpace(out) != "qsc stopped" {
		t.Fatalf("code = %d, out = %q, err = %q", code, out, errw)
	}
	s.waitExit(t)
	if c, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", s.port), time.Second); err == nil {
		c.Close()
		t.Fatal("the port still answers")
	}
	code, out, _ = runStop("--port", fmt.Sprint(s.port), "--config", s.cfg)
	want := fmt.Sprintf("qsc is not running on port %d", s.port)
	if code != 0 || strings.TrimSpace(out) != want {
		t.Fatalf("second stop: code = %d, out = %q", code, out)
	}
}

func TestStopNotRunningIsSuccess(t *testing.T) {
	port := freePort(t)
	code, out, errw := runStop("--port", fmt.Sprint(port), "--config", filepath.Join(t.TempDir(), "c.json"))
	want := fmt.Sprintf("qsc is not running on port %d", port)
	if code != 0 || strings.TrimSpace(out) != want || errw != "" {
		t.Fatalf("code = %d, out = %q, err = %q", code, out, errw)
	}
}

func fakeServer(t *testing.T, h http.HandlerFunc) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: h}
	go srv.Serve(ln)
	t.Cleanup(func() { srv.Close() })
	return ln.Addr().(*net.TCPAddr).Port
}

func TestStopFailsWhenSomethingElseAnswers(t *testing.T) {
	cases := map[string]http.HandlerFunc{
		"not found": func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) },
		"refused":   func(w http.ResponseWriter, r *http.Request) { http.Error(w, "no", http.StatusForbidden) },
		"html":      func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "<html></html>") },
		"wrong 202": func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(202); fmt.Fprint(w, `{"status":"ok"}`) },
	}
	for name, h := range cases {
		port := fakeServer(t, h)
		code, out, errw := runStop("--port", fmt.Sprint(port), "--config", filepath.Join(t.TempDir(), "c.json"))
		if code != 1 || out != "" || !strings.Contains(errw, fmt.Sprintf("port %d", port)) {
			t.Errorf("%s: code = %d, out = %q, err = %q", name, code, out, errw)
		}
	}
}

func TestStopFailsWhenThePortKeepsAnswering(t *testing.T) {
	t.Cleanup(stopWait.set(300 * time.Millisecond))
	port := fakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		fmt.Fprint(w, `{"status":"stopping"}`)
	})
	code, out, errw := runStop("--port", fmt.Sprint(port), "--config", filepath.Join(t.TempDir(), "c.json"))
	if code != 1 || out != "" || !strings.Contains(errw, "still answering") {
		t.Fatalf("code = %d, out = %q, err = %q", code, out, errw)
	}
}

func TestStopSendsTheGuardedRequest(t *testing.T) {
	var mu sync.Mutex
	var got *http.Request
	var srv *http.Server
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv = &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		got = r
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		fmt.Fprint(w, `{"status":"stopping"}`)
		go srv.Close()
	})}
	go srv.Serve(ln)
	t.Cleanup(func() { srv.Close() })
	port := ln.Addr().(*net.TCPAddr).Port
	code, out, errw := runStop("--port", fmt.Sprint(port), "--config", filepath.Join(t.TempDir(), "c.json"))
	if code != 0 || strings.TrimSpace(out) != "qsc stopped" {
		t.Fatalf("code = %d, out = %q, err = %q", code, out, errw)
	}
	mu.Lock()
	defer mu.Unlock()
	if got == nil {
		t.Fatal("no request received")
	}
	if got.Method != "POST" || got.URL.Path != "/api/quit" || got.Host != fmt.Sprintf("127.0.0.1:%d", port) ||
		got.Header.Get("X-Requested-With") != "qsc" || got.Header.Get("Content-Type") != "application/json" || got.Header.Get("Origin") != "" {
		t.Fatalf("bad request: %s %s host=%s hdr=%v", got.Method, got.URL.Path, got.Host, got.Header)
	}
}

func TestStopPortResolution(t *testing.T) {
	t.Run("config port", func(t *testing.T) {
		s := startStoppable(t)
		cfg := writeConfig(t, fmt.Sprintf(`{"port": %d}`, s.port))
		if code, out, errw := runStop("--config", cfg); code != 0 || strings.TrimSpace(out) != "qsc stopped" {
			t.Fatalf("code = %d, out = %q, err = %q", code, out, errw)
		}
		s.waitExit(t)
	})
	t.Run("flag beats config", func(t *testing.T) {
		s := startStoppable(t)
		cfg := writeConfig(t, fmt.Sprintf(`{"port": %d}`, freePort(t)))
		if code, out, errw := runStop("--port", fmt.Sprint(s.port), "--config", cfg); code != 0 || strings.TrimSpace(out) != "qsc stopped" {
			t.Fatalf("code = %d, out = %q, err = %q", code, out, errw)
		}
		s.waitExit(t)
	})
}

func TestStopHasNoHostFlagAndRejectsBadInput(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), "c.json")
	for _, args := range [][]string{{"--host", "192.168.1.5"}, {"--quaver-dir", "x"}, {"--port", "0"}, {"--port", "70000"}, {"extra"}} {
		if code, _, _ := runStop(append(args, "--config", cfg)...); code != 2 {
			t.Errorf("%v: code = %d, want 2", args, code)
		}
	}
	if code, _, errw := runStop("-h"); code != 0 || !strings.Contains(errw, "qsc stop") {
		t.Errorf("-h: code = %d, err = %q", code, errw)
	}
}

func TestQuitEndpointStopsTheWholeApp(t *testing.T) {
	before := runtime.NumGoroutine()
	s := startStoppable(t)
	client := &http.Client{Transport: &http.Transport{DisableKeepAlives: true}}
	r, _ := http.NewRequest("POST", fmt.Sprintf("http://127.0.0.1:%d/api/quit", s.port), strings.NewReader("{}"))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Requested-With", "qsc")
	resp, err := client.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	s.waitExit(t)
	var after int
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		client.CloseIdleConnections()
		http.DefaultClient.CloseIdleConnections()
		if after = runtime.NumGoroutine(); after <= before {
			return
		}
	}
	t.Fatalf("goroutines: %d before, %d after the quit", before, after)
}

func TestStopDoesNotFollowRedirects(t *testing.T) {
	var hits int32
	target := fakeServer(t, func(w http.ResponseWriter, r *http.Request) { atomic.AddInt32(&hits, 1) })
	for _, status := range []int{301, 302, 307, 308} {
		port := fakeServer(t, func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, fmt.Sprintf("http://127.0.0.1:%d/api/quit", target), status)
		})
		code, out, errw := runStop("--port", fmt.Sprint(port), "--config", filepath.Join(t.TempDir(), "c.json"))
		if code != 1 || out != "" || !strings.Contains(errw, fmt.Sprintf("port %d", port)) {
			t.Errorf("%d: code = %d, out = %q, err = %q", status, code, out, errw)
		}
	}
	if n := atomic.LoadInt32(&hits); n != 0 {
		t.Fatalf("the redirect target received %d requests", n)
	}
}

func TestStopPortFlagIgnoresAnInvalidConfig(t *testing.T) {
	cfg := writeConfig(t, "{not json")
	port := freePort(t)
	code, out, _ := runStop("--port", fmt.Sprint(port), "--config", cfg)
	if want := fmt.Sprintf("qsc is not running on port %d", port); code != 0 || strings.TrimSpace(out) != want {
		t.Fatalf("code = %d, out = %q", code, out)
	}
	if got, code := stopPort([]string{"--config", cfg}, &bytes.Buffer{}); code != -1 || got != 7878 {
		t.Fatalf("without --port: port = %d, code = %d", got, code)
	}
}
