//go:build windows

package main

import (
	"os"
	"syscall"
)

// isConsole reports whether f is attached to a console window.
func isConsole(f *os.File) bool {
	var mode uint32
	return syscall.GetConsoleMode(syscall.Handle(f.Fd()), &mode) == nil
}
