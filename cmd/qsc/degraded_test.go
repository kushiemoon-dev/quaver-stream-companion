package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/diagnose"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/testutil"
)

const (
	selectLine  = "[1:00:00] - RUNTIME - IMPORTANT: Screen has been switched to type: `Select`"
	resultsLine = "[1:00:00] - RUNTIME - IMPORTANT: Screen has been switched to type: `Results`"
)

// emptySteam points detection at a temporary Steam tree with no Quaver, and re-checks quickly.
func emptySteam(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	prevRoots, prevEvery := steamRoots, recheckInterval
	steamRoots = func() []string { return []string{root} }
	recheckInterval = 20 * time.Millisecond
	t.Cleanup(func() { steamRoots, recheckInterval = prevRoots, prevEvery })
	return root
}

type degradedServer struct {
	base string
	stop func() int
	errw *syncBuffer
}

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func startDegraded(t *testing.T, cfgJSON string, flags ...string) degradedServer {
	t.Helper()
	port := freePort(t)
	args := append([]string{"--config", writeConfig(t, cfgJSON), "--port", fmt.Sprint(port)}, flags...)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	var out, errw syncBuffer
	go func() { done <- run(ctx, args, &out, &errw) }()
	t.Cleanup(cancel)
	return degradedServer{
		base: fmt.Sprintf("http://127.0.0.1:%d", port),
		errw: &errw,
		stop: func() int {
			cancel()
			select {
			case code := <-done:
				return code
			case <-time.After(3 * time.Second):
				t.Error("server did not stop")
				return -1
			}
		},
	}
}

func (s degradedServer) request(t *testing.T, method, path, body string) (int, string) {
	t.Helper()
	client := &http.Client{Transport: &http.Transport{DisableKeepAlives: true}}
	var last error
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); time.Sleep(25 * time.Millisecond) {
		req, _ := http.NewRequest(method, s.base+path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Requested-With", "qsc")
		resp, err := client.Do(req)
		if err != nil {
			last = err
			continue
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return resp.StatusCode, string(b)
	}
	t.Fatalf("%s %s: %v (stderr: %s)", method, path, last, s.errw.String())
	return 0, ""
}

func (s degradedServer) checks(t *testing.T) map[string]diagnose.Check {
	t.Helper()
	code, body := s.request(t, "GET", "/api/status", "")
	if code != 200 {
		t.Fatalf("status = %d %s", code, body)
	}
	var st struct {
		Checks []diagnose.Check `json:"checks"`
	}
	if err := json.Unmarshal([]byte(body), &st); err != nil {
		t.Fatal(err)
	}
	m := map[string]diagnose.Check{}
	for _, c := range st.Checks {
		m[c.ID] = c
	}
	return m
}

func (s degradedServer) waitGame(t *testing.T, game string) {
	t.Helper()
	var body string
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); time.Sleep(25 * time.Millisecond) {
		_, body = s.request(t, "GET", "/state", "")
		if strings.Contains(body, `"game":"`+game+`"`) {
			return
		}
	}
	t.Fatalf("state never became %s: %s", game, body)
}

func TestServeStartsWithoutQuaver(t *testing.T) {
	emptySteam(t)
	s := startDegraded(t, `{}`)

	if code, body := s.request(t, "GET", "/state", ""); code != 200 || !strings.Contains(body, `"game":"unknown"`) {
		t.Fatalf("state = %d %s", code, body)
	}
	c := s.checks(t)
	if c["quaver_dir"].OK || !strings.Contains(c["quaver_dir"].Detail, "quaver folder not found") || !strings.Contains(c["quaver_dir"].Detail, "--quaver-dir") {
		t.Fatalf("quaver_dir = %+v", c["quaver_dir"])
	}
	for _, path := range []string{"/", "/overlays/now-playing/", "/api/config", "/api/themes", "/api/overlays"} {
		if code, _ := s.request(t, "GET", path, ""); code != 200 {
			t.Errorf("GET %s = %d", path, code)
		}
	}
	if code, body := s.request(t, "GET", "/api/history", ""); code != 503 || !strings.Contains(body, "not available yet") {
		t.Errorf("history without Quaver = %d %s, want 503", code, body)
	}
	if code, _ := s.request(t, "GET", "/api/scores/1/timeline", ""); code != 503 {
		t.Errorf("timeline without Quaver = %d, want 503", code)
	}
	if !strings.Contains(s.errw.String(), "Quaver") || !strings.Contains(s.errw.String(), "Settings") {
		t.Errorf("stderr lacks the advice: %q", s.errw.String())
	}
	if code := s.stop(); code != 0 {
		t.Fatalf("exit code = %d", code)
	}
}

func TestServeRecoversWhenQuaverAppears(t *testing.T) {
	root := emptySteam(t)
	s := startDegraded(t, `{}`)
	if s.checks(t)["quaver_dir"].OK {
		t.Fatal("expected a failed check first")
	}

	d := testutil.NewQuaverDir(t)
	d.AppendLog(t, selectLine)
	target := filepath.Join(root, "steamapps", "common", "Quaver")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(d.Path, target); err != nil {
		t.Fatal(err)
	}

	s.waitGame(t, "select")
	for id, c := range s.checks(t) {
		if id != "keys" && !c.OK {
			t.Errorf("check %s = %+v", id, c)
		}
	}
	if code, _ := s.request(t, "GET", "/api/history", ""); code != 200 {
		t.Errorf("history after recovery = %d", code)
	}
	s.stop()
}

func TestSavingQuaverDirWhileDegradedAppliesLive(t *testing.T) {
	emptySteam(t)
	s := startDegraded(t, `{}`)
	d := testutil.NewQuaverDir(t)
	d.AppendLog(t, resultsLine)

	code, body := s.request(t, "PUT", "/api/config", fmt.Sprintf(`{"quaver_dir": "%s"}`, jsonPath(d.Path)))
	if code != 200 || !strings.Contains(body, `"restart_required":false`) {
		t.Fatalf("put = %d %s", code, body)
	}
	s.waitGame(t, "results")
	s.stop()
}

func TestInvalidQuaverDirOverrideIsDegradedNotFatal(t *testing.T) {
	emptySteam(t)
	unc := string([]rune{0x5c, 0x5c}) + "server" + string([]rune{0x5c}) + "share"
	for name, c := range map[string]struct{ dir, want string }{
		"not a quaver folder": {t.TempDir(), "is not a Quaver folder"},
		"network path":        {unc, "network path"},
	} {
		t.Run(name, func(t *testing.T) {
			s := startDegraded(t, `{}`, "--quaver-dir", c.dir)
			got := s.checks(t)["quaver_dir"]
			if got.OK || !strings.Contains(got.Detail, c.want) {
				t.Fatalf("quaver_dir = %+v", got)
			}
			if code := s.stop(); code != 0 {
				t.Fatalf("exit code = %d", code)
			}
		})
	}
}

func TestMissingDatabaseOrLogInValidFolderIsDegraded(t *testing.T) {
	emptySteam(t)
	d := testutil.NewQuaverDir(t)
	if err := os.Remove(filepath.Join(d.Path, "quaver.db")); err != nil {
		t.Fatal(err)
	}
	s := startDegraded(t, `{}`, "--quaver-dir", d.Path)
	c := s.checks(t)
	if !c["quaver_dir"].OK || c["database"].OK || !strings.Contains(c["database"].Detail, "quaver.db") {
		t.Fatalf("checks = %+v", c)
	}
	if code, _ := s.request(t, "GET", "/state", ""); code != 200 {
		t.Fatalf("state = %d", code)
	}
	s.stop()

	d2 := testutil.NewQuaverDir(t)
	if err := os.Remove(d2.LogPath()); err != nil {
		t.Fatal(err)
	}
	s = startDegraded(t, `{}`, "--quaver-dir", d2.Path)
	if c = s.checks(t); c["log"].OK || !strings.Contains(c["log"].Detail, "runtime.log") {
		t.Fatalf("checks = %+v", c)
	}
	s.stop()
}

func TestDegradedServeLeaksNoGoroutines(t *testing.T) {
	emptySteam(t)
	runtime.GC()
	before := runtime.NumGoroutine()
	s := startDegraded(t, `{}`)
	s.checks(t)
	if code := s.stop(); code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	var after int
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); time.Sleep(25 * time.Millisecond) {
		if after = runtime.NumGoroutine(); after <= before {
			return
		}
	}
	buf := make([]byte, 1<<16)
	t.Fatalf("goroutines: %d before, %d after\n%s", before, after, buf[:runtime.Stack(buf, true)])
}

func TestDoctorPrintsAdviceWhenQuaverIsMissing(t *testing.T) {
	emptySteam(t)
	var out, errw bytes.Buffer
	args := []string{"doctor", "--config", writeConfig(t, `{}`), "--port", fmt.Sprint(freePort(t))}
	if code := run(context.Background(), args, &out, &errw); code != 1 {
		t.Fatalf("code = %d", code)
	}
	for _, want := range append([]string{"[FAIL] Quaver folder"}, diagnose.Advice()...) {
		if !strings.Contains(out.String(), want) {
			t.Errorf("doctor output lacks %q:\n%s", want, out.String())
		}
	}
}
