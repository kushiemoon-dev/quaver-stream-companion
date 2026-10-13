package main

import (
	"context"

	"io"
	"sync"
	"time"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/api"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/scores"
)

// recheckInterval is how often a degraded qsc looks for a usable Quaver folder again.
var recheckInterval = 15 * time.Second

// backend holds what only exists once a usable Quaver folder was found.
type backend struct {
	mu    sync.RWMutex
	dir   string
	store *scores.Store
}

func (b *backend) set(dir string, s *scores.Store) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.dir, b.store = dir, s
}

func (b *backend) current() (string, *scores.Store) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.dir, b.store
}

func (b *backend) close() {
	if _, s := b.current(); s != nil {
		s.Close()
	}
}

func (b *backend) recent(ctx context.Context, limit int) ([]scores.Result, error) {
	_, s := b.current()
	if s == nil {
		return nil, api.ErrUnavailable
	}
	return s.Recent(ctx, limit)
}

func (b *backend) timeline(ctx context.Context, id int64) (scores.Timeline, error) {
	_, s := b.current()
	if s == nil {
		return scores.Timeline{}, api.ErrUnavailable
	}
	return s.Timeline(ctx, id)
}

// syncWriter serializes writes from the goroutines that report progress.
type syncWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (s *syncWriter) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.w.Write(p)
}
