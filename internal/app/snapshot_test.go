package app

import (
	"context"
	"testing"
	"time"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/bus"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/scores"
)

func seedSource() *fakeSource {
	return &fakeSource{
		lastMap: func(int) (scores.Map, error) { return scores.Map{MD5: "m", Title: "Last", LastPlayedMS: 7}, nil },
		newest:  func() (scores.Result, error) { return scores.Result{Score: scores.Score{ID: 90}}, nil },
		score:   instantScore,
	}
}

func TestSeedSnapshotFillsMapAndScoreWithoutEvents(t *testing.T) {
	for _, state := range []string{"results", "select", "menu"} {
		b := bus.New()
		a := New(b, seedSource())
		events, unsub := b.Subscribe()

		m, r := a.SeedSnapshot(context.Background(), state)

		if m == nil || m.Title != "Last" || r == nil || r.Score.ID != 90 {
			t.Fatalf("%s: map = %+v, score = %+v", state, m, r)
		}
		select {
		case e := <-events:
			t.Fatalf("%s: seeded data must not be published, got %q", state, e.Type)
		case <-time.After(20 * time.Millisecond):
		}
		unsub()
	}
}

func TestSeedSnapshotSkipsTheMapInOtherStates(t *testing.T) {
	for _, state := range []string{"gameplay", "loading", "closed", "unknown", ""} {
		m, r := New(bus.New(), seedSource()).SeedSnapshot(context.Background(), state)
		if m != nil {
			t.Errorf("%s: map = %+v, want none", state, m)
		}
		if r == nil || r.Score.ID != 90 {
			t.Errorf("%s: score = %+v", state, r)
		}
	}
}

func TestSeededScoreIsNeverAnnouncedAgain(t *testing.T) {
	b := bus.New()
	a := New(b, seedSource())
	a.SeedSnapshot(context.Background(), "select")
	events, unsub := b.Subscribe()
	defer unsub()

	a.publishScore(context.Background(), 90)

	select {
	case e := <-events:
		t.Fatalf("unexpected %q", e.Type)
	default:
	}
}

func TestSeedSnapshotToleratesAnEmptyDatabase(t *testing.T) {
	src := &fakeSource{lastMap: func(int) (scores.Map, error) { return scores.Map{}, scores.ErrNotFound }}
	m, r := New(bus.New(), src).SeedSnapshot(context.Background(), "menu")
	if m != nil || r != nil {
		t.Fatalf("map = %+v, score = %+v", m, r)
	}
}
