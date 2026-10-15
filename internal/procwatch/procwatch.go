// Package procwatch tells whether a Quaver process exists. It reads process names only: no
// command line or other process data is kept, logged or sent.
package procwatch

import "strings"

// Lister returns the names of the running programs.
type Lister interface {
	Names() ([]string, error)
}

// IsQuaver tells whether a process name is the Quaver game: Quaver.exe on Windows and under
// Wine, Quaver for a native build.
func IsQuaver(name string) bool {
	return strings.EqualFold(name, "Quaver.exe") || strings.EqualFold(name, "Quaver")
}

// Running reports whether a Quaver process is listed. A listing error is returned as such: it
// never means that Quaver is absent.
func Running(l Lister) (bool, error) {
	names, err := l.Names()
	if err != nil {
		return false, err
	}
	for _, n := range names {
		if IsQuaver(n) {
			return true, nil
		}
	}
	return false, nil
}
