package app

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestWatchStartsImmediatelyWhenReady(t *testing.T) {
	started := make(chan string, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		Watch(context.Background(), time.Hour,
			func() (string, error) { return "/q", nil },
			func(dir string) error { started <- dir; return nil })
	}()
	select {
	case dir := <-started:
		if dir != "/q" {
			t.Fatalf("dir = %q", dir)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("not started")
	}
	waitClosed(t, done)
}

func TestWatchRechecksUntilFoundThenStopsChecking(t *testing.T) {
	var finds atomic.Int32
	started := make(chan string, 4)
	done := make(chan struct{})
	go func() {
		defer close(done)
		Watch(context.Background(), 5*time.Millisecond,
			func() (string, error) {
				if finds.Add(1) < 3 {
					return "", errors.New("missing")
				}
				return "/q", nil
			},
			func(dir string) error { started <- dir; return nil })
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("never started")
	}
	waitClosed(t, done)
	n := finds.Load()
	time.Sleep(40 * time.Millisecond)
	if finds.Load() != n || n != 3 || len(started) != 0 {
		t.Fatalf("finds = %d (then %d), extra starts = %d", n, finds.Load(), len(started))
	}
}

func TestWatchRetriesWhenStartFails(t *testing.T) {
	var starts atomic.Int32
	done := make(chan struct{})
	go func() {
		defer close(done)
		Watch(context.Background(), 5*time.Millisecond,
			func() (string, error) { return "/q", nil },
			func(string) error {
				if starts.Add(1) < 2 {
					return errors.New("db locked")
				}
				return nil
			})
	}()
	waitClosed(t, done)
	if starts.Load() != 2 {
		t.Fatalf("starts = %d", starts.Load())
	}
}

func TestWatchStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		Watch(ctx, time.Hour,
			func() (string, error) { return "", errors.New("missing") },
			func(string) error { t.Error("started"); return nil })
	}()
	cancel()
	waitClosed(t, done)
}

func waitClosed(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatal("did not return")
	}
}
