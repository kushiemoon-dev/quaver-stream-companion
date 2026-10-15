package procwatch

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// cmdlineLimit bounds what is read of a command line.
const cmdlineLimit = 4096

// ProcFS lists processes from a /proc style tree.
type ProcFS struct{ Root string }

// Names returns the process names. A Wine loader is reported as Quaver.exe when its command
// line holds that name; the command line is read for Wine loaders only and is dropped at once.
// A tree without any process is an error, as when /proc is hidden.
func (p ProcFS) Names() ([]string, error) {
	entries, err := os.ReadDir(p.Root)
	if err != nil {
		return nil, err
	}
	var names []string
	seen := 0
	for _, e := range entries {
		if !e.IsDir() || !isPID(e.Name()) {
			continue
		}
		seen++
		if p.isZombie(e.Name()) {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(p.Root, e.Name(), "comm"))
		if err != nil {
			continue
		}
		name := strings.TrimSpace(string(raw))
		if strings.HasPrefix(strings.ToLower(name), "wine") && p.cmdlineHasQuaver(e.Name()) {
			name = "Quaver.exe"
		}
		names = append(names, name)
	}
	if seen == 0 {
		return nil, errors.New("no process is visible")
	}
	if len(names) == 0 {
		return nil, errors.New("no process name could be read")
	}
	return names, nil
}

// isZombie tells whether the process has ended and only waits to be reaped; it no longer runs.
func (p ProcFS) isZombie(pid string) bool {
	f, err := os.Open(filepath.Join(p.Root, pid, "stat"))
	if err != nil {
		return false
	}
	defer f.Close()
	buf, err := io.ReadAll(io.LimitReader(f, cmdlineLimit))
	if err != nil {
		return false
	}
	i := bytes.LastIndexByte(buf, ')')
	return i >= 0 && i+2 < len(buf) && buf[i+1] == ' ' && buf[i+2] == 'Z'
}

func (p ProcFS) cmdlineHasQuaver(pid string) bool {
	f, err := os.Open(filepath.Join(p.Root, pid, "cmdline"))
	if err != nil {
		return false
	}
	defer f.Close()
	buf, err := io.ReadAll(io.LimitReader(f, cmdlineLimit))
	return err == nil && bytes.Contains(bytes.ToLower(buf), []byte("quaver.exe"))
}

func isPID(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
