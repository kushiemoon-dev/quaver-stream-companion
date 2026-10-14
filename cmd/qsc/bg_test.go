package main

import (
	"bytes"
	"context"
	"testing"
	"time"
)

// runInBackground runs qsc and, when the test ends, stops it and waits for run to return, so no
// serve goroutine outlives its test and races with the next one.
func runInBackground(t *testing.T, ctx context.Context, cancel context.CancelFunc, args []string) {
	t.Helper()
	done := make(chan struct{})
	var out, errw bytes.Buffer
	go func() {
		defer close(done)
		run(ctx, args, &out, &errw)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Error("run did not return after the test")
		}
	})
}
