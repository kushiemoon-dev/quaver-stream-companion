package gamestate

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func startTail(t *testing.T, path string) <-chan string {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan string, 16)
	done := make(chan struct{})
	t.Cleanup(func() { cancel(); <-done })
	go func() { defer close(done); Tail(ctx, path, 10*time.Millisecond, ch) }()
	time.Sleep(60 * time.Millisecond) // let Tail take its baseline
	return ch
}

func recv(t *testing.T, ch <-chan string) string {
	t.Helper()
	select {
	case s := <-ch:
		return s
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for a line")
		return ""
	}
}

func appendFile(t *testing.T, path, s string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(s); err != nil {
		t.Fatal(err)
	}
}

func TestTailSkipsExistingContentAndReadsNew(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runtime.log")
	appendFile(t, path, "old\n")
	ch := startTail(t, path)
	appendFile(t, path, "new\n")
	if got := recv(t, ch); got != "new" {
		t.Fatalf("got %q", got)
	}
}

func TestTailJoinsPartialLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runtime.log")
	appendFile(t, path, "start\n")
	ch := startTail(t, path)
	appendFile(t, path, "par")
	time.Sleep(60 * time.Millisecond)
	appendFile(t, path, "tial\r\n")
	if got := recv(t, ch); got != "partial" {
		t.Fatalf("got %q", got)
	}
}

func TestTailRestartsOnTruncate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runtime.log")
	appendFile(t, path, "a long old line that is quite long\n")
	ch := startTail(t, path)
	if err := os.WriteFile(path, []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := recv(t, ch); got != "new" {
		t.Fatalf("got %q", got)
	}
}

func TestTailDetectsRotationWithLargerContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runtime.log")
	appendFile(t, path, "[1:00:00] old session\n")
	ch := startTail(t, path)
	fresh := "[1:05:02] new session first line, much longer than the old content\nsecond\n"
	if err := os.WriteFile(path, []byte(fresh), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := recv(t, ch); got != "[1:05:02] new session first line, much longer than the old content" {
		t.Fatalf("first = %q", got)
	}
	if got := recv(t, ch); got != "second" {
		t.Fatalf("second = %q", got)
	}
}

func TestTailReadsFromStartWhenFileAppearsLater(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runtime.log")
	ch := startTail(t, path) // file does not exist yet
	appendFile(t, path, "first\n")
	if got := recv(t, ch); got != "first" {
		t.Fatalf("got %q", got)
	}
}
