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
	// onReplaced runs when a reopen follows a replaced database file.
	onReplaced func()
	// onTick runs at each supervisor check.
	onTick func()
}

func (b *backend) set(dir string, s *scores.Store) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.dir, b.store = dir, s
}

func (b *backend) hook() func() {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.onReplaced
}

func (b *backend) current() (string, *scores.Store) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.dir, b.store
}

// problem tells why the open store should be replaced, or "" while it is fine.
func (b *backend) problem() string {
	b.mu.RLock()
	tick := b.onTick
	b.mu.RUnlock()
	if tick != nil {
		tick()
	}
	_, s := b.current()
	switch {
	case s == nil:
		return ""
	case s.Replaced():
		return "database file replaced"
	case !s.Healthy():
		return "repeated read errors"
	}
	return ""
}

// reopen replaces the store by a new one on the same folder.
func (b *backend) reopen() error {
	dir, old := b.current()
	if old == nil {
		return api.ErrUnavailable
	}
	s, err := scores.Open(dir)
	if err != nil {
		return err
	}
	replaced := old.Replaced()
	b.set(dir, s)
	old.Close()
	if hook := b.hook(); replaced && hook != nil {
		hook()
	}
	return nil
}

// LastPlayedMap, NewestScore and ScoreByID make the backend the score source of the app, so a
// reopened store is picked up without restarting it.
func (b *backend) LastPlayedMap(ctx context.Context) (scores.Map, error) {
	_, s := b.current()
	if s == nil {
		return scores.Map{}, api.ErrUnavailable
	}
	return s.LastPlayedMap(ctx)
}

func (b *backend) NewestScore(ctx context.Context) (scores.Result, error) {
	_, s := b.current()
	if s == nil {
		return scores.Result{}, api.ErrUnavailable
	}
	return s.NewestScore(ctx)
}

func (b *backend) MaxScoreID(ctx context.Context) (int64, error) {
	_, s := b.current()
	if s == nil {
		return 0, api.ErrUnavailable
	}
	return s.MaxScoreID(ctx)
}

func (b *backend) ScoreByID(ctx context.Context, id int64) (scores.Result, error) {
	_, s := b.current()
	if s == nil {
		return scores.Result{}, api.ErrUnavailable
	}
	return s.ScoreByID(ctx, id)
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

// relay passes log lines on, calling seen for each one.
func relay(ctx context.Context, in <-chan string, out chan<- string, seen func()) {
	for {
		select {
		case <-ctx.Done():
			return
		case l := <-in:
			seen()
			select {
			case out <- l:
			case <-ctx.Done():
				return
			}
		}
	}
}
