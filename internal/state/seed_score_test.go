package state

import (
	"testing"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/bus"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/scores"
)

// A seed must not overwrite a live score, and a nil seed leaves an empty field empty.
func TestSeedNeverOverwritesALiveScore(t *testing.T) {
	s := New()
	s.Seed(nil, nil)
	if got := s.Snapshot(); got.Map != nil || got.LastScore != nil {
		t.Fatalf("nil seed filled the snapshot: %+v", got)
	}
	s.Apply(bus.Event{Data: scores.Result{Score: scores.Score{ID: 9}}})
	old := scores.Result{Score: scores.Score{ID: 3}}
	s.Seed(nil, &old)
	if got := s.Snapshot(); got.LastScore == nil || got.LastScore.Score.ID != 9 {
		t.Fatalf("a seed must not overwrite a live score, got %+v", got.LastScore)
	}
}
