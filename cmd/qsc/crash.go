package main

import (
	"fmt"
	"io"
	"log/slog"
	"runtime/debug"
)

// afterStart runs once the server is ready to accept connections; tests replace it to inject a panic.
var afterStart = newHook(func() {})

const crashHint = `qsc crashed: see qsc.log, and run "qsc doctor --report" for a bug report`

// logCrash records a panic and its stack in the log and tells the user where to look.
func logCrash(r any, errw io.Writer) {
	slog.Error("qsc crashed", "panic", fmt.Sprint(r), "stack", string(debug.Stack()))
	fmt.Fprintln(errw, crashHint)
}

// guard runs f and hands a panic to onPanic instead of letting it end the process.
func guard(f func(), onPanic func(any)) {
	defer func() {
		if r := recover(); r != nil {
			onPanic(r)
		}
	}()
	f()
}
