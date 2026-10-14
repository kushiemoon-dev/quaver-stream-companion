package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/testutil"
)

type runResult struct {
	code       int
	out, errw  string
	logPath    string
	configPath string
	quaverPath string
}

// runUntil serves with extra args, waits until ready(body of /state) holds, stops, and returns what was printed.
func runUntil(t *testing.T, d *testutil.FakeDir, ready func(string) bool, extra ...string) runResult {
	t.Helper()
	cfg := filepath.Join(t.TempDir(), "config.json")
	port := freePort(t)
	args := append([]string{"--quaver-dir", d.Path, "--port", fmt.Sprint(port), "--config", cfg}, extra...)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	var out, errw bytes.Buffer
	go func() { done <- run(ctx, args, &out, &errw) }()
	url := fmt.Sprintf("http://127.0.0.1:%d/state", port)
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		if resp, err := http.Get(url); err == nil {
			buf := new(bytes.Buffer)
			buf.ReadFrom(resp.Body)
			resp.Body.Close()
			if ready == nil || ready(buf.String()) {
				break
			}
		}
		time.Sleep(30 * time.Millisecond)
	}
	cancel()
	var code int
	select {
	case code = <-done:
	case <-time.After(4 * time.Second):
		t.Fatal("serve did not stop")
	}
	return runResult{code, out.String(), errw.String(), filepath.Join(filepath.Dir(cfg), "qsc.log"), cfg, d.Path}
}

func readLog(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("qsc.log: %v", err)
	}
	return string(data)
}

func TestServeLogsTheLifecycleWithoutPersonalData(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	d.AddMap(t, testutil.MapRow{MD5: "m1", Artist: "SecretArtistXyz", Title: "SecretTitleXyz", DiffName: "SecretDiffXyz", Mode: 1, LastPlayedMS: time.Now().UnixMilli(), Difficulty: 10})
	os.WriteFile(d.LogPath(), []byte("[1:00:00] - RUNTIME - IMPORTANT: Screen has been switched to type: `Gameplay`\n[1:00:01] - RUNTIME - Logged in as SecretPlayerXyz\n"), 0o644)
	res := runUntil(t, d, func(b string) bool { return strings.Contains(b, "SecretTitleXyz") })
	if res.code != 0 {
		t.Fatalf("code %d", res.code)
	}
	log := readLog(t, res.logPath)
	for _, want := range []string{"qsc started", "qsc stopped", "Quaver folder found", "game state", "state=gameplay", "map started"} {
		if !strings.Contains(log, want) {
			t.Errorf("log lacks %q:\n%s", want, log)
		}
	}
	for _, bad := range []string{"Secret", "SecretPlayerXyz"} {
		if strings.Contains(log, bad) || strings.Contains(res.errw, "SecretPlayerXyz") {
			t.Errorf("personal data %q reached the log or console", bad)
		}
	}
	if !strings.Contains(res.errw, "game state") {
		t.Errorf("log lines must also reach the console, got %q", res.errw)
	}
}

func TestVerboseFlagAddsDebugLines(t *testing.T) {
	for _, flagName := range []string{"-v", "--verbose"} {
		res := runUntil(t, testutil.NewQuaverDir(t), nil, flagName)
		if !strings.Contains(readLog(t, res.logPath), "level=DEBUG") {
			t.Errorf("%s: no debug lines", flagName)
		}
	}
	res := runUntil(t, testutil.NewQuaverDir(t), nil)
	if strings.Contains(readLog(t, res.logPath), "level=DEBUG") {
		t.Error("debug lines without -v")
	}
}

func TestLogFileFailureNeverBlocksStartup(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	cfgDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(cfgDir, "qsc.log"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(cfgDir, "config.json")
	port := freePort(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	var out, errw bytes.Buffer
	go func() {
		done <- run(ctx, []string{"--quaver-dir", d.Path, "--port", fmt.Sprint(port), "--config", cfg}, &out, &errw)
	}()
	up := false
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline) && !up; time.Sleep(30 * time.Millisecond) {
		if resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/state", port)); err == nil {
			resp.Body.Close()
			up = true
		}
	}
	cancel()
	<-done
	if !up {
		t.Fatal("qsc did not start with an unusable log file")
	}
	if !strings.Contains(errw.String(), "log file") {
		t.Fatalf("the log problem must be shown: %q", errw.String())
	}
}

func TestCrashExitsTwoWithTheHint(t *testing.T) {
	t.Cleanup(afterStart.set(func() { panic("boom-test") }))
	d := testutil.NewQuaverDir(t)
	cfg := filepath.Join(t.TempDir(), "config.json")
	var out, errw bytes.Buffer
	args := []string{"--quaver-dir", d.Path, "--port", fmt.Sprint(freePort(t)), "--config", cfg}
	if code := run(context.Background(), args, &out, &errw); code != 2 {
		t.Fatalf("code = %d", code)
	}
	if !strings.Contains(errw.String(), `qsc crashed: see qsc.log, and run "qsc doctor --report" for a bug report`) {
		t.Fatalf("stderr = %q", errw.String())
	}
	log := readLog(t, filepath.Join(filepath.Dir(cfg), "qsc.log"))
	if !strings.Contains(log, "boom-test") || !strings.Contains(log, "goroutine") {
		t.Fatalf("panic and stack must be in the log:\n%s", log)
	}
}

func TestGuardRecoversAGoroutinePanic(t *testing.T) {
	var got any
	guard(func() { panic("worker down") }, func(r any) { got = r })
	if got != "worker down" {
		t.Fatalf("got %v", got)
	}
	called := false
	guard(func() {}, func(any) { called = true })
	if called {
		t.Fatal("onPanic must not run without a panic")
	}
}

func TestUsageListsCommandsAndFlags(t *testing.T) {
	for _, arg := range []string{"-h", "--help", "help"} {
		var out, errw bytes.Buffer
		if code := run(context.Background(), []string{arg}, &out, &errw); code != 0 {
			t.Fatalf("%s: code %d", arg, code)
		}
		for _, want := range []string{"serve", "doctor", "stop", "version", "--version", "--port", "--config", "--quaver-dir", "--open", "--verbose", "--report"} {
			if !strings.Contains(out.String(), want) {
				t.Errorf("%s: usage lacks %q:\n%s", arg, want, out.String())
			}
		}
	}
}

func TestDoctorReportPrintsTheSupportReport(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	cfg := writeConfig(t, `{"port": 7321}`)
	var out, errw bytes.Buffer
	args := []string{"doctor", "--report", "--config", cfg, "--quaver-dir", d.Path, "--port", fmt.Sprint(freePort(t))}
	if code := run(context.Background(), args, &out, &errw); code != 0 {
		t.Fatalf("code %d, stderr %q", code, errw.String())
	}
	for _, want := range []string{"support report", "Review this report before you share it", "[ OK ] quaver_dir", `"port": 7321`} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("report lacks %q", want)
		}
	}
}

func TestDiagnosticsEndpointOnALiveServer(t *testing.T) {
	s := startServer(t)
	code, body := s.req(t, "GET", "/api/diagnostics", "", "", nil)
	if code != 200 || !strings.Contains(body, "support report") || !strings.Contains(body, "qsc.log") {
		t.Fatalf("code %d body %q", code, body)
	}
	if code, _ := s.req(t, "GET", "/api/diagnostics", "", "", map[string]string{"Origin": "http://evil.example"}); code != 403 {
		t.Fatalf("cross-origin code %d", code)
	}
}
