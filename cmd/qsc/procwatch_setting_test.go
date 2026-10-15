package main

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestWatchEnabledReadsTheFileEachTime(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.json")
	on := watchEnabled(cfg)
	if !on() {
		t.Fatal("a missing file means on")
	}
	write := func(s string) {
		if err := os.WriteFile(cfg, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(`{"watch_quaver_process": false}`)
	if on() {
		t.Fatal("false in the file must give false")
	}
	write(`{"port": 7878}`)
	if !on() {
		t.Fatal("a file without the key means on")
	}
	write(`{"watch_quaver_process": true}`)
	if !on() {
		t.Fatal("true in the file must give true")
	}
	write(`{not json`)
	if on() {
		t.Fatal("an unreadable file must give false")
	}
	write(`{"watch_quaver_process": true}`)
	if !on() {
		t.Fatal("a readable file after a broken one must give true again")
	}
	if watchEnabled(dir)() {
		t.Fatal("a path that cannot be read as a file must give false")
	}
}

func TestProcessLookupFollowsTheSettingWithoutRestart(t *testing.T) {
	var offTicks, onTicks atomic.Int64
	offSeen := make(chan struct{}, 1)
	t.Cleanup(processTick.set(func(on bool) {
		if on {
			onTicks.Add(1)
			return
		}
		if offTicks.Add(1) == 25 {
			offSeen <- struct{}{}
		}
	}))
	l := &countingLister{}
	port, cfg := startWatched(t, `{"watch_quaver_process": false}`, l)
	waitGame(t, port, "menu")
	select {
	case <-offSeen:
	case <-time.After(10 * time.Second):
		t.Fatal("the checks never ran")
	}
	if n := l.calls.Load(); n != 0 || onTicks.Load() != 0 {
		t.Fatalf("after %d checks with the setting off: listed %d times, %d checks saw it on", offTicks.Load(), n, onTicks.Load())
	}
	if got := gameState(port); got != "menu" {
		t.Fatalf("state = %q with the setting off", got)
	}
	if err := os.WriteFile(cfg, []byte(`{"watch_quaver_process": true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	waitGame(t, port, "closed")
}
