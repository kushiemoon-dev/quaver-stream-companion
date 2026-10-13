// Package logging writes the rolling qsc.log file and reads the end of log files for support reports.
package logging

import (
	"bytes"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"
)

// MaxBytes is the size at which qsc.log rolls over to qsc.log.1.
const MaxBytes = 1 << 20

const tailWindow = 256 << 10

// RollingFile appends to a file and, past its size limit, moves it to <path>.1 (replacing the older one).
type RollingFile struct {
	mu       sync.Mutex
	path     string
	max      int64
	f        *os.File
	size     int64
	now      func() time.Time
	nextRoll time.Time
}

// rename is replaced by tests to simulate a file another program holds open.
var rename = os.Rename

// rollBackoff is how long a failed roll is not tried again.
const rollBackoff = time.Minute

// OpenRolling opens path for appending, creating it with mode 0600 where the platform supports it.
func OpenRolling(path string, max int64) (*RollingFile, error) {
	r := &RollingFile{path: path, max: max, now: time.Now}
	if err := r.open(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *RollingFile) open() error {
	f, err := os.OpenFile(r.path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600)
	if err != nil {
		return err
	}
	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return err
	}
	r.f, r.size = f, fi.Size()
	return nil
}

// Write appends p, rolling the file first when p would not fit.
func (r *RollingFile) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.f == nil {
		return 0, os.ErrClosed
	}
	if r.size > 0 && r.size+int64(len(p)) > r.max && !r.now().Before(r.nextRoll) {
		if err := r.roll(); err != nil {
			return 0, err
		}
	}
	n, err := r.f.Write(p)
	r.size += int64(n)
	return n, err
}

// roll moves the log to a temporary name first, so the older copy is only replaced once that worked.
// When the file cannot be renamed (on Windows another program may hold it open) logging goes on in
// the same file and the roll is not tried again for a minute.
func (r *RollingFile) roll() error {
	if err := r.f.Close(); err != nil {
		return err
	}
	r.f = nil
	tmp := r.path + ".rolling"
	if err := rename(r.path, tmp); err != nil {
		r.nextRoll = r.now().Add(rollBackoff)
		return r.open()
	}
	old := r.path + ".1"
	_ = os.Remove(old)
	if err := rename(tmp, old); err != nil {
		r.nextRoll = r.now().Add(rollBackoff)
		_ = os.Remove(tmp)
	}
	return r.open()
}

// Close closes the file.
func (r *RollingFile) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.f == nil {
		return nil
	}
	err := r.f.Close()
	r.f = nil
	return err
}

type nopCloser struct{}

func (nopCloser) Close() error { return nil }

// Setup returns a logger that writes to the console and to the rolling file at path.
// It never fails: when the file cannot be opened the logger writes to the console only and says so once.
func Setup(path string, verbose bool, console io.Writer) (*slog.Logger, io.Closer) {
	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}
	opts := &slog.HandlerOptions{Level: level}
	if path == "" {
		return slog.New(slog.NewTextHandler(console, opts)), nopCloser{}
	}
	f, err := OpenRolling(path, MaxBytes)
	if err != nil {
		log := slog.New(slog.NewTextHandler(console, opts))
		log.Warn("cannot open the log file; logging to the console only", "err", err.Error())
		return log, nopCloser{}
	}
	return slog.New(slog.NewTextHandler(io.MultiWriter(console, f), opts)), f
}

// Tail returns the last n lines of the file at path, or nil when it cannot be read.
// Only the end of the file is read.
func Tail(path string, n int) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil
	}
	start := fi.Size() - tailWindow
	if start < 0 {
		start = 0
	}
	buf := make([]byte, fi.Size()-start)
	if _, err := f.ReadAt(buf, start); err != nil && err != io.EOF {
		return nil
	}
	if start > 0 {
		if i := bytes.IndexByte(buf, 10); i >= 0 {
			buf = buf[i+1:]
		}
	}
	text := strings.TrimRight(strings.ReplaceAll(string(buf), "\r\n", "\n"), "\n")
	if text == "" {
		return nil
	}
	lines := strings.Split(text, "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines
}
