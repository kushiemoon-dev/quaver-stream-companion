package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/bus"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/scores"
)

type probe struct {
	found   bool
	err     error
	calls   int
	enabled bool
}

func (p *probe) alive() (bool, error) { p.calls++; return p.found, p.err }
func (p *probe) on() bool             { return p.enabled }

func watcher(t *testing.T) (*App, *procWatch, *probe, <-chan bus.Event) {
	t.Helper()
	b := bus.New()
	a := New(b, &fakeSource{})
	events, unsub := b.Subscribe()
	t.Cleanup(unsub)
	p := &probe{enabled: true}
	return a, newProcWatch(a, p.alive, p.on), p, events
}

func steps(w *procWatch, n int) {
	for range n {
		w.step()
	}
}

func noEvent(t *testing.T, ch <-chan bus.Event) {
	t.Helper()
	select {
	case e := <-ch:
		t.Fatalf("unexpected event %q", e.Type)
	default:
	}
}

func TestThreeMissesPublishClosed(t *testing.T) {
	a, w, p, events := watcher(t)
	a.publishState("gameplay", "gameplay")
	next(t, events, bus.TypeGameState)

	steps(w, 2)
	noEvent(t, events)
	w.step()
	gs := next(t, events, bus.TypeGameState).Data.(bus.GameState)
	if gs.State != "closed" || gs.Screen != "" {
		t.Fatalf("state = %+v", gs)
	}
	if p.calls != 3 {
		t.Fatalf("calls = %d", p.calls)
	}
}

func TestFoundProcessResetsTheCount(t *testing.T) {
	a, w, p, events := watcher(t)
	a.publishState("menu", "menu")
	next(t, events, bus.TypeGameState)

	steps(w, 2)
	p.found = true
	w.step()
	p.found = false
	steps(w, 2)
	noEvent(t, events)
	w.step()
	if gs := next(t, events, bus.TypeGameState).Data.(bus.GameState); gs.State != "closed" {
		t.Fatalf("state = %q", gs.State)
	}
}

func TestEnumerationErrorNeverCloses(t *testing.T) {
	a, w, p, events := watcher(t)
	a.publishState("gameplay", "gameplay")
	next(t, events, bus.TypeGameState)

	p.err = errors.New("cannot list")
	steps(w, 10)
	noEvent(t, events)

	p.err = nil
	steps(w, 2)
	p.err = errors.New("cannot list")
	w.step()
	p.err = nil
	steps(w, 2)
	noEvent(t, events)
}

func TestDisabledDoesNothing(t *testing.T) {
	a, w, p, events := watcher(t)
	a.publishState("gameplay", "gameplay")
	next(t, events, bus.TypeGameState)

	p.enabled = false
	steps(w, 10)
	noEvent(t, events)
	if p.calls != 0 {
		t.Fatalf("listed %d times while disabled", p.calls)
	}
	p.enabled = true
	steps(w, 2)
	p.enabled = false
	w.step()
	p.enabled = true
	steps(w, 2)
	noEvent(t, events)
}

func TestNoStateYetOrClosedOrUnknownIsNotChecked(t *testing.T) {
	a, w, p, events := watcher(t)
	steps(w, 5)
	a.publishState("closed", "")
	next(t, events, bus.TypeGameState)
	steps(w, 5)
	a.publishState("unknown", "unknown")
	next(t, events, bus.TypeGameState)
	steps(w, 5)
	noEvent(t, events)
	if p.calls != 0 {
		t.Fatalf("listed %d times", p.calls)
	}
}

func TestClosedIsNotRepeated(t *testing.T) {
	a, w, _, events := watcher(t)
	a.publishState("results", "results")
	next(t, events, bus.TypeGameState)
	steps(w, 3)
	next(t, events, bus.TypeGameState)
	steps(w, 9)
	noEvent(t, events)
}

func TestNewLogStateRestartsTheCount(t *testing.T) {
	a, w, _, events := watcher(t)
	a.publishState("gameplay", "gameplay")
	next(t, events, bus.TypeGameState)
	steps(w, 2)
	a.publishState("results", "results")
	next(t, events, bus.TypeGameState)
	steps(w, 2)
	noEvent(t, events)
	w.step()
	if gs := next(t, events, bus.TypeGameState).Data.(bus.GameState); gs.State != "closed" {
		t.Fatalf("state = %q", gs.State)
	}
}

func TestRelaunchAfterCloseMovesOutOfClosed(t *testing.T) {
	b := bus.New()
	a := New(b, &fakeSource{lastMap: func(int) (scores.Map, error) { return fresh("Song"), nil }})
	a.retry, a.maxDelay, a.budget = time.Millisecond, time.Millisecond, 3*time.Millisecond
	events, unsub := b.Subscribe()
	defer unsub()
	p := &probe{enabled: true}
	w := newProcWatch(a, p.alive, p.on)
	a.publishState("gameplay", "gameplay")
	next(t, events, bus.TypeGameState)
	steps(w, 3)
	next(t, events, bus.TypeGameState)

	lines := make(chan string, 2)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); a.Run(ctx, lines) }()
	lines <- "[1:05:29] - RUNTIME - IMPORTANT: Screen has been switched to type: `Menu`"
	if gs := next(t, events, bus.TypeGameState).Data.(bus.GameState); gs.State != "menu" {
		t.Fatalf("state = %q", gs.State)
	}
	p.found = true
	steps(w, 6)
	noEvent(t, events)
	cancel()
	<-done
}

func TestWatchLoopStopsWithTheContext(t *testing.T) {
	a, _, p, events := watcher(t)
	a.publishState("gameplay", "gameplay")
	next(t, events, bus.TypeGameState)
	ticks := make(chan time.Time)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); a.WatchProcess(ctx, ticks, p.alive, p.on) }()
	for range 4 {
		ticks <- time.Time{}
	}
	cancel()
	<-done
	if gs := next(t, events, bus.TypeGameState).Data.(bus.GameState); gs.State != "closed" {
		t.Fatalf("state = %q", gs.State)
	}
}

func TestStateChangedDuringTheLastCheckIsNotClosed(t *testing.T) {
	b := bus.New()
	a := New(b, &fakeSource{})
	events, unsub := b.Subscribe()
	defer unsub()
	calls := 0
	alive := func() (bool, error) {
		calls++
		if calls == missesToClose {
			a.publishState("results", "results")
		}
		return false, nil
	}
	w := newProcWatch(a, alive, func() bool { return true })
	a.publishState("gameplay", "gameplay")
	next(t, events, bus.TypeGameState)

	steps(w, missesToClose)
	if gs := next(t, events, bus.TypeGameState).Data.(bus.GameState); gs.State != "results" {
		t.Fatalf("state = %q", gs.State)
	}
	noEvent(t, events)
	if state, _ := a.stateNow(); state != "results" {
		t.Fatalf("state = %q: the late close must not overwrite a newer state", state)
	}
}
