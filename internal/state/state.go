package state

import (
	"context"
	"sync"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/bus"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/keys"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/scores"
)

type Snapshot struct {
	Game      string         `json:"game"`
	Map       *scores.Map    `json:"map"`
	LastScore *scores.Result `json:"last_score"`
	Keys      *keys.Frame    `json:"keys"`
}

type Store struct {
	mu   sync.RWMutex
	snap Snapshot
}

func New() *Store {
	return &Store{snap: Snapshot{Game: "unknown"}}
}

func (s *Store) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snap
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
	default:
		return
	}
	s.snap = next
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
			s.Apply(e)
		}
	}
}
