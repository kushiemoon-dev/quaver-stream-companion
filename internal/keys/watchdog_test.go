package keys

import (
	"testing"
	"time"
)

func (h *harness) offersTicks() bool {
	select {
	case h.ticks <- h.clock.now():
		return true
	case <-time.After(50 * time.Millisecond):
		return false
	}
}

func (h *harness) sendTick(t *testing.T) {
	t.Helper()
	select {
	case h.ticks <- h.clock.now():
	case <-time.After(2 * time.Second):
		t.Fatal("the service does not take ticks")
	}
}

// tickAndWait runs two sampling passes and returns once both have called the sampler.
func (h *harness) tickAndWait(t *testing.T, d time.Duration) {
	t.Helper()
	want := h.fake.calls.Load() + 2
	h.tick(t, d)
	deadline := time.Now().Add(2 * time.Second)
	for h.fake.calls.Load() < want {
		if time.Now().After(deadline) {
			t.Fatal("the sampling passes did not finish")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestWatchdogStopsSamplingAfterFifteenQuietMinutes(t *testing.T) {
	h := start(t, enabled("4k"))
	h.begin(t)
	h.tickAndWait(t, 14*time.Minute)

	h.clock.add(2 * time.Minute)
	h.sendTick(t)
	before := h.fake.calls.Load()
	if h.offersTicks() {
		t.Fatal("sampling must stop after 15 quiet minutes in gameplay")
	}
	if n := h.fake.calls.Load(); n != before {
		t.Fatalf("sampler called %d times after the watchdog, had %d", n, before)
	}
}

func TestWatchdogResumesWhenTheStateChanges(t *testing.T) {
	h := start(t, enabled("4k"))
	h.begin(t)
	h.clock.add(16 * time.Minute)
	h.sendTick(t)

	h.game("results")
	h.game("gameplay")
	h.fake.set("A", true)
	h.tick(t, 40*time.Millisecond)
	for {
		if f := h.next(t); f.Down[0] {
			return
		}
	}
}

func TestLogActivityKeepsALongPlayAlive(t *testing.T) {
	h := start(t, enabled("4k"))
	h.begin(t)
	h.tickAndWait(t, 14*time.Minute)
	h.svc.Touch()
	h.tickAndWait(t, 14*time.Minute)
	h.clock.add(2 * time.Minute)
	h.sendTick(t)
	if h.offersTicks() {
		t.Fatal("quiet for more than 15 minutes since the last line: sampling must stop")
	}
}
