package app

import (
	"context"
	"log/slog"
	"time"
)

// missesToClose is how many checks in a row must find no Quaver process before the game is
// reported closed.
const missesToClose = 3

// procWatch counts consecutive checks without a Quaver process.
type procWatch struct {
	a       *App
	alive   func() (bool, error)
	enabled func() bool

	misses int
	seq    int
}

func newProcWatch(a *App, alive func() (bool, error), enabled func() bool) *procWatch {
	return &procWatch{a: a, alive: alive, enabled: enabled}
}

// step runs one check. A check that cannot tell, or a state that moved since the last check,
// starts the count again, so only an unbroken run of clear absences closes the game.
func (w *procWatch) step() {
	if !w.enabled() {
		w.misses = 0
		return
	}
	state, seq := w.a.stateNow()
	if state == "" || state == "closed" || state == "unknown" {
		w.misses, w.seq = 0, seq
		return
	}
	if seq != w.seq {
		w.misses, w.seq = 0, seq
	}
	found, err := w.alive()
	if err != nil {
		slog.Debug("process check unavailable", "err", err)
		w.misses = 0
		return
	}
	if found {
		w.misses = 0
		return
	}
	if w.misses++; w.misses < missesToClose {
		return
	}
	w.misses = 0
	if w.a.closeIfStill(seq) {
		slog.Info("Quaver is not running; reporting the game as closed")
	}
}

// WatchProcess calls alive on every tick while the game is thought to be running, and reports it
// closed after three checks in a row that find no Quaver process. alive reports whether a Quaver
// process exists; an error never counts as an absence. enabled is asked at each tick. It returns
// when ctx is done.
func (a *App) WatchProcess(ctx context.Context, ticks <-chan time.Time, alive func() (bool, error), enabled func() bool) {
	w := newProcWatch(a, alive, enabled)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticks:
			w.step()
		}
	}
}

func (a *App) stateNow() (string, int) {
	a.pubMu.Lock()
	defer a.pubMu.Unlock()
	return a.state, a.stateSeq
}

// closeIfStill publishes the closed state unless a state was published after number seq.
func (a *App) closeIfStill(seq int) bool {
	a.pubMu.Lock()
	defer a.pubMu.Unlock()
	if a.stateSeq != seq || a.state == "closed" {
		return false
	}
	a.setState("closed", "")
	return true
}
