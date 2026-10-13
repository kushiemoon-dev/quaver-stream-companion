package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/bus"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/gamestate"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/scores"
)

// freshWindow is how recent a map must have been played to count as just started.
const freshWindow = 30 * time.Second

// Source reads the Quaver database.
type Source interface {
	LastPlayedMap(ctx context.Context) (scores.Map, error)
	ScoreByID(ctx context.Context, id int64) (scores.Result, error)
}

// App turns log lines into bus events.
type App struct {
	bus   *bus.Bus
	src   Source
	retry time.Duration
	tries int
	now   func() time.Time
}

// New builds an App publishing on b and reading from src.
func New(b *bus.Bus, src Source) *App {
	return &App{bus: b, src: src, retry: 400 * time.Millisecond, tries: 8, now: time.Now}
}

// Run turns runtime.log lines into bus events until ctx is done or lines closes.
func (a *App) Run(ctx context.Context, lines <-chan string) {
	for {
		select {
		case <-ctx.Done():
			return
		case l, ok := <-lines:
			if !ok {
				return
			}
			a.handle(ctx, l)
		}
	}
}

func (a *App) handle(ctx context.Context, raw string) {
	a.process(ctx, raw, a.now().Add(-freshWindow).UnixMilli())
}

// Seed handles the line found in an existing log at startup: the map in play is the last
// played one, however long ago the play began.
func (a *App) Seed(ctx context.Context, raw string) {
	a.process(ctx, raw, 0)
}

func (a *App) process(ctx context.Context, raw string, minPlayedMS int64) {
	ln, ok := gamestate.ParseLine(raw)
	if !ok {
		return
	}
	switch ln.Kind {
	case gamestate.KindState:
		slog.Info("game state", "state", ln.State)
		a.bus.Publish(bus.TypeGameState, bus.GameState{State: ln.State})
		if ln.State == "gameplay" {
			a.publishMap(ctx, minPlayedMS)
		}
	case gamestate.KindExit:
		slog.Info("game state", "state", "closed")
		a.bus.Publish(bus.TypeGameState, bus.GameState{State: "closed"})
	case gamestate.KindScoreSaved:
		a.publishScore(ctx, ln.ScoreID)
	}
}

func (a *App) publishMap(ctx context.Context, minPlayedMS int64) {
	var lastErr error
	for i := 0; i < a.tries; i++ {
		m, err := a.src.LastPlayedMap(ctx)
		lastErr = err
		if err == nil && m.LastPlayedMS >= minPlayedMS {
			slog.Info("map started", "md5", m.MD5, "keys", m.Keys)
			a.bus.Publish(bus.TypeMapStarted, m)
			return
		}
		if !a.wait(ctx) {
			return
		}
	}
	slog.Warn("no recently played map found in quaver.db; map.started skipped", "err", lastErr)
}

func (a *App) publishScore(ctx context.Context, id int64) {
	var lastErr error
	for i := 0; i < a.tries; i++ {
		r, err := a.src.ScoreByID(ctx, id)
		if err == nil {
			slog.Info("score saved", "id", id)
			a.bus.Publish(bus.TypeScoreSaved, r)
			return
		}
		lastErr = err
		if !a.wait(ctx) {
			return
		}
	}
	slog.Warn("score announced in the log could not be read", "id", id, "err", lastErr)
}

func (a *App) wait(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(a.retry):
		return true
	}
}
