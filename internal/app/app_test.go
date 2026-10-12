package app

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/bus"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/scores"
)

const (
	lineGameplay = "[1:05:29] - RUNTIME - IMPORTANT: Screen has been switched to type: `Gameplay`"
	lineExit     = "[3:31:14] - RUNTIME - IMPORTANT: Discord RPC has shutdown"
	lineScore78  = "[1:06:06] - RUNTIME - IMPORTANT: Successfully saved score to database with id: 78"
)

type fakeSource struct {
	mu         sync.Mutex
	lastMap    func(call int) (scores.Map, error)
	score      func(call int, id int64) (scores.Result, error)
	mapCalls   int
	scoreCalls int
}

func (f *fakeSource) LastPlayedMap(context.Context) (scores.Map, error) {
	f.mu.Lock()
	f.mapCalls++
	n := f.mapCalls
	f.mu.Unlock()
	return f.lastMap(n)
}

func (f *fakeSource) ScoreByID(_ context.Context, id int64) (scores.Result, error) {
	f.mu.Lock()
	f.scoreCalls++
	n := f.scoreCalls
	f.mu.Unlock()
	return f.score(n, id)
}

func setup(t *testing.T, src Source) (chan<- string, <-chan bus.Event) {
	t.Helper()
	b := bus.New()
	a := New(b, src)
	a.retry = time.Millisecond
	a.tries = 3
	events, unsub := b.Subscribe()
	t.Cleanup(unsub)
	lines := make(chan string, 8)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go a.Run(ctx, lines)
	return lines, events
}

func next(t *testing.T, ch <-chan bus.Event, typ string) bus.Event {
	t.Helper()
	select {
	case e := <-ch:
		if e.Type != typ {
			t.Fatalf("got event %q, want %q", e.Type, typ)
		}
		return e
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for %q", typ)
		return bus.Event{}
	}
}

func fresh(title string) scores.Map {
	return scores.Map{Title: title, LastPlayedMS: time.Now().UnixMilli()}
}

func TestGameplayPublishesStateThenMap(t *testing.T) {
	src := &fakeSource{lastMap: func(int) (scores.Map, error) { return fresh("Song"), nil }}
	lines, events := setup(t, src)

	lines <- lineGameplay

	gs := next(t, events, bus.TypeGameState).Data.(bus.GameState)
	if gs.State != "gameplay" {
		t.Fatalf("state = %q", gs.State)
	}
	m := next(t, events, bus.TypeMapStarted).Data.(scores.Map)
	if m.Title != "Song" {
		t.Fatalf("map = %+v", m)
	}
}

func TestExitPublishesClosedState(t *testing.T) {
	lines, events := setup(t, &fakeSource{})

	lines <- lineExit

	gs := next(t, events, bus.TypeGameState).Data.(bus.GameState)
	if gs.State != "closed" {
		t.Fatalf("state = %q", gs.State)
	}
}

func TestStaleMapIsRetriedUntilFresh(t *testing.T) {
	src := &fakeSource{lastMap: func(call int) (scores.Map, error) {
		if call < 2 {
			return scores.Map{Title: "stale", LastPlayedMS: 1}, nil
		}
		return fresh("fresh"), nil
	}}
	lines, events := setup(t, src)

	lines <- lineGameplay

	next(t, events, bus.TypeGameState)
	if m := next(t, events, bus.TypeMapStarted).Data.(scores.Map); m.Title != "fresh" {
		t.Fatalf("map = %+v", m)
	}
}

func TestStaleMapIsNotPublished(t *testing.T) {
	src := &fakeSource{lastMap: func(int) (scores.Map, error) {
		return scores.Map{Title: "stale", LastPlayedMS: 1}, nil
	}}
	lines, events := setup(t, src)

	lines <- lineGameplay

	next(t, events, bus.TypeGameState)
	select {
	case e := <-events:
		t.Fatalf("unexpected event %q", e.Type)
	case <-time.After(150 * time.Millisecond):
	}
}

func TestSeedGameplayPublishesOldMap(t *testing.T) {
	src := &fakeSource{lastMap: func(int) (scores.Map, error) {
		return scores.Map{Title: "old", LastPlayedMS: 1}, nil
	}}
	b := bus.New()
	a := New(b, src)
	a.retry = time.Millisecond
	a.tries = 3
	events, unsub := b.Subscribe()
	t.Cleanup(unsub)

	a.Seed(context.Background(), lineGameplay)

	next(t, events, bus.TypeGameState)
	if m := next(t, events, bus.TypeMapStarted).Data.(scores.Map); m.Title != "old" {
		t.Fatalf("map = %+v", m)
	}
}

func TestSeedOtherStatePublishesOnlyState(t *testing.T) {
	src := &fakeSource{lastMap: func(int) (scores.Map, error) { return fresh("Song"), nil }}
	b := bus.New()
	a := New(b, src)
	events, unsub := b.Subscribe()
	t.Cleanup(unsub)

	a.Seed(context.Background(), "[1:00:00] - RUNTIME - IMPORTANT: Screen has been switched to type: `Select`")

	if gs := next(t, events, bus.TypeGameState).Data.(bus.GameState); gs.State != "select" {
		t.Fatalf("state = %q", gs.State)
	}
	select {
	case e := <-events:
		t.Fatalf("unexpected event %q", e.Type)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestScoreSavedRetriesUntilRowExists(t *testing.T) {
	src := &fakeSource{score: func(call int, id int64) (scores.Result, error) {
		if call < 3 {
			return scores.Result{}, scores.ErrNotFound
		}
		return scores.Result{Score: scores.Score{ID: id}, FullCombo: true}, nil
	}}
	lines, events := setup(t, src)

	lines <- lineScore78

	r := next(t, events, bus.TypeScoreSaved).Data.(scores.Result)
	if r.Score.ID != 78 || !r.FullCombo {
		t.Fatalf("result = %+v", r)
	}
}

func TestUnrelatedLinesAreIgnored(t *testing.T) {
	src := &fakeSource{}
	lines, events := setup(t, src)

	lines <- "[1:05:29] - RUNTIME - IMPORTANT: Scheduled Screen Change"

	select {
	case e := <-events:
		t.Fatalf("unexpected event %q", e.Type)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestPublishMapKeepsGoingWhenSourceAlwaysErrors(t *testing.T) {
	src := &fakeSource{lastMap: func(int) (scores.Map, error) { return scores.Map{}, errors.New("database is locked") }}
	lines, events := setup(t, src)
	lines <- lineGameplay
	next(t, events, bus.TypeGameState)
	select {
	case e := <-events:
		t.Fatalf("unexpected event %v", e.Type)
	case <-time.After(100 * time.Millisecond):
	}
}
