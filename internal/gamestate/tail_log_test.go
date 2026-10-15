package gamestate

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
)

type logSink struct {
	mu   sync.Mutex
	recs []slog.Record
}

func (s *logSink) Enabled(context.Context, slog.Level) bool { return true }
func (s *logSink) WithAttrs([]slog.Attr) slog.Handler       { return s }
func (s *logSink) WithGroup(string) slog.Handler            { return s }
func (s *logSink) Handle(_ context.Context, r slog.Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recs = append(s.recs, r.Clone())
	return nil
}

// withMessage returns the records logged with msg so far.
func (s *logSink) withMessage(msg string) []slog.Record {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []slog.Record
	for _, r := range s.recs {
		if r.Message == msg {
			out = append(out, r)
		}
	}
	return out
}

func captureLogs(t *testing.T) *logSink {
	t.Helper()
	sink := &logSink{}
	old := slog.Default()
	slog.SetDefault(slog.New(sink))
	t.Cleanup(func() { slog.SetDefault(old) })
	return sink
}

func attrInt(r slog.Record, key string) int64 {
	var v int64 = -1
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == key {
			v = a.Value.Int64()
		}
		return true
	})
	return v
}

const missingMsg = "runtime.log is missing; waiting for it"

func newTestTailer(path string) *tailer {
	return &tailer{path: path, offset: -1, chunk: make([]byte, chunkSize)}
}

func TestMissingLogIsAnnouncedOnceUntilItAppears(t *testing.T) {
	sink := captureLogs(t)
	tl := newTestTailer(filepath.Join(t.TempDir(), "runtime.log"))
	tl.step()
	if n := len(sink.withMessage(missingMsg)); n != 1 {
		t.Fatalf("first step logged %d times, want 1", n)
	}
	tl.step()
	if n := len(sink.withMessage(missingMsg)); n != 1 {
		t.Fatalf("second step logged %d times, want still 1", n)
	}
}

func TestLogRemovedAfterBeingFollowedIsAnnouncedAgain(t *testing.T) {
	sink := captureLogs(t)
	path := filepath.Join(t.TempDir(), "runtime.log")
	tl := newTestTailer(path)
	tl.step()
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	tl.step()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	tl.step()
	if n := len(sink.withMessage(missingMsg)); n != 2 {
		t.Fatalf("logged %d times, want 2", n)
	}
}

func TestOversizedLineIsReportedWithItsLength(t *testing.T) {
	const dropped = "dropped an oversized line from runtime.log"
	sink := captureLogs(t)
	tl := &tailer{}
	whole := bytes.Repeat([]byte("w"), maxPending+7)
	tl.feed(nil, append(whole, '\n'))
	tl = &tailer{}
	half := bytes.Repeat([]byte("h"), maxPending/2+3)
	tl.feed(nil, half)
	tl.feed(nil, half)
	var sizes []int64
	for _, r := range sink.withMessage(dropped) {
		sizes = append(sizes, attrInt(r, "bytes"))
	}
	for _, want := range []int64{maxPending + 7, int64(len(half) * 2)} {
		if !slices.Contains(sizes, want) {
			t.Errorf("no report with %d bytes in %v", want, sizes)
		}
	}
}

func TestLinesAfterAnOversizedLineInTheSameChunkAreKept(t *testing.T) {
	over := bytes.Repeat([]byte("o"), maxPending+1)
	data := append(append(over, '\n'), []byte("one\ntwo\n")...)
	var got []string
	got = (&tailer{}).feed(got, data)
	if !slices.Equal(got, []string{"one", "two"}) {
		t.Fatalf("got %q", got)
	}
}

func TestStepReportsMoreOnlyWhenBytesRemainAfterTheBatch(t *testing.T) {
	for _, tc := range []struct {
		extra    int
		wantMore bool
	}{{0, false}, {1, true}} {
		path := filepath.Join(t.TempDir(), "runtime.log")
		if err := os.WriteFile(path, bytes.Repeat([]byte("x"), batchChunks*chunkSize+tc.extra), 0o644); err != nil {
			t.Fatal(err)
		}
		tl := &tailer{path: path, offset: -1, seenMissing: true, chunk: make([]byte, chunkSize)}
		if _, more := tl.step(); more != tc.wantMore {
			t.Errorf("extra %d: more = %v, want %v", tc.extra, more, tc.wantMore)
		}
	}
}
