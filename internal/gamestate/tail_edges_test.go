package gamestate

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// A line of exactly maxPending bytes is kept, one byte more is dropped, whether it arrives whole
// or with its newline in a later chunk.
func TestFeedMaxPendingBoundary(t *testing.T) {
	exact := bytes.Repeat([]byte("x"), maxPending)
	over := bytes.Repeat([]byte("y"), maxPending+1)
	for _, tc := range []struct {
		name   string
		chunks [][]byte
		want   []string
	}{
		{"exact whole", [][]byte{append(slices.Clone(exact), '\n')}, []string{string(exact)}},
		{"exact then newline", [][]byte{exact, []byte("\nok\n")}, []string{string(exact), "ok"}},
		{"over whole", [][]byte{append(slices.Clone(over), '\n'), []byte("ok\n")}, []string{"ok"}},
		{"over then newline", [][]byte{over, []byte("\nok\n")}, []string{"ok"}},
	} {
		tl := &tailer{}
		var got []string
		for _, c := range tc.chunks {
			got = tl.feed(got, c)
		}
		if !slices.Equal(got, tc.want) {
			t.Errorf("%s: got %d lines, want %d", tc.name, len(got), len(tc.want))
		}
	}
}

// Two halves that each fit but together exceed maxPending are dropped, and the held part never
// grows past maxPending.
func TestFeedSplitOversizedLineStaysBounded(t *testing.T) {
	half := bytes.Repeat([]byte("z"), maxPending/2+10)
	tl := &tailer{}
	var got []string
	got = tl.feed(got, half)
	got = tl.feed(got, half)
	if len(tl.pending) > maxPending {
		t.Fatalf("pending grew to %d bytes", len(tl.pending))
	}
	got = tl.feed(got, []byte("tail\nnext\n"))
	if !slices.Equal(got, []string{"next"}) {
		t.Fatalf("got %d lines, want only %q", len(got), "next")
	}
}

// An empty line, also when a chunk starts with its newline, is delivered as "".
func TestFeedEmptyLineAtChunkStart(t *testing.T) {
	tl := &tailer{}
	var got []string
	got = tl.feed(got, []byte("a\r"))
	got = tl.feed(got, []byte("\n\nb\n"))
	if !slices.Equal(got, []string{"a", "", "b"}) {
		t.Fatalf("got %q", got)
	}
}

// A log that exists but is empty at startup is followed from offset 0: its first line is not skipped.
func TestStepReadsFirstLineOfAnInitiallyEmptyLog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runtime.log")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	tl := &tailer{path: path, offset: -1, chunk: make([]byte, chunkSize)}
	if lines, _ := tl.step(); len(lines) != 0 {
		t.Fatalf("baseline returned %q", lines)
	}
	if err := os.WriteFile(path, []byte("first\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if lines, _ := tl.step(); !slices.Equal(lines, []string{"first"}) {
		t.Fatalf("got %q, want [first]", lines)
	}
}

// step reads at most batchChunks chunks and reports that more data is waiting.
func TestStepReportsMoreAfterABatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runtime.log")
	line := append(bytes.Repeat([]byte("l"), 1023), '\n')
	data := bytes.Repeat(line, (batchChunks*chunkSize)/len(line)+64)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	tl := &tailer{path: path, offset: -1, seenMissing: true, chunk: make([]byte, chunkSize)}
	lines, more := tl.step()
	if !more || len(lines) == 0 || tl.offset != int64(batchChunks*chunkSize) {
		t.Fatalf("first step: %d lines, more=%v, offset=%d", len(lines), more, tl.offset)
	}
	rest, more := tl.step()
	if more || len(lines)+len(rest) != len(data)/len(line) {
		t.Fatalf("second step: %d lines, more=%v", len(rest), more)
	}
}
