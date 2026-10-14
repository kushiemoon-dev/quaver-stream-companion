package app

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/bus"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/gamestate"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/scores"
)

const (
	// freshWindow is how recent a map must have been played to count as just started.
	freshWindow = 30 * time.Second
	// lookupBudget is how long a score or map row is waited for before giving up.
	lookupBudget = 15 * time.Second
	// seedBudget is the shorter wait for the map of the line found at startup.
	seedBudget = 3 * time.Second
	firstRetry = 400 * time.Millisecond
	maxRetry   = 2 * time.Second
	queueSize  = 64
)

// Source reads the Quaver database.
type Source interface {
	LastPlayedMap(ctx context.Context) (scores.Map, error)
	ScoreByID(ctx context.Context, id int64) (scores.Result, error)
	NewestScore(ctx context.Context) (scores.Result, error)
	// MaxScoreID returns the highest score id, whatever the date of the row.
	MaxScoreID(ctx context.Context) (int64, error)
}

// App turns log lines into bus events.
type App struct {
	bus *bus.Bus
	src Source
	now func() time.Time

	retry, maxDelay, budget time.Duration
	sleep                   func(ctx context.Context, d time.Duration) bool

	mapJobs, scoreJobs chan job
	workers            sync.WaitGroup
	// lastMapDone is closed when the lookup of the newest map job has ended.
	lastMapDone chan struct{}

	// pubMu orders state, map and score events; stateSeq counts the states published.
	pubMu    sync.Mutex
	stateSeq int
	// nextState is closed and replaced each time a state is published.
	nextState chan struct{}

	lastMap   scores.Map
	hasMap    bool
	announced idSet
}

// New builds an App publishing on b and reading from src.
func New(b *bus.Bus, src Source) *App {
	return &App{bus: b, src: src, now: time.Now, retry: firstRetry, maxDelay: maxRetry, budget: lookupBudget, sleep: sleep}
}

func sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// job is a database lookup. done, when set, is closed once the lookup has ended or was dropped.
type job struct {
	run  func()
	done chan struct{}
}

// Run turns runtime.log lines into bus events until ctx is done or lines closes. State changes
// are published at once. The database lookups run on two ordered workers, one for maps and one
// for scores, so a slow score row never holds back the next map. A score waits for the map
// lookup of its play to end, so it never passes that map, and a map still unread when a later
// state arrives is dropped, never published late.
func (a *App) Run(ctx context.Context, lines <-chan string) {
	a.mapJobs, a.scoreJobs = make(chan job, queueSize), make(chan job, queueSize)
	for _, q := range []chan job{a.mapJobs, a.scoreJobs} {
		a.workers.Add(1)
		go func() {
			defer a.workers.Done()
			for j := range q {
				if ctx.Err() == nil {
					j.run()
				}
				if j.done != nil {
					close(j.done)
				}
			}
		}()
	}
	defer func() {
		close(a.mapJobs)
		close(a.scoreJobs)
		a.workers.Wait()
	}()
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

// enqueue adds a lookup. When the queue is full the oldest pending one is dropped, so the
// newest play is never lost in a burst.
func enqueue(q chan job, what string, j job) {
	for {
		select {
		case q <- j:
			return
		default:
		}
		select {
		case old := <-q:
			if old.done != nil {
				close(old.done)
			}
			slog.Warn("too many pending database lookups; dropped the oldest", "what", what)
		default:
		}
	}
}

func (a *App) handle(ctx context.Context, raw string) {
	minPlayed := a.now().Add(-freshWindow).UnixMilli()
	ln, ok := gamestate.ParseLine(raw)
	if !ok {
		return
	}
	switch ln.Kind {
	case gamestate.KindState:
		seq := a.publishState(ln.State)
		if ln.State == "gameplay" {
			done := make(chan struct{})
			a.lastMapDone = done
			enqueue(a.mapJobs, "map", job{run: func() { a.publishMap(ctx, minPlayed, a.budget, seq) }, done: done})
		}
	case gamestate.KindExit:
		a.publishState("closed")
	case gamestate.KindScoreSaved:
		gate := a.lastMapDone
		enqueue(a.scoreJobs, "score", job{run: func() {
			if gate != nil {
				select {
				case <-gate:
				case <-ctx.Done():
					return
				}
			}
			a.publishScore(ctx, ln.ScoreID)
		}})
	}
}

// Seed handles the line found in an existing log at startup: the map in play is the last
// played one, however long ago the play began. It runs in the caller before Run starts.
func (a *App) Seed(ctx context.Context, raw string) {
	ln, ok := gamestate.ParseLine(raw)
	if !ok {
		return
	}
	switch ln.Kind {
	case gamestate.KindState:
		a.publishState(ln.State)
		if ln.State == "gameplay" {
			a.publishMap(ctx, 0, min(a.budget, seedBudget), noSeq)
		}
	case gamestate.KindExit:
		a.publishState("closed")
	case gamestate.KindScoreSaved:
		a.publishScore(ctx, ln.ScoreID)
	}
}

// SeedSnapshot reads what a snapshot needs after a restart: the newest score, and the last
// played map while the game sits in results, select or menu. Nothing is published; the caller
// stores the values. It runs before Run starts.
func (a *App) SeedSnapshot(ctx context.Context, game string) (*scores.Map, *scores.Result) {
	var m *scores.Map
	var r *scores.Result
	if res, err := a.src.NewestScore(ctx); err == nil {
		r = &res
		a.announced.add(res.Score.ID, fingerprint(res))
	} else {
		slog.Debug("no score to seed the snapshot", "err", err)
	}
	switch game {
	case "results", "select", "menu":
		if mp, err := a.src.LastPlayedMap(ctx); err == nil {
			m = &mp
			a.lastMap, a.hasMap = mp, true
		} else {
			slog.Debug("no map to seed the snapshot", "err", err)
		}
	}
	return m, r
}

// ForgetScores lets every score id be announced again, as after the database was replaced.
func (a *App) ForgetScores() { a.announced.reset() }

// noSeq publishes a map whatever state came since: used for the line read at startup.
const noSeq = -1

// publishState publishes a state and returns its sequence number.
func (a *App) publishState(state string) int {
	a.pubMu.Lock()
	defer a.pubMu.Unlock()
	a.stateSeq++
	if a.nextState != nil {
		close(a.nextState)
		a.nextState = nil
	}
	slog.Info("game state", "state", state)
	a.bus.Publish(bus.TypeGameState, bus.GameState{State: state})
	return a.stateSeq
}

// retryLoop calls try until it reports done, waiting with a growing delay for at most budget.
func (a *App) retryLoop(ctx context.Context, budget time.Duration, try func() (done bool)) bool {
	delay := a.retry
	for waited := time.Duration(0); ; {
		if try() {
			return true
		}
		if waited >= budget || !a.sleep(ctx, delay) {
			return false
		}
		waited += delay
		delay = min(delay*3/2+1, a.maxDelay)
	}
}

// publishMap waits for the map row of the play that started with state number seq. It gives up
// without publishing once a later state was published.
func (a *App) publishMap(ctx context.Context, minPlayedMS int64, budget time.Duration, seq int) {
	var lastErr error
	stale := false
	ctx, stop := a.untilNextState(ctx, seq)
	defer stop()
	ok := a.retryLoop(ctx, budget, func() bool {
		if stale = a.superseded(seq); stale {
			return true
		}
		m, err := a.src.LastPlayedMap(ctx)
		lastErr = err
		if err != nil || m.LastPlayedMS < minPlayedMS || !a.isNewMap(m) {
			return false
		}
		a.pubMu.Lock()
		defer a.pubMu.Unlock()
		if stale = seq != noSeq && seq != a.stateSeq; stale {
			return true
		}
		a.lastMap, a.hasMap = m, true
		slog.Info("map started", "md5", m.MD5, "keys", m.Keys)
		a.bus.Publish(bus.TypeMapStarted, m)
		return true
	})
	if stale || (!ok && a.superseded(seq)) {
		slog.Debug("map dropped: the game state changed first")
		return
	}
	if !ok && ctx.Err() == nil {
		slog.Warn("no recently played map found in quaver.db; map.started skipped", "err", lastErr)
	}
}

// untilNextState returns a context that ends as soon as a state after number seq is published,
// so a map lookup waiting between retries gives up at once.
func (a *App) untilNextState(ctx context.Context, seq int) (context.Context, func()) {
	ctx, cancel := context.WithCancel(ctx)
	a.pubMu.Lock()
	if a.nextState == nil {
		a.nextState = make(chan struct{})
	}
	changed := a.nextState
	late := seq != noSeq && seq != a.stateSeq
	a.pubMu.Unlock()
	if late {
		cancel()
		return ctx, cancel
	}
	if seq != noSeq {
		go func() {
			select {
			case <-changed:
				cancel()
			case <-ctx.Done():
			}
		}()
	}
	return ctx, cancel
}

func (a *App) superseded(seq int) bool {
	a.pubMu.Lock()
	defer a.pubMu.Unlock()
	return seq != noSeq && seq != a.stateSeq
}

// isNewMap tells whether m is not the map row that was published last.
func (a *App) isNewMap(m scores.Map) bool {
	return !a.hasMap || m.MD5 != a.lastMap.MD5 || m.LastPlayedMS > a.lastMap.LastPlayedMS
}

// fingerprint identifies the play behind a score row, so a reused id can be told from a replay.
func fingerprint(r scores.Result) string {
	return fmt.Sprintf("%s|%d|%s|%v", r.Score.PlayedAt, r.Score.TotalScore, r.Map.MD5, r.Score.Accuracy)
}

// rowChanged tells whether the row of an announced id is another play now: the id was reused by a
// restored database before the next restore check.
func (a *App) rowChanged(ctx context.Context, id int64) bool {
	r, err := a.src.ScoreByID(ctx, id)
	return err == nil && fingerprint(r) != a.announced.fingerprint(id)
}

// CheckRestore forgets the announced ids when the newest score of the database is lower than
// one already announced: an older database was copied over the live file.
func (a *App) CheckRestore(ctx context.Context) {
	top := a.announced.highest()
	if top == 0 {
		return
	}
	if max, err := a.src.MaxScoreID(ctx); err == nil && max < top {
		slog.Info("quaver.db has older scores than announced; forgetting announced scores", "newest", max, "announced", top)
		a.ForgetScores()
	}
}

func (a *App) publishScore(ctx context.Context, id int64) {
	a.CheckRestore(ctx)
	if a.announced.has(id) && !a.rowChanged(ctx, id) {
		slog.Debug("score already announced", "id", id)
		return
	}
	var lastErr error
	ok := a.retryLoop(ctx, a.budget, func() bool {
		r, err := a.src.ScoreByID(ctx, id)
		lastErr = err
		if err != nil {
			return false
		}
		a.announced.add(id, fingerprint(r))
		slog.Info("score saved", "id", id)
		a.bus.Publish(bus.TypeScoreSaved, r)
		return true
	})
	if !ok && ctx.Err() == nil {
		slog.Warn("score announced in the log could not be read", "id", id, "err", lastErr)
	}
}
