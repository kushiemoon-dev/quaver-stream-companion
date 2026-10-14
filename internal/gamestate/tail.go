package gamestate

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"
)

const (
	// headLen is how many leading bytes we remember to detect a rewritten file.
	headLen = 64
	// chunkSize is the size of each read from the log.
	chunkSize = 64 << 10
	// maxPending caps a line still waiting for its newline; a longer one is dropped.
	maxPending = 1 << 20
	// batchChunks bounds how much one step reads before its lines are handed on.
	batchChunks = 16
)

type tailer struct {
	path        string
	offset      int64
	pending     []byte
	skipping    bool
	prevHead    []byte
	prevInfo    os.FileInfo
	seenMissing bool
	chunk       []byte
}

// Tail sends every complete line appended to path on out until ctx is done.
// Quaver truncates and rewrites runtime.log at each launch, so a file that
// shrinks, whose first bytes change or that is replaced by another file is read again from the start.
func Tail(ctx context.Context, path string, poll time.Duration, out chan<- string) {
	t := &tailer{path: path, offset: -1, chunk: make([]byte, chunkSize)}
	ticker := time.NewTicker(poll)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		for more := true; more; {
			var lines []string
			lines, more = t.step()
			for _, line := range lines {
				select {
				case out <- line:
				case <-ctx.Done():
					return
				}
			}
		}
	}
}

func (t *tailer) reset() {
	t.pending, t.skipping = nil, false
}

// step reads what was appended since the last call, at most batchChunks chunks; more tells
// that the file holds further unread data.
func (t *tailer) step() (lines []string, more bool) {
	f, err := os.Open(t.path)
	if err != nil {
		if !t.seenMissing || t.offset >= 0 {
			slog.Info("runtime.log is missing; waiting for it")
		}
		t.offset, t.prevHead, t.prevInfo, t.seenMissing = -1, nil, nil, true
		t.reset()
		return nil, false
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, false
	}
	size := fi.Size()
	head := make([]byte, min(size, headLen))
	if _, err := io.ReadFull(f, head); err != nil {
		return nil, false
	}

	if t.offset < 0 {
		if !t.seenMissing {
			// File already existed when we started: only follow new content.
			t.offset, t.prevHead, t.prevInfo = size, head, fi
			return nil, false
		}
		slog.Info("runtime.log found; reading it from the start")
		t.offset = 0
	}
	if size < t.offset || !bytes.HasPrefix(head, t.prevHead) || (t.prevInfo != nil && !os.SameFile(t.prevInfo, fi)) {
		slog.Info("runtime.log was rewritten; reading it from the start")
		t.offset = 0
		t.reset()
	}
	t.prevHead, t.prevInfo = head, fi
	if size == t.offset {
		return nil, false
	}

	if _, err := f.Seek(t.offset, io.SeekStart); err != nil {
		return nil, false
	}
	for range batchChunks {
		n, err := f.Read(t.chunk)
		t.offset += int64(n)
		lines = t.feed(lines, t.chunk[:n])
		if err != nil || n == 0 {
			return lines, false
		}
	}
	return lines, t.offset < size
}

// feed splits data into lines, joining them with the partial line kept from before.
func (t *tailer) feed(lines []string, data []byte) []string {
	for len(data) > 0 {
		i := bytes.IndexByte(data, '\n')
		if i < 0 {
			t.hold(data)
			return lines
		}
		seg := data[:i]
		data = data[i+1:]
		if t.skipping {
			t.skipping = false
			continue
		}
		if len(t.pending)+len(seg) > maxPending {
			slog.Warn("dropped an oversized line from runtime.log", "bytes", len(t.pending)+len(seg))
			t.pending = nil
			continue
		}
		line := seg
		if len(t.pending) > 0 {
			line = append(t.pending, seg...)
		}
		lines = append(lines, strings.TrimRight(string(line), "\r"))
		t.pending = nil
	}
	return lines
}

// hold keeps the start of a line whose end has not been written yet.
func (t *tailer) hold(data []byte) {
	if t.skipping {
		return
	}
	if len(t.pending)+len(data) > maxPending {
		slog.Warn("dropped an oversized line from runtime.log", "bytes", len(t.pending)+len(data))
		t.pending, t.skipping = nil, true
		return
	}
	t.pending = append(t.pending, data...)
}
