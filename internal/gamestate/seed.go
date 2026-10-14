package gamestate

import (
	"bytes"
	"io"
	"os"
	"time"
)

// seedWindow is how much of the end of the log is searched for the last state line.
const seedWindow = 64 << 10

// LastStateLine returns the last screen-change or game-exit line in the last 64 KiB of the
// log at path, if the file was modified within maxAge. It only reads the file.
func LastStateLine(path string, maxAge time.Duration) (string, bool) {
	f, err := os.Open(path)
	if err != nil {
		return "", false
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil || time.Since(fi.ModTime()) > maxAge {
		return "", false
	}
	// One byte before the window tells whether the window starts at the start of a line.
	start := max(fi.Size()-seedWindow-1, 0)
	data := make([]byte, fi.Size()-start)
	n, err := f.ReadAt(data, start)
	if err != nil && err != io.EOF {
		return "", false
	}
	data = data[:n]
	if start > 0 {
		// The window starts inside a line: drop that partial line.
		i := bytes.IndexByte(data, '\n')
		if i < 0 {
			return "", false
		}
		data = data[i+1:]
	}
	lines := bytes.Split(data, []byte{'\n'})
	for i := len(lines) - 1; i >= 0; i-- {
		line := string(bytes.TrimRight(lines[i], "\r"))
		if ln, ok := ParseLine(line); ok && (ln.Kind == KindState || ln.Kind == KindExit) {
			return line, true
		}
	}
	return "", false
}
