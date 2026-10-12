package gamestate

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"time"
)

// headLen is how many leading bytes we remember to detect a rewritten file.
const headLen = 64

type tailer struct {
	path        string
	offset      int64
	pending     []byte
	prevHead    []byte
	seenMissing bool
}

// Tail sends every complete line appended to path on out until ctx is done.
// Quaver truncates and rewrites runtime.log at each launch, so a file that
// shrinks or whose first bytes change is read again from the start.
func Tail(ctx context.Context, path string, poll time.Duration, out chan<- string) {
	t := &tailer{path: path, offset: -1}
	ticker := time.NewTicker(poll)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		for _, line := range t.step() {
			select {
			case out <- line:
			case <-ctx.Done():
				return
			}
		}
	}
}

func (t *tailer) step() []string {
	f, err := os.Open(t.path)
	if err != nil {
		t.offset, t.pending, t.prevHead, t.seenMissing = -1, nil, nil, true
		return nil
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil
	}
	size := fi.Size()
	head := make([]byte, min(size, headLen))
	if _, err := io.ReadFull(f, head); err != nil {
		return nil
	}

	if t.offset < 0 {
		if !t.seenMissing {
			// File already existed when we started: only follow new content.
			t.offset, t.prevHead = size, head
			return nil
		}
		t.offset = 0
	}
	if size < t.offset || !bytes.HasPrefix(head, t.prevHead) {
		t.offset, t.pending = 0, nil
	}
	t.prevHead = head
	if size == t.offset {
		return nil
	}

	if _, err := f.Seek(t.offset, io.SeekStart); err != nil {
		return nil
	}
	buf := make([]byte, size-t.offset)
	n, _ := io.ReadFull(f, buf)
	t.offset += int64(n)
	t.pending = append(t.pending, buf[:n]...)
	return t.drain()
}

func (t *tailer) drain() []string {
	var lines []string
	for {
		i := bytes.IndexByte(t.pending, '\n')
		if i < 0 {
			return lines
		}
		lines = append(lines, strings.TrimRight(string(t.pending[:i]), "\r"))
		t.pending = t.pending[i+1:]
	}
}
