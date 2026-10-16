package state

import (
	"context"
	"sync"
	"time"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/bus"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/keys"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/scores"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/session"
)

type Snapshot struct {
	Game      string         `json:"game"`
	Map       *scores.Map    `json:"map"`
	LastScore *scores.Result `json:"last_score"`
	Keys      *keys.Frame    `json:"keys"`
	// Session holds the statistics since qsc started or the last reset.
	Session session.Summary `json:"session"`
}

type Store struct {
	mu    sync.RWMutex
	snap  Snapshot
	stats *session.Stats
}

func New() *Store {
	return &Store{snap: Snapshot{Game: "unknown"}, stats: session.New(time.Now().UTC())}
}

func (s *Store) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snap := s.snap
	snap.Session = s.stats.Summary()
	return snap
}

// Seed fills the map and last score of a fresh snapshot without an event. A value already
// known is kept, so a live event that raced the seed wins.
func (s *Store) Seed(m *scores.Map, r *scores.Result) {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.snap
	if next.Map == nil && m != nil {
		next.Map = m
	}
	if next.LastScore == nil && r != nil {
		next.LastScore = r
	}
	s.snap = next
}

// Apply folds one event into the snapshot. Snapshots are replaced, never
// mutated, so a copy handed out earlier stays valid.
func (s *Store) Apply(e bus.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.snap
	switch d := e.Data.(type) {
	case bus.GameState:
		next.Game = d.State
	case scores.Map:
		m := d
		next.Map = &m
	case keys.Frame:
		f := d
		next.Keys = &f
	case scores.Result:
		r := d
		next.LastScore = &r
		s.stats.Saved(r)
	case bus.PlayEnded:
		s.stats.Abandoned()
		return
	default:
		return
	}
	s.snap = next
}

// Handle applies e and, after a saved score, announces the new session statistics.
func (s *Store) Handle(b *bus.Bus, e bus.Event) {
	s.Apply(e)
	if e.Type == bus.TypeScoreSaved {
		b.Publish(bus.TypeSessionUpdated, s.Snapshot().Session)
	}
}

// ResetSession clears the session statistics and announces them.
func (s *Store) ResetSession(b *bus.Bus) {
	s.stats.Reset(time.Now().UTC())
	b.Publish(bus.TypeSessionUpdated, s.Snapshot().Session)
}

func (s *Store) Run(ctx context.Context, b *bus.Bus) {
	ch, unsub := b.Subscribe()
	defer unsub()
	for {
		select {
		case <-ctx.Done():
			return
		case e, ok := <-ch:
			if !ok {
				return
			}
			s.Handle(b, e)
		}
	}
}
