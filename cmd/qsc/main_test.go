package main

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/testutil"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/themes"
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "qsc-cfg-")
	if err != nil {
		panic(err)
	}
	for _, k := range []string{"APPDATA", "XDG_CONFIG_HOME", "HOME", "USERPROFILE"} {
		os.Setenv(k, dir)
	}
	useFakeSampler()
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func jsonPath(p string) string { return strings.ReplaceAll(p, `\`, `\\`) }

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func TestVersion(t *testing.T) {
	var out, errw bytes.Buffer
	if code := run(context.Background(), []string{"version"}, &out, &errw); code != 0 {
		t.Fatalf("code = %d", code)
	}
	if !strings.Contains(out.String(), "quaver-stream-companion") {
		t.Fatalf("out = %q", out.String())
	}
}

func TestDoctorOKOnFakeFolder(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	var out, errw bytes.Buffer
	args := []string{"doctor", "--quaver-dir", d.Path, "--port", fmt.Sprint(freePort(t))}
	if code := run(context.Background(), args, &out, &errw); code != 0 {
		t.Fatalf("code = %d\n%s%s", code, out.String(), errw.String())
	}
	if !strings.Contains(out.String(), "quaver.db") || strings.Contains(out.String(), "[FAIL]") {
		t.Fatalf("out = %q", out.String())
	}
}

func TestDoctorFailsOnWrongFolder(t *testing.T) {
	var out, errw bytes.Buffer
	args := []string{"doctor", "--quaver-dir", t.TempDir(), "--port", fmt.Sprint(freePort(t))}
	if code := run(context.Background(), args, &out, &errw); code != 1 {
		t.Fatalf("code = %d", code)
	}
	if !strings.Contains(out.String(), "[FAIL]") || !strings.Contains(out.String(), "Quaver or Quaver.exe") {
		t.Fatalf("out = %q", out.String())
	}
}

func TestServePortInUse(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	var out, errw bytes.Buffer
	args := []string{"--quaver-dir", d.Path, "--port", fmt.Sprint(port)}
	if code := run(context.Background(), args, &out, &errw); code != 1 {
		t.Fatalf("code = %d", code)
	}
	if !strings.Contains(errw.String(), fmt.Sprintf("port %d is used by another program", port)) {
		t.Fatalf("stderr = %q", errw.String())
	}
}

func TestServeRejectsBadPort(t *testing.T) {
	var out, errw bytes.Buffer
	if code := run(context.Background(), []string{"--port", "70000"}, &out, &errw); code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	if !strings.Contains(errw.String(), "port") {
		t.Fatalf("stderr = %q", errw.String())
	}
}

func TestServeAnswersStateThenStops(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	port := freePort(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	var out, errw bytes.Buffer
	go func() {
		done <- run(ctx, []string{"--quaver-dir", d.Path, "--port", fmt.Sprint(port)}, &out, &errw)
	}()

	url := fmt.Sprintf("http://127.0.0.1:%d/state", port)
	var body string
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			buf := new(bytes.Buffer)
			buf.ReadFrom(resp.Body)
			resp.Body.Close()
			body = buf.String()
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !strings.Contains(body, `"game":"unknown"`) {
		t.Fatalf("state body = %q (stderr: %s)", body, errw.String())
	}

	cancel()
	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("exit code = %d (stderr: %s)", code, errw.String())
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server did not stop after cancel")
	}
}

func TestServeSeedsStateFromExistingLog(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	logPath := filepath.Join(d.Path, "Logs", "runtime.log")
	line := "[1:00:00] - RUNTIME - IMPORTANT: Screen has been switched to type: `Select`\n"
	if err := os.WriteFile(logPath, []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
	port := freePort(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runInBackground(t, ctx, cancel, []string{"--quaver-dir", d.Path, "--port", fmt.Sprint(port)})

	url := fmt.Sprintf("http://127.0.0.1:%d/state", port)
	deadline := time.Now().Add(3 * time.Second)
	var body string
	for time.Now().Before(deadline) {
		if resp, err := http.Get(url); err == nil {
			buf := new(bytes.Buffer)
			buf.ReadFrom(resp.Body)
			resp.Body.Close()
			body = buf.String()
			if strings.Contains(body, `"game":"select"`) {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("state body = %q", body)
}

func TestServeShowsMapWhenStartedDuringPlay(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	d.AddMap(t, testutil.MapRow{MD5: "m1", Artist: "A", Title: "Long Play", DiffName: "hard", Mode: 1, LastPlayedMS: time.Now().Add(-10 * time.Minute).UnixMilli(), Difficulty: 15.7})
	line := "[1:00:00] - RUNTIME - IMPORTANT: Screen has been switched to type: `Gameplay`\n"
	if err := os.WriteFile(filepath.Join(d.Path, "Logs", "runtime.log"), []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
	port := freePort(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runInBackground(t, ctx, cancel, []string{"--quaver-dir", d.Path, "--port", fmt.Sprint(port)})

	url := fmt.Sprintf("http://127.0.0.1:%d/state", port)
	deadline := time.Now().Add(3 * time.Second)
	var body string
	for time.Now().Before(deadline) {
		if resp, err := http.Get(url); err == nil {
			buf := new(bytes.Buffer)
			buf.ReadFrom(resp.Body)
			resp.Body.Close()
			body = buf.String()
			if strings.Contains(body, `"game":"gameplay"`) && strings.Contains(body, "Long Play") {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("state body = %q", body)
}

func TestRejectsUnexpectedArguments(t *testing.T) {
	for _, args := range [][]string{
		{"doctr"},
		{"--port", "9000", "doctor"},
		{"doctor", "extra"},
		{"serve", "extra"},
	} {
		var out, errw bytes.Buffer
		if code := run(context.Background(), args, &out, &errw); code != 2 {
			t.Fatalf("%v: code = %d", args, code)
		}
		if !strings.Contains(errw.String(), "unexpected argument") {
			t.Fatalf("%v: stderr = %q", args, errw.String())
		}
	}
}

func TestHelpExitsZeroAndUnknownFlagExitsTwo(t *testing.T) {
	var out, errw bytes.Buffer
	if code := run(context.Background(), []string{"-h"}, &out, &errw); code != 0 {
		t.Fatalf("-h: code = %d", code)
	}
	if code := run(context.Background(), []string{"--nope"}, &out, &errw); code != 2 {
		t.Fatalf("--nope: code = %d", code)
	}
}

func TestServeSubcommandHonorsFlags(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	port := freePort(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	var out, errw bytes.Buffer
	go func() {
		done <- run(ctx, []string{"serve", "--quaver-dir", d.Path, "--port", fmt.Sprint(port)}, &out, &errw)
	}()

	url := fmt.Sprintf("http://127.0.0.1:%d/state", port)
	deadline := time.Now().Add(3 * time.Second)
	served := false
	for time.Now().Before(deadline) && !served {
		if resp, err := http.Get(url); err == nil {
			resp.Body.Close()
			served = true
		} else {
			time.Sleep(50 * time.Millisecond)
		}
	}
	if !served {
		t.Fatalf("not serving on port %d (stderr: %s)", port, errw.String())
	}
	cancel()
	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("exit code = %d", code)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server did not stop after cancel")
	}
}

func TestInvalidConfigExitsOneWithPath(t *testing.T) {
	for _, sub := range []string{"serve"} {
		for _, content := range []string{`{"port": `, `{"port": 70000}`, `{"language": "xx"}`} {
			cfg := writeConfig(t, content)
			var out, errw bytes.Buffer
			if code := run(context.Background(), []string{sub, "--config", cfg}, &out, &errw); code != 1 {
				t.Fatalf("%s %s: code = %d", sub, content, code)
			}
			if !strings.Contains(errw.String(), "invalid config "+cfg) {
				t.Fatalf("%s %s: stderr = %q", sub, content, errw.String())
			}
		}
	}
}

func TestDoctorUsesConfigPort(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	port := freePort(t)
	cfg := writeConfig(t, fmt.Sprintf(`{"quaver_dir": "%s", "port": %d}`, jsonPath(d.Path), port))
	var out, errw bytes.Buffer
	if code := run(context.Background(), []string{"doctor", "--config", cfg}, &out, &errw); code != 0 {
		t.Fatalf("code = %d\n%s%s", code, out.String(), errw.String())
	}
	if !strings.Contains(out.String(), fmt.Sprintf("port %d", port)) {
		t.Fatalf("out = %q", out.String())
	}
}

func TestDoctorFlagPortOverridesConfig(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	flagPort := freePort(t)
	cfg := writeConfig(t, fmt.Sprintf(`{"quaver_dir": "%s", "port": %d}`, jsonPath(d.Path), freePort(t)))
	var out, errw bytes.Buffer
	args := []string{"doctor", "--config", cfg, "--port", fmt.Sprint(flagPort)}
	if code := run(context.Background(), args, &out, &errw); code != 0 {
		t.Fatalf("code = %d\n%s", code, out.String())
	}
	if !strings.Contains(out.String(), fmt.Sprintf("port %d", flagPort)) {
		t.Fatalf("out = %q", out.String())
	}
}

func TestDoctorFlagQuaverDirOverridesConfig(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	cfg := writeConfig(t, fmt.Sprintf(`{"quaver_dir": "%s"}`, jsonPath(d.Path)))
	var out, errw bytes.Buffer
	args := []string{"doctor", "--config", cfg, "--quaver-dir", t.TempDir(), "--port", fmt.Sprint(freePort(t))}
	if code := run(context.Background(), args, &out, &errw); code != 1 {
		t.Fatalf("code = %d\n%s", code, out.String())
	}
}

func TestServeUsesConfigPortAndFlagWins(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	cfgPort, flagPort := freePort(t), freePort(t)
	cfg := writeConfig(t, fmt.Sprintf(`{"quaver_dir": "%s", "port": %d}`, jsonPath(d.Path), cfgPort))

	for _, c := range []struct {
		args []string
		port int
	}{
		{[]string{"--config", cfg}, cfgPort},
		{[]string{"--config", cfg, "--port", fmt.Sprint(flagPort)}, flagPort},
	} {
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan int, 1)
		var out, errw bytes.Buffer
		go func() { done <- run(ctx, c.args, &out, &errw) }()
		url := fmt.Sprintf("http://127.0.0.1:%d/state", c.port)
		served := false
		for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline) && !served; {
			if resp, err := http.Get(url); err == nil {
				resp.Body.Close()
				served = true
			} else {
				time.Sleep(50 * time.Millisecond)
			}
		}
		cancel()
		<-done
		if !served {
			t.Fatalf("%v: not serving on %d (stderr: %s)", c.args, c.port, errw.String())
		}
	}
}

func TestStaleSavedFolderDoesNotBlockFlagOrDoctor(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	stale := filepath.Join(t.TempDir(), "moved")
	cfg := writeConfig(t, fmt.Sprintf(`{"quaver_dir": "%s"}`, jsonPath(stale)))

	var out, errw bytes.Buffer
	args := []string{"doctor", "--config", cfg, "--quaver-dir", d.Path, "--port", fmt.Sprint(freePort(t))}
	if code := run(context.Background(), args, &out, &errw); code != 0 {
		t.Fatalf("flag must win: code = %d\n%s%s", code, out.String(), errw.String())
	}

	out.Reset()
	errw.Reset()
	args = []string{"doctor", "--config", cfg, "--port", fmt.Sprint(freePort(t))}
	if code := run(context.Background(), args, &out, &errw); code != 1 {
		t.Fatalf("code = %d", code)
	}
	if strings.Contains(errw.String(), "invalid config") || !strings.Contains(out.String(), "[FAIL] Quaver folder") {
		t.Fatalf("stdout = %q, stderr = %q", out.String(), errw.String())
	}
}

func TestUnreadableConfigIsNotReportedAsInvalid(t *testing.T) {
	var out, errw bytes.Buffer
	if code := run(context.Background(), []string{"doctor", "--config", t.TempDir()}, &out, &errw); code != 1 {
		t.Fatalf("code = %d", code)
	}
	if !strings.Contains(out.String(), "cannot read config") || strings.Contains(out.String(), "invalid config") {
		t.Fatalf("stderr = %q", errw.String())
	}
}

func TestMissingDefaultThemeNeverBlocksStartup(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	cfg := writeConfig(t, `{"overlay": {"theme": "gone-theme"}}`)
	var errw bytes.Buffer
	o, code := parseFlags("serve", true, []string{"--config", cfg}, &errw)
	if code != -1 || o.configPath != cfg {
		t.Fatalf("code = %d, stderr = %q", code, errw.String())
	}
	want := "warning: default theme \"gone-theme\" not found, using neutral-light\n"
	if errw.String() != want {
		t.Fatalf("stderr = %q", errw.String())
	}
	var out bytes.Buffer
	errw.Reset()
	args := []string{"doctor", "--config", cfg, "--quaver-dir", d.Path, "--port", fmt.Sprint(freePort(t))}
	if code := run(context.Background(), args, &out, &errw); code != 0 || !strings.Contains(errw.String(), "gone-theme") {
		t.Fatalf("doctor: code = %d\n%s%s", code, out.String(), errw.String())
	}
}

func TestSyntaxInvalidDefaultThemeStillFails(t *testing.T) {
	for _, id := range []string{"../x", "UP", "a b"} {
		cfg := writeConfig(t, fmt.Sprintf(`{"overlay": {"theme": %q}}`, id))
		var errw bytes.Buffer
		if _, code := parseFlags("serve", true, []string{"--config", cfg}, &errw); code != 1 || !strings.Contains(errw.String(), "invalid config") {
			t.Errorf("%q: code = %d, stderr = %q", id, code, errw.String())
		}
	}
}

func TestStartupAcceptsCustomThemeFromConfigFolder(t *testing.T) {
	cfg := writeConfig(t, `{"overlay": {"theme": "mine"}}`)
	themesDir := filepath.Join(filepath.Dir(cfg), "themes")
	if err := os.Mkdir(themesDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(themesDir, "mine.css"), []byte(":root{--bg:#123}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(themes.IDs(availableThemes(cfg)), "mine") {
		t.Fatal("availableThemes does not list the custom theme")
	}
	var errw bytes.Buffer
	if _, code := parseFlags("serve", true, []string{"--config", cfg}, &errw); code != -1 || errw.Len() != 0 {
		t.Fatalf("code = %d, stderr = %q", code, errw.String())
	}
}

func TestServeSeedsLastScoreAndMapAfterRestart(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	d.AddMap(t, testutil.MapRow{MD5: "m1", Artist: "A", Title: "Played Before", DiffName: "hard", Mode: 1, LastPlayedMS: time.Now().Add(-2 * time.Hour).UnixMilli(), Difficulty: 15.7})
	d.AddScore(t, testutil.ScoreRow{MapMD5: "m1", Total: 900000, Grade: 7, Accuracy: 98.2, MaxCombo: 300})
	line := "[1:00:00] - RUNTIME - IMPORTANT: Screen has been switched to type: `Results`\n"
	if err := os.WriteFile(filepath.Join(d.Path, "Logs", "runtime.log"), []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
	port := freePort(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runInBackground(t, ctx, cancel, []string{"--quaver-dir", d.Path, "--port", fmt.Sprint(port)})

	url := fmt.Sprintf("http://127.0.0.1:%d/state", port)
	deadline := time.Now().Add(3 * time.Second)
	var body string
	for time.Now().Before(deadline) {
		if resp, err := http.Get(url); err == nil {
			buf := new(bytes.Buffer)
			buf.ReadFrom(resp.Body)
			resp.Body.Close()
			body = buf.String()
			if strings.Contains(body, `"game":"results"`) && strings.Contains(body, "Played Before") && strings.Contains(body, `"accuracy":98.2`) {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("state body = %q", body)
}
