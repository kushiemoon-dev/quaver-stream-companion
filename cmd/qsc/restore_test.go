package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/testutil"
)

func waitStateContains(t *testing.T, port int, want string) {
	t.Helper()
	url := fmt.Sprintf("http://127.0.0.1:%d/state", port)
	for deadline := time.Now().Add(4 * time.Second); time.Now().Before(deadline); time.Sleep(30 * time.Millisecond) {
		if resp, err := http.Get(url); err == nil {
			var buf bytes.Buffer
			buf.ReadFrom(resp.Body)
			resp.Body.Close()
			if strings.Contains(buf.String(), want) {
				return
			}
		}
	}
	t.Fatalf("state never contained %q", want)
}

// TestSupervisorTickForgetsAnnouncedScoresAfterARestore proves the serve wiring: after the
// database was rolled back to older ids, an id that comes back is announced again, which only
// happens when the periodic check cleared the announced set.
func TestSupervisorTickForgetsAnnouncedScoresAfterARestore(t *testing.T) {
	old := recheckInterval
	recheckInterval = 40 * time.Millisecond
	t.Cleanup(func() { recheckInterval = old })
	d := testutil.NewQuaverDir(t)
	d.AddMap(t, testutil.MapRow{MD5: "m1", Title: "Song", Mode: 1, LastPlayedMS: time.Now().Add(-time.Hour).UnixMilli()})
	play := testutil.ScoreRow{MapMD5: "m1", Total: 900000, Grade: 7, Accuracy: 98.1, MaxCombo: 300, Marv: 200}
	for range 7 {
		d.AddScore(t, play)
	}
	port := freePort(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cfg := filepath.Join(t.TempDir(), "config.json")
	var out, errw bytes.Buffer
	go run(ctx, []string{"--quaver-dir", d.Path, "--port", fmt.Sprint(port), "--config", cfg}, &out, &errw)
	waitStateContains(t, port, `"id":7`)

	var announced atomic.Int32
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/events", port))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	go func() {
		sc := bufio.NewScanner(resp.Body)
		for sc.Scan() {
			if sc.Text() == "event: score.saved" {
				announced.Add(1)
			}
		}
	}()

	d.Exec(t, `DELETE FROM Score WHERE Id > 3`)
	d.Exec(t, `UPDATE sqlite_sequence SET seq = 3 WHERE name = 'Score'`)
	time.Sleep(600 * time.Millisecond)
	for range 4 {
		d.AddScore(t, play)
	}
	d.AppendLog(t, "[1:06:06] - RUNTIME - IMPORTANT: Successfully saved score to database with id: 7")

	for deadline := time.Now().Add(4 * time.Second); announced.Load() == 0 && time.Now().Before(deadline); time.Sleep(30 * time.Millisecond) {
	}
	if announced.Load() != 1 {
		t.Fatalf("score.saved events = %d, want 1: the restored id 7 must be announced again", announced.Load())
	}
}

func TestServeLogRedactsTheHomeFolder(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	home := filepath.Dir(d.Path)
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)
	port := freePort(t)
	ctx, cancel := context.WithCancel(context.Background())
	cfg := filepath.Join(t.TempDir(), "config.json")
	var out, errw bytes.Buffer
	done := make(chan int, 1)
	go func() {
		done <- run(ctx, []string{"--quaver-dir", d.Path, "--port", fmt.Sprint(port), "--config", cfg}, &out, &errw)
	}()
	waitStateContains(t, port, `"game"`)
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not stop")
	}

	log := readLog(t, filepath.Join(filepath.Dir(cfg), "qsc.log"))
	if !strings.Contains(log, "Quaver folder found") {
		t.Fatalf("log:\n%s", log)
	}
	if strings.Contains(strings.ToLower(log), strings.ToLower(home)) || strings.Contains(strings.ToLower(errw.String()), strings.ToLower(home)) {
		t.Fatalf("the home folder %q leaks into the log:\n%s\n%s", home, log, errw.String())
	}
	if !strings.Contains(log, "dir=~") {
		t.Fatalf("log:\n%s", log)
	}
}
