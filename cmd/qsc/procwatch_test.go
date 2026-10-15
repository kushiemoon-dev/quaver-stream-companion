package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/procwatch"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/testutil"
)

const menuLine = "[1:05:29] - RUNTIME - IMPORTANT: Screen has been switched to type: `Menu`"

type countingLister struct {
	calls atomic.Int64
	names []string
}

func (c *countingLister) Names() ([]string, error) { c.calls.Add(1); return c.names, nil }

func gameState(port int) string {
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/state", port))
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	var s struct {
		Game string `json:"game"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&s)
	return s.Game
}

func waitGame(t *testing.T, port int, want string) {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if gameState(port) == want {
			return
		}
	}
	t.Fatalf("game state never became %q (now %q)", want, gameState(port))
}

func startWatched(t *testing.T, cfgJSON string, l procwatch.Lister) (port int, cfg string) {
	t.Helper()
	t.Cleanup(processLister.set(func() (procwatch.Lister, bool) { return l, true }))
	t.Cleanup(processInterval.set(2 * time.Millisecond))
	d := testutil.NewQuaverDir(t)
	d.AppendLog(t, menuLine)
	port = freePort(t)
	cfg = filepath.Join(t.TempDir(), "config.json")
	if cfgJSON != "" {
		if err := os.WriteFile(cfg, []byte(cfgJSON), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	var out, errw bytes.Buffer
	go func() {
		done <- run(ctx, []string{"--quaver-dir", d.Path, "--port", fmt.Sprint(port), "--config", cfg}, &out, &errw)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Error("serve did not exit")
		}
	})
	return port, cfg
}

func TestServeReportsClosedWhenNoQuaverProcessExists(t *testing.T) {
	l := &countingLister{names: []string{"explorer.exe"}}
	port, _ := startWatched(t, "", l)
	waitGame(t, port, "closed")
	if l.calls.Load() < 3 {
		t.Fatalf("lister called %d times", l.calls.Load())
	}
}

func TestServeKeepsTheStateWhileQuaverRuns(t *testing.T) {
	l := &countingLister{names: []string{"Quaver.exe"}}
	port, _ := startWatched(t, "", l)
	waitGame(t, port, "menu")
	for deadline := time.Now().Add(10 * time.Second); l.calls.Load() < 10; {
		if time.Now().After(deadline) {
			t.Fatal("the watcher never ran")
		}
		time.Sleep(time.Millisecond)
	}
	if got := gameState(port); got != "menu" {
		t.Fatalf("state = %q", got)
	}
}

func TestProcessListerIsNotTheRealOneInTests(t *testing.T) {
	l, ok := processLister.get()()
	if ok || reflect.TypeOf(l) != nil {
		t.Fatal("cmd tests must not list the real processes")
	}
}
