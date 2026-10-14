package app

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/bus"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/scores"
)

func scoreLine(id string) string {
	return "[1:06:06] - RUNTIME - IMPORTANT: Successfully saved score to database with id: " + id
}

func slowSetup(t *testing.T, src Source) (chan<- string, <-chan bus.Event) {
	t.Helper()
	b := bus.New()
	a := New(b, src)
	tune(a)
	events, unsub := b.Subscribe()
	t.Cleanup(unsub)
	lines := make(chan string, 8)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go a.Run(ctx, lines)
	return lines, events
}

func TestSlowScoreRowDoesNotBlockTheNextState(t *testing.T) {
	var releaseAt atomic.Int64
	releaseAt.Store(time.Now().Add(500 * time.Millisecond).UnixMilli())
	src := &fakeSource{
		lastMap: func(int) (scores.Map, error) { return fresh("Song"), nil },
		score: func(_ int, id int64) (scores.Result, error) {
			if time.Now().UnixMilli() < releaseAt.Load() {
				return scores.Result{}, scores.ErrNotFound
			}
			return scores.Result{Score: scores.Score{ID: id}}, nil
		},
	}
	lines, events := slowSetup(t, src)

	lines <- scoreLine("30")
	lines <- lineGameplay

	started := time.Now()
	if gs := next(t, events, bus.TypeGameState).Data.(bus.GameState); gs.State != "gameplay" {
		t.Fatalf("state = %q", gs.State)
	}
	if d := time.Since(started); d > 250*time.Millisecond {
		t.Fatalf("game.state took %v: the slow score row blocked the tail", d)
	}
	next(t, events, bus.TypeMapStarted)
	if r := next(t, events, bus.TypeScoreSaved).Data.(scores.Result); r.Score.ID != 30 {
		t.Fatalf("score = %+v", r.Score)
	}
}

func TestQuickPlaysKeepScoreOrder(t *testing.T) {
	var releaseAt atomic.Int64
	releaseAt.Store(time.Now().Add(300 * time.Millisecond).UnixMilli())
	src := &fakeSource{score: func(_ int, id int64) (scores.Result, error) {
		if id == 40 && time.Now().UnixMilli() < releaseAt.Load() {
			return scores.Result{}, scores.ErrNotFound
		}
		return scores.Result{Score: scores.Score{ID: id}}, nil
	}}
	lines, events := slowSetup(t, src)

	lines <- scoreLine("40")
	lines <- scoreLine("41")

	if r := next(t, events, bus.TypeScoreSaved).Data.(scores.Result); r.Score.ID != 40 {
		t.Fatalf("first score = %d, want 40", r.Score.ID)
	}
	if r := next(t, events, bus.TypeScoreSaved).Data.(scores.Result); r.Score.ID != 41 {
		t.Fatalf("second score = %d, want 41", r.Score.ID)
	}
}

func TestStateIsPublishedBeforeItsMap(t *testing.T) {
	src := &fakeSource{lastMap: func(int) (scores.Map, error) { return fresh("Song"), nil }}
	lines, events := slowSetup(t, src)

	lines <- lineGameplay

	next(t, events, bus.TypeGameState)
	next(t, events, bus.TypeMapStarted)
}

// virtualClock lets a test run the 15 second retry budget without waiting.
type virtualClock struct{ elapsed atomic.Int64 }

func (c *virtualClock) sleep(_ context.Context, d time.Duration) bool {
	c.elapsed.Add(int64(d))
	return true
}

func (c *virtualClock) since() time.Duration { return time.Duration(c.elapsed.Load()) }

func virtualApp(src Source, c *virtualClock) (*App, <-chan bus.Event, func()) {
	b := bus.New()
	a := New(b, src)
	a.sleep = c.sleep
	events, unsub := b.Subscribe()
	return a, events, unsub
}

func TestScoreWrittenFourAndAHalfSecondsLateIsStillAnnounced(t *testing.T) {
	c := &virtualClock{}
	src := &fakeSource{score: func(_ int, id int64) (scores.Result, error) {
		if c.since() < 4500*time.Millisecond {
			return scores.Result{}, scores.ErrNotFound
		}
		return scores.Result{Score: scores.Score{ID: id}}, nil
	}}
	a, events, unsub := virtualApp(src, c)
	defer unsub()

	a.publishScore(context.Background(), 50)

	if r := next(t, events, bus.TypeScoreSaved).Data.(scores.Result); r.Score.ID != 50 {
		t.Fatalf("score = %+v", r.Score)
	}
}

func TestScoreNeverWrittenIsGivenUpAfterTheBudget(t *testing.T) {
	c := &virtualClock{}
	src := &fakeSource{score: func(int, int64) (scores.Result, error) { return scores.Result{}, scores.ErrNotFound }}
	a, events, unsub := virtualApp(src, c)
	defer unsub()

	a.publishScore(context.Background(), 51)

	if got := c.since(); got < 14*time.Second || got > 20*time.Second {
		t.Fatalf("waited %v, want about 15s", got)
	}
	select {
	case e := <-events:
		t.Fatalf("unexpected event %q", e.Type)
	default:
	}
	if src.scoreCalls < 5 || src.scoreCalls > 40 {
		t.Fatalf("%d lookups in the budget", src.scoreCalls)
	}
}

func TestSeedMapWaitIsShorterThanTheBudget(t *testing.T) {
	c := &virtualClock{}
	src := &fakeSource{lastMap: func(int) (scores.Map, error) { return scores.Map{}, scores.ErrNotFound }}
	a, _, unsub := virtualApp(src, c)
	defer unsub()

	a.Seed(context.Background(), lineGameplay)

	if got := c.since(); got > 6*time.Second {
		t.Fatalf("startup waited %v for a map", got)
	}
}

func TestForgetScoresAllowsLowerIdsAfterADatabaseSwap(t *testing.T) {
	src := &fakeSource{score: func(_ int, id int64) (scores.Result, error) {
		return scores.Result{Score: scores.Score{ID: id}}, nil
	}}
	a, events, unsub := virtualApp(src, &virtualClock{})
	defer unsub()

	a.publishScore(context.Background(), 500)
	next(t, events, bus.TypeScoreSaved)
	a.publishScore(context.Background(), 500)
	select {
	case e := <-events:
		t.Fatalf("unexpected %q", e.Type)
	default:
	}
	a.ForgetScores()
	a.publishScore(context.Background(), 500)
	if r := next(t, events, bus.TypeScoreSaved).Data.(scores.Result); r.Score.ID != 500 {
		t.Fatalf("score = %+v", r.Score)
	}
}
