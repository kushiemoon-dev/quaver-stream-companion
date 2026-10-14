package app

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/bus"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/scores"
)

const (
	lineResults = "[1:05:50] - RUNTIME - IMPORTANT: Screen has been switched to type: `Results`"
	lineSelect  = "[1:05:31] - RUNTIME - IMPORTANT: Screen has been switched to type: `Select`"
)

func label(e bus.Event) string {
	switch d := e.Data.(type) {
	case bus.GameState:
		return "game.state:" + d.State
	case scores.Result:
		return fmt.Sprintf("score.saved:%d", d.Score.ID)
	case scores.Map:
		return "map.started"
	}
	return e.Type
}

// sequence returns the events published until none came for quiet.
func sequence(ch <-chan bus.Event, quiet time.Duration) []string {
	var out []string
	for {
		select {
		case e := <-ch:
			out = append(out, label(e))
		case <-time.After(quiet):
			return out
		}
	}
}

func laggingMap(delay time.Duration) func(int) (scores.Map, error) {
	releaseAt := time.Now().Add(delay)
	return func(int) (scores.Map, error) {
		if time.Now().Before(releaseAt) {
			return scores.Map{}, scores.ErrNotFound
		}
		return fresh("Song"), nil
	}
}

func instantScore(_ int, id int64) (scores.Result, error) {
	return scores.Result{Score: scores.Score{ID: id}}, nil
}

func eachProcs(t *testing.T, f func(t *testing.T)) {
	for _, n := range []int{1, 8} {
		t.Run(fmt.Sprintf("procs%d", n), func(t *testing.T) {
			old := runtime.GOMAXPROCS(n)
			defer runtime.GOMAXPROCS(old)
			f(t)
		})
	}
}

func same(a, b []string) bool { return fmt.Sprint(a) == fmt.Sprint(b) }

func TestLateMapNeverFollowsALaterStateOrScore(t *testing.T) {
	eachProcs(t, func(t *testing.T) {
		src := &fakeSource{lastMap: laggingMap(time.Second), score: instantScore}
		lines, events := slowSetup(t, src)

		lines <- lineGameplay
		lines <- lineResults
		lines <- scoreLine("77")

		got := sequence(events, 1500*time.Millisecond)
		want := []string{"game.state:gameplay", "game.state:results", "score.saved:77"}
		if !same(got, want) {
			t.Fatalf("events = %v, want %v", got, want)
		}
	})
}

func TestQuickQuitDropsThePendingMap(t *testing.T) {
	eachProcs(t, func(t *testing.T) {
		src := &fakeSource{lastMap: laggingMap(time.Second)}
		lines, events := slowSetup(t, src)

		lines <- lineGameplay
		lines <- lineSelect

		got := sequence(events, 1500*time.Millisecond)
		if want := []string{"game.state:gameplay", "game.state:select"}; !same(got, want) {
			t.Fatalf("events = %v, want %v", got, want)
		}
	})
}

func TestScoreWaitsForTheMapOfItsPlay(t *testing.T) {
	eachProcs(t, func(t *testing.T) {
		src := &fakeSource{lastMap: laggingMap(time.Second), score: instantScore}
		lines, events := slowSetup(t, src)

		lines <- lineGameplay
		lines <- scoreLine("78")

		got := sequence(events, 1500*time.Millisecond)
		want := []string{"game.state:gameplay", "map.started", "score.saved:78"}
		if !same(got, want) {
			t.Fatalf("events = %v, want %v", got, want)
		}
	})
}

func TestStatesAreNotDelayedByAnUnreadMap(t *testing.T) {
	src := &fakeSource{lastMap: laggingMap(4 * time.Second), score: instantScore}
	lines, events := slowSetup(t, src)

	lines <- lineGameplay
	lines <- lineResults
	start := time.Now()
	next(t, events, bus.TypeGameState)
	next(t, events, bus.TypeGameState)
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("states took %v", d)
	}
}

func TestBurstKeepsTheNewestScore(t *testing.T) {
	release := make(chan struct{})
	src := &fakeSource{score: func(call int, id int64) (scores.Result, error) {
		if call == 1 {
			<-release
		}
		return scores.Result{Score: scores.Score{ID: id}}, nil
	}}
	lines, events := slowSetup(t, src)

	go func() {
		for i := 1; i <= 1000; i++ {
			lines <- scoreLine(fmt.Sprint(i))
		}
	}()
	time.Sleep(300 * time.Millisecond)
	close(release)

	got := sequence(events, 500*time.Millisecond)
	if len(got) == 0 || got[len(got)-1] != "score.saved:1000" {
		t.Fatalf("the newest score was lost: %d events, last %v", len(got), got[max(0, len(got)-1):])
	}
}

func TestCancelDuringARetryLeavesNothingRunning(t *testing.T) {
	time.Sleep(50 * time.Millisecond)
	baseline := runtime.NumGoroutine()
	src := &fakeSource{score: func(int, int64) (scores.Result, error) { return scores.Result{}, scores.ErrNotFound }}
	a := New(bus.New(), src)
	a.retry, a.maxDelay = 50*time.Millisecond, 50*time.Millisecond
	lines := make(chan string, 4)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx, lines); close(done) }()

	lines <- scoreLine("5")
	time.Sleep(150 * time.Millisecond)
	cancel()
	waitClosed(t, done)

	deadline := time.Now().Add(2 * time.Second)
	for runtime.NumGoroutine() > baseline && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if n := runtime.NumGoroutine(); n > baseline {
		t.Fatalf("goroutines = %d, baseline %d", n, baseline)
	}
}

func TestBackoffGrowsUpToTheCap(t *testing.T) {
	var mu sync.Mutex
	var waits []time.Duration
	src := &fakeSource{score: func(int, int64) (scores.Result, error) { return scores.Result{}, scores.ErrNotFound }}
	a := New(bus.New(), src)
	a.sleep = func(_ context.Context, d time.Duration) bool {
		mu.Lock()
		defer mu.Unlock()
		waits = append(waits, d)
		return true
	}

	a.publishScore(context.Background(), 9)

	if len(waits) < 6 || waits[0] != firstRetry {
		t.Fatalf("waits = %v", waits)
	}
	if waits[1] <= waits[0] || waits[2] <= waits[1] || waits[3] <= waits[2] {
		t.Fatalf("the delay must grow at first: %v", waits)
	}
	if waits[len(waits)-1] != maxRetry {
		t.Fatalf("the delay must stop at the cap: %v", waits)
	}
	var total time.Duration
	for _, w := range waits {
		total += w
	}
	if total < 15*time.Second || total > 18*time.Second {
		t.Fatalf("total wait %v, want about 15s", total)
	}
}

func within(t *testing.T, ch <-chan bus.Event, typ string, d time.Duration) bus.Event {
	t.Helper()
	select {
	case e := <-ch:
		if e.Type != typ {
			t.Fatalf("got %q, want %q", label(e), typ)
		}
		return e
	case <-time.After(d):
		t.Fatalf("no %q within %v", typ, d)
		return bus.Event{}
	}
}

func TestNextMapDoesNotWaitBehindASlowScoreRow(t *testing.T) {
	eachProcs(t, func(t *testing.T) {
		scoreAt := time.Now().Add(3 * time.Second)
		var calls atomic.Int64
		src := &fakeSource{
			lastMap: func(int) (scores.Map, error) {
				return scores.Map{MD5: "m", Title: "Song", LastPlayedMS: time.Now().UnixMilli() + calls.Add(1)}, nil
			},
			score: func(_ int, id int64) (scores.Result, error) {
				if time.Now().Before(scoreAt) {
					return scores.Result{}, scores.ErrNotFound
				}
				return scores.Result{Score: scores.Score{ID: id}}, nil
			},
		}
		lines, events := slowSetup(t, src)

		lines <- lineGameplay
		within(t, events, bus.TypeGameState, time.Second)
		within(t, events, bus.TypeMapStarted, time.Second)
		lines <- scoreLine("90")
		lines <- lineResults
		within(t, events, bus.TypeGameState, time.Second)

		lines <- lineGameplay
		within(t, events, bus.TypeGameState, 2*time.Second)
		within(t, events, bus.TypeMapStarted, 2*time.Second)
		if r := within(t, events, bus.TypeScoreSaved, 8*time.Second).Data.(scores.Result); r.Score.ID != 90 {
			t.Fatalf("score = %+v", r.Score)
		}
	})
}

func TestShortNextPlayStillGetsItsMapBehindASlowScoreRow(t *testing.T) {
	eachProcs(t, func(t *testing.T) {
		scoreAt := time.Now().Add(3 * time.Second)
		src := &fakeSource{
			lastMap: func(int) (scores.Map, error) { return fresh("Song"), nil },
			score: func(_ int, id int64) (scores.Result, error) {
				if time.Now().Before(scoreAt) {
					return scores.Result{}, scores.ErrNotFound
				}
				return scores.Result{Score: scores.Score{ID: id}}, nil
			},
		}
		lines, events := slowSetup(t, src)

		lines <- scoreLine("91")
		lines <- lineGameplay
		within(t, events, bus.TypeGameState, 2*time.Second)
		within(t, events, bus.TypeMapStarted, 2*time.Second)
		within(t, events, bus.TypeScoreSaved, 8*time.Second)
	})
}

func TestScoreDoesNotHangWhenItsMapJobIsDroppedByOverflow(t *testing.T) {
	release := make(chan struct{})
	var calls atomic.Int64
	src := &fakeSource{
		lastMap: func(int) (scores.Map, error) {
			if calls.Add(1) == 1 {
				<-release
			}
			return scores.Map{MD5: "m", Title: "Song", LastPlayedMS: time.Now().UnixMilli() + calls.Load()}, nil
		},
		score: instantScore,
	}
	lines, events := slowSetup(t, src)

	lines <- lineGameplay // its lookup is stuck
	for calls.Load() == 0 {
		time.Sleep(time.Millisecond)
	}
	lines <- lineGameplay // the map job the score will wait for
	lines <- scoreLine("55")
	for range 70 { // the queue overflows and drops the oldest pending map jobs
		lines <- lineGameplay
	}
	close(release)

	deadline := time.After(10 * time.Second)
	for {
		select {
		case e := <-events:
			if e.Type == bus.TypeScoreSaved {
				return
			}
		case <-deadline:
			t.Fatal("the score never came out: a dropped map job must release the scores waiting for it")
		}
	}
}

func TestShutdownEndsAMapLookupWaitingForItsNextRetry(t *testing.T) {
	src := &fakeSource{lastMap: func(int) (scores.Map, error) { return scores.Map{}, scores.ErrNotFound }}
	a := New(bus.New(), src)
	a.retry, a.maxDelay = 30*time.Second, 30*time.Second
	lines := make(chan string, 2)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx, lines); close(done) }()

	lines <- lineGameplay
	time.Sleep(150 * time.Millisecond)
	cancel()
	waitClosed(t, done)
}
