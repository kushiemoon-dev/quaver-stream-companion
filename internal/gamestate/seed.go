package gamestate

import (
	"os"
	"strings"
	"time"
)

// LastStateLine returns the last screen-change or game-exit line of the log at path, if the
// file was modified within maxAge. It only reads the file.
func LastStateLine(path string, maxAge time.Duration) (string, bool) {
	fi, err := os.Stat(path)
	if err != nil || time.Since(fi.ModTime()) > maxAge {
		return "", false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	lines := strings.Split(string(data), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimRight(lines[i], "\r")
		if ln, ok := ParseLine(line); ok && (ln.Kind == KindState || ln.Kind == KindExit) {
			return line, true
		}
	}
	return "", false
}
