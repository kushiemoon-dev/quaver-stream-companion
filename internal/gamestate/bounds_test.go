package gamestate

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestTailDropsAnOversizedLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runtime.log")
	appendFile(t, path, "start\n")
	ch := startTail(t, path)

	appendFile(t, path, strings.Repeat("x", 3<<20))
	time.Sleep(100 * time.Millisecond)
	appendFile(t, path, "tail of the long line\nok\n")

	if got := recv(t, ch); got != "ok" {
		t.Fatalf("got a line of %d bytes, want the line after the oversized one", len(got))
	}
}

func TestTailStreamsALargeAppendWithASmallWorkingSet(t *testing.T) {
	if testing.Short() {
		t.Skip("writes 50 MiB")
	}
	path := filepath.Join(t.TempDir(), "runtime.log")
	appendFile(t, path, "start\n")
	ch := startTail(t, path)

	line := "[1:06:06] - RUNTIME - IMPORTANT: " + strings.Repeat("a", 90) + "\n"
	block := strings.Repeat(line, 8192)
	count := int(50 << 20 / len(block))
	want := count * 8192

	runtime.GC()
	var base runtime.MemStats
	runtime.ReadMemStats(&base)
	var peak atomic.Uint64
	stop := make(chan struct{})
	go func() {
		var ms runtime.MemStats
		for {
			select {
			case <-stop:
				return
			case <-time.After(2 * time.Millisecond):
			}
			runtime.ReadMemStats(&ms)
			if ms.HeapInuse > peak.Load() {
				peak.Store(ms.HeapInuse)
			}
		}
	}()
	defer close(stop)

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	for range count {
		if _, err := f.WriteString(block); err != nil {
			t.Fatal(err)
		}
	}
	f.Close()

	got := 0
	deadline := time.After(60 * time.Second)
	for got < want {
		select {
		case <-ch:
			got++
		case <-deadline:
			t.Fatalf("received %d of %d lines", got, want)
		}
	}
	if grown := int64(peak.Load()) - int64(base.HeapInuse); grown > 24<<20 {
		t.Fatalf("heap grew by %d MiB while reading 50 MiB", grown>>20)
	}
}

func TestLastStateLineReadsOnlyTheEndOfALargeLog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runtime.log")
	filler := bytes.Repeat([]byte("[1:00:00] - RUNTIME - IMPORTANT: nothing to see here, just filler text\n"), 1<<20/70)
	var data []byte
	data = append(data, []byte(seedSelect+"\n")...)
	for range 40 {
		data = append(data, filler...)
	}
	data = append(data, []byte(seedMenu+"\n")...)
	data = append(data, filler[:2000]...)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	line, ok := LastStateLine(path, time.Hour)
	runtime.ReadMemStats(&after)

	if !ok || line != seedMenu {
		t.Fatalf("line = %q, ok = %v", line, ok)
	}
	if used := after.TotalAlloc - before.TotalAlloc; used > 2<<20 {
		t.Fatalf("seeding allocated %d MiB for a %d MiB log", used>>20, len(data)>>20)
	}
}

func TestLastStateLineIgnoresStateLinesBeyondTheLast64KiB(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runtime.log")
	data := []byte(seedSelect + "\n")
	data = append(data, bytes.Repeat([]byte("[1:00:00] - RUNTIME - IMPORTANT: filler filler filler filler filler\n"), 3000)...)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if line, ok := LastStateLine(path, time.Hour); ok {
		t.Fatalf("found %q beyond the window", line)
	}
}

func TestTailDetectsAReplacedFileWithTheSameHead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "runtime.log")
	head := "[1:00:00] - RUNTIME - IMPORTANT: Logger initialized for session number 1234567890\n"
	appendFile(t, path, head+"old one\n")
	old := handleStat(t, path)
	ch := startTail(t, path)

	next := filepath.Join(dir, "next.log")
	appendFile(t, next, head+"first of the new session\nsecond of the new session, grown past the old offset\n")
	if err := os.Rename(next, path); err != nil {
		t.Skipf("cannot replace the open log on this system: %v", err)
	}
	if os.SameFile(old, handleStat(t, path)) {
		t.Skip("this file system reused the file identity")
	}

	if got := recv(t, ch); got != strings.TrimSuffix(head, "\n") {
		t.Fatalf("got %q, want the new session from its first line", got)
	}
}

// handleStat stats an open handle: on Windows only that gives a file identity that stays valid.
func handleStat(t *testing.T, path string) os.FileInfo {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	return fi
}

func TestLastStateLineKeepsALineThatStartsExactlyAtTheWindow(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runtime.log")
	data := bytes.Repeat([]byte("[1:00:00] - RUNTIME - IMPORTANT: prefix\n"), 100)
	data = append(data, []byte(seedMenu+"\n")...)
	pad := seedWindow - len(seedMenu) - 1
	data = append(data, bytes.Repeat([]byte("x"), pad-1)...)
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	line, ok := LastStateLine(path, time.Hour)
	if !ok || line != seedMenu {
		t.Fatalf("line = %q, ok = %v", line, ok)
	}
}
