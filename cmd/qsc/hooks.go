package main

import "sync/atomic"

// hook holds a value that tests replace. Reads and writes are atomic, so a test that swaps it
// cannot race with a serve goroutine of an earlier test that is still winding down.
type hook[T any] struct{ p atomic.Pointer[T] }

func newHook[T any](v T) *hook[T] {
	h := &hook[T]{}
	h.p.Store(&v)
	return h
}

func (h *hook[T]) get() T { return *h.p.Load() }

// set replaces the value and returns a function that puts the old one back.
func (h *hook[T]) set(v T) (restore func()) {
	old := h.p.Swap(&v)
	return func() { h.p.Store(old) }
}
