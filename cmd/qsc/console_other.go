//go:build !windows

package main

import "os"

// isConsole reports whether f is a terminal; qsc only pauses on Windows, so this is a plain device check.
func isConsole(f *os.File) bool {
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}
