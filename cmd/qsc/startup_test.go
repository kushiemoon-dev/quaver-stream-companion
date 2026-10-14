package main

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/testutil"
)

func portOf(t *testing.T, raw string) int {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	var p int
	fmt.Sscan(u.Port(), &p)
	return p
}

func TestBusyPortMessageWhenQscAnswers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"version":"dev","checks":[],"uptime_s":3}`)
	}))
	defer srv.Close()
	port := portOf(t, srv.URL)
	want := fmt.Sprintf(`qsc is already running on port %d. Open http://127.0.0.1:%d/ or run "qsc stop".`, port, port)
	if got := busyPortMessage(port); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestBusyPortMessageForOtherPrograms(t *testing.T) {
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"hello":"world"}`)
	}))
	defer other.Close()
	raw, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	go func() {
		for {
			c, err := raw.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()
	for _, port := range []int{portOf(t, other.URL), raw.Addr().(*net.TCPAddr).Port} {
		want := fmt.Sprintf("port %d is used by another program. Choose another one with --port or in the config file (and update your OBS links).", port)
		if got := busyPortMessage(port); got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	}
}

func TestServeOnPortOfRunningQscExplainsAndExitsOne(t *testing.T) {
	s := startStoppable(t)
	d := testutil.NewQuaverDir(t)
	var out, errw bytes.Buffer
	args := []string{"--quaver-dir", d.Path, "--port", fmt.Sprint(s.port), "--config", s.cfg + ".other"}
	if code := run(context.Background(), args, &out, &errw); code != 1 {
		t.Fatalf("code = %d", code)
	}
	if want := fmt.Sprintf(`qsc is already running on port %d.`, s.port); !strings.Contains(errw.String(), want) {
		t.Fatalf("stderr = %q", errw.String())
	}
	if code := stop([]string{"--port", fmt.Sprint(s.port)}, &out, &errw); code != 0 {
		t.Fatalf("stop = %d", code)
	}
	s.waitExit(t)
}

func TestPauseOnErrorGating(t *testing.T) {
	cases := []struct {
		name        string
		args        []string
		code        int
		interactive bool
		wantPause   bool
	}{
		{"no args, failure, console", nil, 1, true, true},
		{"crash exit code", nil, 2, true, true},
		{"success", nil, 0, true, false},
		{"redirected", nil, 1, false, false},
		{"serve given", []string{"serve"}, 1, true, false},
		{"any flag given", []string{"--port", "9"}, 1, true, false},
		{"stop", []string{"stop"}, 1, true, false},
		{"doctor", []string{"doctor"}, 1, true, false},
		{"version", []string{"version"}, 1, true, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := &countingReader{data: "\n"}
			var out bytes.Buffer
			pauseOnError(c.args, c.code, c.interactive, in, &out)
			if paused := strings.Contains(out.String(), "Press Enter to close"); paused != c.wantPause {
				t.Fatalf("prompt shown = %v, want %v (out %q)", paused, c.wantPause, out.String())
			}
			if (in.reads > 0) != c.wantPause {
				t.Fatalf("reads = %d, want pause = %v", in.reads, c.wantPause)
			}
		})
	}
}

type countingReader struct {
	data  string
	reads int
}

func (r *countingReader) Read(p []byte) (int, error) {
	r.reads++
	if r.data == "" {
		return 0, fmt.Errorf("eof")
	}
	n := copy(p, r.data)
	r.data = r.data[n:]
	return n, nil
}

func TestBrokenConfigNamesThePathAndTheFixAndIsKept(t *testing.T) {
	for _, content := range []string{`{"port": `, `{"port": 70000}`} {
		cfg := writeConfig(t, content)
		var out, errw bytes.Buffer
		if code := run(context.Background(), []string{"--config", cfg}, &out, &errw); code != 1 {
			t.Fatalf("%s: code = %d", content, code)
		}
		msg := errw.String()
		if !strings.Contains(msg, cfg) || !strings.Contains(msg, "delete this file to reset the settings") {
			t.Fatalf("%s: stderr = %q", content, msg)
		}
		if got, _ := os.ReadFile(cfg); string(got) != content {
			t.Fatalf("broken config was rewritten: %q", got)
		}
	}
}

func TestDoctorReportsABrokenConfigAsAFailedCheck(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	cfg := writeConfig(t, `{"port": `)
	var out, errw bytes.Buffer
	args := []string{"doctor", "--config", cfg, "--quaver-dir", d.Path, "--port", fmt.Sprint(freePort(t))}
	if code := run(context.Background(), args, &out, &errw); code != 1 {
		t.Fatalf("code = %d", code)
	}
	if !strings.Contains(out.String(), "[FAIL] Config file") || !strings.Contains(out.String(), cfg) || !strings.Contains(out.String(), "[ OK ] Quaver folder") {
		t.Fatalf("stdout = %q", out.String())
	}
}

func TestStopIgnoresConfigErrors(t *testing.T) {
	cfg := writeConfig(t, "{not json")
	port, code := stopPort([]string{"--config", cfg}, &bytes.Buffer{})
	if code != -1 || port != 7878 {
		t.Fatalf("port = %d, code = %d", port, code)
	}
	cfg = writeConfig(t, `{"port": 7123}`)
	if port, code = stopPort([]string{"--config", cfg}, &bytes.Buffer{}); code != -1 || port != 7123 {
		t.Fatalf("port = %d, code = %d", port, code)
	}
	if port, code = stopPort([]string{"--config", t.TempDir()}, &bytes.Buffer{}); code != -1 || port != 7878 {
		t.Fatalf("unreadable: port = %d, code = %d", port, code)
	}
}

func TestStopReadsThePortOfAConfigThatFailsValidation(t *testing.T) {
	var errw bytes.Buffer
	cfg := writeConfig(t, `{"port": 7123, "keys": "oops"}`)
	if port, code := stopPort([]string{"--config", cfg}, &errw); code != -1 || port != 7123 || !strings.Contains(errw.String(), "port 7123") {
		t.Fatalf("port %d code %d note %q", port, code, errw.String())
	}
	errw.Reset()
	cfg = writeConfig(t, "{broken")
	if port, _ := stopPort([]string{"--config", cfg}, &errw); port != 7878 || !strings.Contains(errw.String(), "port 7878") {
		t.Fatalf("port %d note %q", port, errw.String())
	}
}

func TestStartupEndsWithTheNextSteps(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	port := freePort(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	var out, errw bytes.Buffer
	args := []string{"--quaver-dir", d.Path, "--port", fmt.Sprint(port), "--config", writeConfig(t, "")}
	go func() { done <- run(ctx, args, &out, &errw) }()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/state", port)); err == nil {
			resp.Body.Close()
			break
		}
		time.Sleep(30 * time.Millisecond)
	}
	cancel()
	<-done
	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	n := len(lines)
	if n < 2 || lines[n-2] != "Next: open the dashboard, OBS page, to add the overlays." || lines[n-1] != "Keep this window open while you stream, or run qsc stop to quit." {
		t.Fatalf("output tail = %q", lines)
	}
}

func TestBindMessageKeepsTheSystemErrorWhenNotAddressInUse(t *testing.T) {
	denied := &net.OpError{Op: "listen", Err: os.NewSyscallError("bind", fmt.Errorf("An attempt was made to access a socket in a way forbidden by its access permissions"))}
	msg := bindMessage(80, denied)
	if !strings.Contains(msg, "forbidden by its access permissions") || !strings.Contains(msg, "port 80") {
		t.Fatalf("msg = %q", msg)
	}
	inUse := &net.OpError{Op: "listen", Err: os.NewSyscallError("bind", syscall.Errno(wsaeAddrInUse))}
	if msg := bindMessage(81, inUse); !strings.Contains(msg, "used by another program") {
		t.Fatalf("msg = %q", msg)
	}
}

func TestFailedBindIsLoggedAndNotReportedAsStarted(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	cfg := filepath.Join(t.TempDir(), "config.json")
	var out, errw bytes.Buffer
	args := []string{"--quaver-dir", testutil.NewQuaverDir(t).Path, "--port", fmt.Sprint(ln.Addr().(*net.TCPAddr).Port), "--config", cfg}
	if code := run(context.Background(), args, &out, &errw); code != 1 {
		t.Fatalf("code = %d", code)
	}
	log := readLog(t, filepath.Join(filepath.Dir(cfg), "qsc.log"))
	if strings.Contains(log, "qsc started") || strings.Contains(log, "qsc stopped") || !strings.Contains(log, "cannot listen") {
		t.Fatalf("log:\n%s", log)
	}
	if !strings.Contains(errw.String(), "used by another program") {
		t.Fatalf("console: %q", errw.String())
	}
}

func TestBusyPortConsoleShowsOnlyTheFriendlyMessage(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port
	cfg := filepath.Join(t.TempDir(), "config.json")

	var out, errw bytes.Buffer
	args := []string{"--quaver-dir", d.Path, "--port", fmt.Sprint(port), "--config", cfg}
	if code := run(context.Background(), args, &out, &errw); code != 1 {
		t.Fatalf("code = %d", code)
	}
	if strings.Contains(errw.String(), "level=ERROR") || !strings.Contains(errw.String(), "is used by another program") {
		t.Fatalf("console = %q", errw.String())
	}
	data, _ := os.ReadFile(filepath.Join(filepath.Dir(cfg), "qsc.log"))
	if !strings.Contains(string(data), "cannot listen") {
		t.Fatalf("qsc.log = %q", data)
	}
}
