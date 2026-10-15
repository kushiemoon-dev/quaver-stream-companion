package app

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/bus"
)

func TestRetryLoopDelaysAndTries(t *testing.T) {
	ms := time.Millisecond
	for _, tc := range []struct {
		name            string
		first, maxDelay time.Duration
		budget          time.Duration
		wantWaits       []time.Duration
		wantTries       int
	}{
		{"stops once the waits reach the budget", 400 * ms, 2000 * ms, 1000*ms + 1, []time.Duration{400 * ms, 600*ms + 1}, 3},
		{"stops after the wait that passes the budget", 400 * ms, 2000 * ms, 1000 * ms, []time.Duration{400 * ms, 600*ms + 1}, 3},
		{"delay is capped", 1000 * ms, 1200 * ms, 3400 * ms, []time.Duration{1000 * ms, 1200 * ms, 1200 * ms}, 4},
		{"zero budget tries once", 400 * ms, 2000 * ms, 0, nil, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := New(bus.New(), nil)
			a.retry, a.maxDelay = tc.first, tc.maxDelay
			var waits []time.Duration
			a.sleep = func(_ context.Context, d time.Duration) bool {
				waits = append(waits, d)
				return true
			}
			tries := 0
			if a.retryLoop(context.Background(), tc.budget, func() bool { tries++; return false }) {
				t.Fatal("retryLoop reported success")
			}
			if !slices.Equal(waits, tc.wantWaits) || tries != tc.wantTries {
				t.Fatalf("waits %v tries %d, want %v and %d", waits, tries, tc.wantWaits, tc.wantTries)
			}
		})
	}
}

func TestRetryLoopStopsWhenTheWaitIsCancelled(t *testing.T) {
	a := New(bus.New(), nil)
	calls := 0
	a.sleep = func(context.Context, time.Duration) bool { return false }
	if a.retryLoop(context.Background(), time.Hour, func() bool { calls++; return false }) || calls != 1 {
		t.Fatalf("calls = %d", calls)
	}
}

func TestRetryLoopReturnsOnceTheTryIsDone(t *testing.T) {
	a := New(bus.New(), nil)
	var waits []time.Duration
	a.sleep = func(_ context.Context, d time.Duration) bool { waits = append(waits, d); return true }
	tries := 0
	if !a.retryLoop(context.Background(), time.Hour, func() bool { tries++; return tries == 3 }) || len(waits) != 2 {
		t.Fatalf("tries %d waits %v", tries, waits)
	}
}
