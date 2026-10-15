package gamestate

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
)

var wordRe = regexp.MustCompile(`^\w+$`)

func FuzzParseLine(f *testing.F) {
	for _, s := range []string{
		"Screen has been switched to type: `Gameplay`",
		"Successfully saved score to database with id: 42",
		"Successfully saved score to database with id: 99999999999999999999999",
		"Successfully saved replay to path: C:\\r\\x.qr",
		"[INFO] Discord RPC has shutdown",
		"", "\r", "Screen has been switched to type: ``",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		ln, ok := ParseLine(s)
		if !ok {
			if ln != (Line{}) {
				t.Fatalf("not ok but line %+v", ln)
			}
			return
		}
		switch ln.Kind {
		case KindState:
			if !wordRe.MatchString(ln.State) || ln.State != strings.ToLower(ln.State) {
				t.Fatalf("state %q", ln.State)
			}
		case KindScoreSaved:
			if ln.ScoreID < 0 {
				t.Fatalf("score id %d", ln.ScoreID)
			}
		case KindReplaySaved:
			if ln.ReplayPath == "" || strings.Contains(ln.ReplayPath, "\n") {
				t.Fatalf("replay path %q", ln.ReplayPath)
			}
		case KindExit:
		default:
			t.Fatalf("kind %d", ln.Kind)
		}
	})
}

// refLines is the reference line splitter: complete lines only, trailing CRs removed.
func refLines(data []byte) []string {
	parts := strings.Split(string(data), "\n")
	out := []string{}
	for _, p := range parts[:len(parts)-1] {
		if len(p) > maxPending {
			continue
		}
		out = append(out, strings.TrimRight(p, "\r"))
	}
	return out
}

// splitAt cuts data at the positions given by cuts (each taken modulo len+1).
func splitAt(data, cuts []byte) [][]byte {
	pos := []int{0, len(data)}
	for _, c := range cuts {
		pos = append(pos, int(c)%(len(data)+1))
	}
	slices.Sort(pos)
	var out [][]byte
	for i := 1; i < len(pos); i++ {
		out = append(out, data[pos[i-1]:pos[i]])
	}
	return out
}

// FuzzTailFeed: whatever the chunking, feed yields the same lines as a whole-buffer split.
func FuzzTailFeed(f *testing.F) {
	f.Add([]byte("a\r\nb\nc"), []byte{2, 3})
	f.Add([]byte("\r\n\r\n\n"), []byte{1, 1, 1})
	f.Add([]byte("Screen has been switched to type: `Menu`\r\nxx"), []byte{40})
	f.Fuzz(func(t *testing.T, data, cuts []byte) {
		if len(cuts) > 64 {
			cuts = cuts[:64]
		}
		tl := &tailer{}
		got := []string{}
		for _, c := range splitAt(data, cuts) {
			got = tl.feed(got, c)
		}
		want := refLines(data)
		if !slices.Equal(got, want) {
			t.Fatalf("chunked %q\n got %q\nwant %q", data, got, want)
		}
		last := data[strings.LastIndexByte(string(data), '\n')+1:]
		if string(tl.pending) != string(last) {
			t.Fatalf("pending %q, want %q", tl.pending, last)
		}
	})
}

// FuzzTailStep drives the real file reader: the content is appended in pieces, with a step after each.
func FuzzTailStep(f *testing.F) {
	f.Add([]byte("one\r\ntw"), []byte("o\nthree\r"), []byte("\n"))
	f.Add([]byte(""), []byte("\n\n"), []byte("x"))
	f.Fuzz(func(t *testing.T, a, b, c []byte) {
		path := filepath.Join(t.TempDir(), "runtime.log")
		tl := &tailer{path: path, offset: -1, seenMissing: true, chunk: make([]byte, 7)}
		var all []byte
		got := []string{}
		for _, part := range [][]byte{a, b, c} {
			fh, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
			if err != nil {
				t.Fatal(err)
			}
			_, werr := fh.Write(part)
			fh.Close()
			if werr != nil {
				t.Fatal(werr)
			}
			all = append(all, part...)
			for i := 0; i < 1000; i++ {
				lines, more := tl.step()
				got = append(got, lines...)
				if !more {
					break
				}
			}
		}
		if want := refLines(all); !slices.Equal(got, want) {
			t.Fatalf("parts %q %q %q\n got %q\nwant %q", a, b, c, got, want)
		}
	})
}

// FuzzLastStateLine: the seed is the last state/exit line of a small log.
func FuzzLastStateLine(f *testing.F) {
	f.Add([]byte("Screen has been switched to type: `Menu`\r\nfoo\nDiscord RPC has shutdown"))
	f.Add([]byte("Screen has been switched to type: `Gameplay`\nSuccessfully saved score to database with id: 3\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		path := filepath.Join(t.TempDir(), "runtime.log")
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
		got, ok := LastStateLine(path, time.Hour)
		want, wantOK := "", false
		lines := strings.Split(string(data), "\n")
		for i := len(lines) - 1; i >= 0; i-- {
			l := strings.TrimRight(lines[i], "\r")
			if ln, k := ParseLine(l); k && (ln.Kind == KindState || ln.Kind == KindExit) {
				want, wantOK = l, true
				break
			}
		}
		if got != want || ok != wantOK {
			t.Fatalf("data %q: got %q %v, want %q %v", data, got, ok, want, wantOK)
		}
	})
}
