package app

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/bus"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/scores"
)

func TestRestoredDatabaseWithLowerIdsIsStillAnnounced(t *testing.T) {
	a, events, unsub := virtualApp(&fakeSource{score: instantScore}, &virtualClock{})
	defer unsub()
	ctx := context.Background()

	a.publishScore(ctx, 900)
	a.publishScore(ctx, 900)
	a.publishScore(ctx, 12)
	a.publishScore(ctx, 12)

	got := sequence(events, 20*time.Millisecond)
	if want := []string{"score.saved:900", "score.saved:12"}; !same(got, want) {
		t.Fatalf("events = %v, want %v", got, want)
	}
}

func TestAnnouncedIdsAreBounded(t *testing.T) {
	a := New(bus.New(), &fakeSource{})
	for i := int64(1); i <= 3*maxAnnounced; i++ {
		a.announced.add(i, "")
	}
	if n := a.announced.size(); n > maxAnnounced {
		t.Fatalf("%d ids remembered", n)
	}
	if a.announced.has(1) || !a.announced.has(3*maxAnnounced) {
		t.Fatal("the oldest ids must be forgotten first")
	}
}

// dbSource is a score source whose newest id the test moves like a restored database would.
func dbSource(newest *atomic.Int64) *fakeSource {
	return &fakeSource{
		score:  instantScore,
		newest: func() (scores.Result, error) { return scores.Result{Score: scores.Score{ID: newest.Load()}}, nil },
		maxID:  func() (int64, error) { return newest.Load(), nil },
	}
}

func TestCopyOverRestoreAnnouncesTheReusedIds(t *testing.T) {
	var newest atomic.Int64
	a, events, unsub := virtualApp(dbSource(&newest), &virtualClock{})
	defer unsub()
	ctx := context.Background()

	newest.Store(6)
	a.publishScore(ctx, 6)
	newest.Store(7)
	a.publishScore(ctx, 7)
	newest.Store(6) // an older copy of quaver.db with newest id 5 is copied over, then a play makes id 6
	a.publishScore(ctx, 6)
	newest.Store(7)
	a.publishScore(ctx, 7)
	newest.Store(8)
	a.publishScore(ctx, 8)

	got := sequence(events, 20*time.Millisecond)
	want := []string{"score.saved:6", "score.saved:7", "score.saved:6", "score.saved:7", "score.saved:8"}
	if !same(got, want) {
		t.Fatalf("events = %v, want %v", got, want)
	}
}

func TestPlainTailReplayIsStillSkipped(t *testing.T) {
	var newest atomic.Int64
	a, events, unsub := virtualApp(dbSource(&newest), &virtualClock{})
	defer unsub()
	ctx := context.Background()

	newest.Store(7)
	a.publishScore(ctx, 6)
	a.publishScore(ctx, 7)
	a.publishScore(ctx, 6)
	a.publishScore(ctx, 7)

	got := sequence(events, 20*time.Millisecond)
	if want := []string{"score.saved:6", "score.saved:7"}; !same(got, want) {
		t.Fatalf("events = %v, want %v", got, want)
	}
}

func TestCheckRestoreForgetsWhenTheNewestIdDropped(t *testing.T) {
	var newest atomic.Int64
	a, events, unsub := virtualApp(dbSource(&newest), &virtualClock{})
	defer unsub()
	ctx := context.Background()

	newest.Store(7)
	a.publishScore(ctx, 7)
	a.CheckRestore(ctx)
	a.publishScore(ctx, 7)
	newest.Store(5)
	a.CheckRestore(ctx)
	a.publishScore(ctx, 7)

	got := sequence(events, 20*time.Millisecond)
	if want := []string{"score.saved:7", "score.saved:7"}; !same(got, want) {
		t.Fatalf("events = %v, want %v", got, want)
	}
}

func TestFutureDatedNewestRowDoesNotClearTheAnnouncedScores(t *testing.T) {
	var max atomic.Int64
	src := &fakeSource{
		score:  instantScore,
		newest: func() (scores.Result, error) { return scores.Result{Score: scores.Score{ID: 3}}, nil },
		maxID:  func() (int64, error) { return max.Load(), nil },
	}
	a, events, unsub := virtualApp(src, &virtualClock{})
	defer unsub()
	ctx := context.Background()

	max.Store(7)
	a.publishScore(ctx, 7)
	a.CheckRestore(ctx)
	a.publishScore(ctx, 7)

	got := sequence(events, 20*time.Millisecond)
	if want := []string{"score.saved:7"}; !same(got, want) {
		t.Fatalf("events = %v, want %v: the filtered newest score is not the highest id", got, want)
	}
}

func TestReusedIdSavedBeforeTheNextCheckIsAnnounced(t *testing.T) {
	var total atomic.Int64
	src := &fakeSource{
		score: func(_ int, id int64) (scores.Result, error) {
			return scores.Result{Score: scores.Score{ID: id, TotalScore: total.Load(), PlayedAt: "2026-10-14 10:00:00"}}, nil
		},
		maxID: func() (int64, error) { return 6, nil },
	}
	a, events, unsub := virtualApp(src, &virtualClock{})
	defer unsub()
	ctx := context.Background()

	total.Store(900000)
	a.publishScore(ctx, 6)
	a.publishScore(ctx, 6) // a tail replay: the same row
	total.Store(512000)    // the database was restored; the next play got id 6 again
	a.publishScore(ctx, 6)

	got := sequence(events, 20*time.Millisecond)
	if want := []string{"score.saved:6", "score.saved:6"}; !same(got, want) {
		t.Fatalf("events = %v, want %v", got, want)
	}
}

func TestResetForgetsTheHighestIdToo(t *testing.T) {
	var s idSet
	s.add(5, "x")
	s.reset()
	if s.highest() != 0 || s.size() != 0 || s.has(5) {
		t.Fatalf("highest = %d, size = %d after reset", s.highest(), s.size())
	}
}
