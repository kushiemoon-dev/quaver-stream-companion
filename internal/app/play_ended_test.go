package app

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/bus"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/scores"
)

// numberedMaps returns a lookup that gives a new map row (Song1, Song2...) at each call.
func numberedMaps() func(int) (scores.Map, error) {
	var n atomic.Int64
	return func(int) (scores.Map, error) {
		i := n.Add(1)
		return scores.Map{MD5: fmt.Sprint("md5-", i), Title: fmt.Sprint("Song", i), LastPlayedMS: time.Now().UnixMilli() + i}, nil
	}
}

// readUntil returns the labels of the events up to and including the first one equal to stop.
func readUntil(t *testing.T, ch <-chan bus.Event, stop string) []string {
	t.Helper()
	var out []string
	timeout := time.After(10 * time.Second)
	for {
		select {
		case e := <-ch:
			out = append(out, label(e))
			if out[len(out)-1] == stop {
				return out
			}
		case <-timeout:
			t.Fatalf("no %q; got %v", stop, out)
		}
	}
}

func count(labels []string, prefix string) int {
	n := 0
	for _, l := range labels {
		if strings.HasPrefix(l, prefix) {
			n++
		}
	}
	return n
}

func TestPlayWithoutAScoreLineIsReportedUnsaved(t *testing.T) {
	eachProcs(t, func(t *testing.T) {
		lines, events := setup(t, &fakeSource{lastMap: numberedMaps()})

		lines <- lineGameplay
		got := readUntil(t, events, "map.started")
		lines <- lineResults
		got = append(got, readUntil(t, events, "play.ended:Song1:false")...)

		want := []string{"game.state:gameplay", "map.started", "game.state:results", "play.ended:Song1:false"}
		if !slices.Equal(got, want) {
			t.Fatalf("events = %v, want %v", got, want)
		}
	})
}

func TestQuitAndRetryReportsThePlayUnsavedAfterTheNewState(t *testing.T) {
	eachProcs(t, func(t *testing.T) {
		lines, events := setup(t, &fakeSource{lastMap: numberedMaps()})

		lines <- lineGameplay
		got := readUntil(t, events, "map.started")
		lines <- lineGameplay
		got = append(got, readUntil(t, events, "play.ended:Song1:false")...)

		want := []string{"game.state:gameplay", "map.started", "game.state:gameplay", "play.ended:Song1:false"}
		if !slices.Equal(got, want) {
			t.Fatalf("events = %v, want %v", got, want)
		}
	})
}

func TestSavedPlayIsNeverReportedUnsaved(t *testing.T) {
	eachProcs(t, func(t *testing.T) {
		lines, events := setup(t, &fakeSource{lastMap: numberedMaps(), score: instantScore})

		lines <- lineGameplay
		got := readUntil(t, events, "map.started")
		lines <- scoreLine("5")
		lines <- lineResults
		lines <- lineSelect
		got = append(got, readUntil(t, events, "game.state:select")...)
		if !slices.Contains(got, "score.saved:5") {
			got = append(got, readUntil(t, events, "score.saved:5")...)
		}
		if count(got, "play.ended") != 0 {
			t.Fatalf("a saved play must not be reported unsaved: %v", got)
		}
	})
}

func TestTwoQuickPlaysReportOnlyTheUnsavedOne(t *testing.T) {
	eachProcs(t, func(t *testing.T) {
		lines, events := setup(t, &fakeSource{lastMap: numberedMaps(), score: instantScore})

		lines <- lineGameplay
		got := readUntil(t, events, "map.started")
		lines <- lineResults
		got = append(got, readUntil(t, events, "play.ended:Song1:false")...)
		lines <- lineGameplay
		got = append(got, readUntil(t, events, "map.started")...)
		lines <- scoreLine("6")
		lines <- lineResults
		lines <- lineSelect
		got = append(got, readUntil(t, events, "game.state:select")...)
		if !slices.Contains(got, "score.saved:6") {
			got = append(got, readUntil(t, events, "score.saved:6")...)
		}
		if n := count(got, "play.ended"); n != 1 {
			t.Fatalf("play.ended came %d times, want 1: %v", n, got)
		}
	})
}

func TestGameStateCarriesTheScreenAndClosedDoesNot(t *testing.T) {
	lines, events := setup(t, &fakeSource{})

	lines <- lineSelect
	if gs := next(t, events, bus.TypeGameState).Data.(bus.GameState); gs.State != "select" || gs.Screen != "select" {
		t.Fatalf("game state = %+v", gs)
	}
	lines <- lineExit
	if gs := next(t, events, bus.TypeGameState).Data.(bus.GameState); gs.State != "closed" || gs.Screen != "" {
		t.Fatalf("game state = %+v", gs)
	}
}

func TestPlayEndedJSONShape(t *testing.T) {
	b, err := json.Marshal(bus.PlayEnded{Map: scores.Map{Title: "Song"}})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || string(got["saved"]) != "false" || !strings.Contains(string(got["map"]), `"title":"Song"`) {
		t.Fatalf("shape = %s", b)
	}
}

func TestCancelAfterAnUnsavedPlayLeavesNoGoroutine(t *testing.T) {
	baseline := runtime.NumGoroutine()
	b := bus.New()
	a := New(b, &fakeSource{lastMap: numberedMaps()})
	tune(a)
	events, unsub := b.Subscribe()
	defer unsub()
	lines := make(chan string, 4)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx, lines); close(done) }()

	lines <- lineGameplay
	readUntil(t, events, "map.started")
	lines <- lineResults
	readUntil(t, events, "play.ended:Song1:false")
	cancel()
	waitClosed(t, done)

	for deadline := time.Now().Add(5 * time.Second); runtime.NumGoroutine() > baseline && time.Now().Before(deadline); {
		runtime.Gosched()
	}
	if n := runtime.NumGoroutine(); n > baseline {
		t.Fatalf("goroutines = %d, baseline %d", n, baseline)
	}
}

// harness runs an App whose waits return at once and whose lookups give up after one try.
type harness struct {
	lines  chan string
	events <-chan bus.Event
	cancel context.CancelFunc
	done   chan struct{}
}

func startHarness(t *testing.T, src Source, sleep func(context.Context, time.Duration) bool) *harness {
	t.Helper()
	b := bus.New()
	a := New(b, src)
	a.retry, a.maxDelay, a.budget = time.Millisecond, time.Millisecond, 0
	if sleep != nil {
		a.sleep, a.budget = sleep, time.Hour
	}
	events, unsub := b.Subscribe()
	t.Cleanup(unsub)
	h := &harness{lines: make(chan string, 8), events: events, done: make(chan struct{})}
	var ctx context.Context
	ctx, h.cancel = context.WithCancel(context.Background())
	t.Cleanup(h.cancel)
	go func() { a.Run(ctx, h.lines); close(h.done) }()
	return h
}

// calls signals each ScoreByID call and answers with fn.
func signalling(calls chan<- int64, fn func(call int, id int64) (scores.Result, error)) *fakeSource {
	return &fakeSource{lastMap: numberedMaps(), score: func(n int, id int64) (scores.Result, error) {
		select {
		case calls <- id:
		default:
		}
		return fn(n, id)
	}}
}

func notFound(int, int64) (scores.Result, error) { return scores.Result{}, scores.ErrNotFound }

func TestLookupThatGaveUpBeforeResultsIsReportedAfterTheState(t *testing.T) {
	eachProcs(t, func(t *testing.T) {
		calls := make(chan int64, 8)
		h := startHarness(t, signalling(calls, notFound), nil)

		h.lines <- lineGameplay
		got := readUntil(t, h.events, "map.started")
		h.lines <- scoreLine("1")
		h.lines <- scoreLine("2")
		<-calls
		<-calls // the worker runs one job at a time: the first lookup has given up
		h.lines <- lineResults
		got = append(got, readUntil(t, h.events, "play.ended:Song1:false")...)

		want := []string{"game.state:gameplay", "map.started", "game.state:results", "play.ended:Song1:false"}
		if !slices.Equal(got, want) {
			t.Fatalf("events = %v, want %v", got, want)
		}
	})
}

func TestLookupThatGivesUpAfterResultsIsReportedOnce(t *testing.T) {
	eachProcs(t, func(t *testing.T) {
		release := make(chan struct{})
		calls := make(chan int64, 8)
		h := startHarness(t, signalling(calls, func(int, int64) (scores.Result, error) {
			<-release
			return scores.Result{}, scores.ErrNotFound
		}), nil)

		h.lines <- lineGameplay
		got := readUntil(t, h.events, "map.started")
		h.lines <- scoreLine("1")
		<-calls
		h.lines <- lineResults
		got = append(got, readUntil(t, h.events, "game.state:results")...)
		close(release)
		got = append(got, readUntil(t, h.events, "play.ended:Song1:false")...)
		h.lines <- lineSelect
		got = append(got, readUntil(t, h.events, "game.state:select")...)

		if n := count(got, "play.ended"); n != 1 {
			t.Fatalf("play.ended came %d times: %v", n, got)
		}
	})
}

func TestScoreFoundAfterResultsIsNeverReportedUnsaved(t *testing.T) {
	eachProcs(t, func(t *testing.T) {
		release := make(chan struct{})
		calls := make(chan int64, 8)
		h := startHarness(t, signalling(calls, func(_ int, id int64) (scores.Result, error) {
			<-release
			return scores.Result{Score: scores.Score{ID: id}}, nil
		}), nil)

		h.lines <- lineGameplay
		got := readUntil(t, h.events, "map.started")
		h.lines <- scoreLine("3")
		<-calls
		h.lines <- lineResults
		got = append(got, readUntil(t, h.events, "game.state:results")...)
		close(release)
		got = append(got, readUntil(t, h.events, "score.saved:3")...)
		h.lines <- lineSelect
		got = append(got, readUntil(t, h.events, "game.state:select")...)

		if count(got, "play.ended") != 0 {
			t.Fatalf("a saved play was reported unsaved: %v", got)
		}
	})
}

func TestShutdownDuringALongRetryReportsNothing(t *testing.T) {
	waiting := make(chan struct{}, 1)
	sleep := func(ctx context.Context, _ time.Duration) bool {
		select {
		case waiting <- struct{}{}:
		default:
		}
		<-ctx.Done()
		return false
	}
	calls := make(chan int64, 8)
	h := startHarness(t, signalling(calls, notFound), sleep)

	h.lines <- lineGameplay
	got := readUntil(t, h.events, "map.started")
	h.lines <- scoreLine("4")
	<-waiting
	h.lines <- lineResults
	got = append(got, readUntil(t, h.events, "game.state:results")...)
	h.cancel()
	waitClosed(t, h.done)

	for {
		select {
		case e := <-h.events:
			got = append(got, label(e))
			continue
		default:
		}
		break
	}
	if count(got, "play.ended") != 0 {
		t.Fatalf("a cancelled lookup reported the play: %v", got)
	}
}

func TestScoreJobDroppedFromAFullQueueReportsThePlayUnsaved(t *testing.T) {
	release := make(chan struct{})
	calls := make(chan int64, 8)
	src := signalling(calls, func(n int, id int64) (scores.Result, error) {
		if n == 1 {
			<-release
		}
		return scores.Result{Score: scores.Score{ID: id}}, nil
	})
	h := startHarness(t, src, nil)
	t.Cleanup(func() { close(release) })

	h.lines <- scoreLine("1") // keeps the score worker busy
	<-calls
	h.lines <- lineGameplay
	readUntil(t, h.events, "map.started")
	h.lines <- scoreLine("2") // waits in the queue
	h.lines <- lineResults
	readUntil(t, h.events, "game.state:results")
	for i := 0; i < queueSize; i++ { // the last line pushes the oldest job out
		h.lines <- scoreLine(fmt.Sprint(100 + i))
	}
	readUntil(t, h.events, "play.ended:Song1:false")
}

func TestAnnouncedScoreWinsOverAnEarlierFailedLookup(t *testing.T) {
	eachProcs(t, func(t *testing.T) {
		calls := make(chan int64, 8)
		h := startHarness(t, signalling(calls, func(n int, id int64) (scores.Result, error) {
			if n == 1 {
				return scores.Result{}, scores.ErrNotFound
			}
			return scores.Result{Score: scores.Score{ID: id}}, nil
		}), nil)

		h.lines <- lineGameplay
		got := readUntil(t, h.events, "map.started")
		for _, id := range []string{"1", "2", "3"} {
			h.lines <- scoreLine(id)
			<-calls
		}
		h.lines <- lineResults
		h.lines <- lineSelect
		got = append(got, readUntil(t, h.events, "game.state:select")...)

		if count(got, "play.ended") != 0 {
			t.Fatalf("a play with an announced score was reported unsaved: %v", got)
		}
	})
}
