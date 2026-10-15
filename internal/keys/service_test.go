package keys

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/bus"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/scores"
)

// fakeSampler stands in for the keyboard: tests never read the real one.
type fakeSampler struct {
	mu    sync.Mutex
	down  map[Key]bool
	calls atomic.Int64
	asked []Key
}

func newFake() *fakeSampler { return &fakeSampler{down: map[Key]bool{}} }

func (f *fakeSampler) set(k Key, v bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.down[k] = v
}

func (f *fakeSampler) Down(keys []Key) []bool {
	f.calls.Add(1)
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.asked == nil {
		f.asked = append([]Key(nil), keys...)
	}
	out := make([]bool, len(keys))
	for i, k := range keys {
		out[i] = f.down[k]
	}
	return out
}

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) add(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

type harness struct {
	b      *bus.Bus
	svc    *Service
	fake   *fakeSampler
	clock  *fakeClock
	ticks  chan time.Time
	frames <-chan bus.Event
}

func enabled(layout string) Config {
	c := Defaults()
	c.Enabled, c.Layout = true, layout
	return c
}

// start runs a service driven by a fake clock and manual ticks.
func start(t *testing.T, cfg Config) *harness {
	t.Helper()
	b := bus.New()
	fake := newFake()
	created := 0
	svc := NewService(b, cfg, func() (Sampler, error) { created++; return fake, nil })
	if cfg.Enabled && created != 1 {
		t.Fatalf("sampler created %d times, want 1", created)
	}
	clock := &fakeClock{t: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)}
	ticks := make(chan time.Time)
	svc.now, svc.ticks = clock.now, ticks
	ch, unsub := b.Subscribe()
	t.Cleanup(unsub)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { svc.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	return &harness{b: b, svc: svc, fake: fake, clock: clock, ticks: ticks, frames: ch}
}

func (h *harness) game(state string) { h.b.Publish(bus.TypeGameState, bus.GameState{State: state}) }

// tick moves the clock and runs one sampling pass. The second, empty tick returns only
// once the first one is fully processed.
func (h *harness) tick(t *testing.T, d time.Duration) {
	t.Helper()
	h.clock.add(d)
	for i := 0; i < 2; i++ {
		select {
		case h.ticks <- h.clock.now():
		case <-time.After(2 * time.Second):
			t.Fatal("service does not take ticks")
		}
	}
}

// next returns the next keys.frame, skipping other events.
func (h *harness) next(t *testing.T) Frame {
	t.Helper()
	for {
		select {
		case e := <-h.frames:
			if e.Type == bus.TypeKeysFrame {
				return e.Data.(Frame)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("no keys.frame")
		}
	}
}

// drain returns the frames already published.
func (h *harness) drain() []Frame {
	var out []Frame
	for {
		select {
		case e := <-h.frames:
			if e.Type == bus.TypeKeysFrame {
				out = append(out, e.Data.(Frame))
			}
		default:
			return out
		}
	}
}

// begin enters gameplay and returns the starting frame.
func (h *harness) begin(t *testing.T) Frame {
	t.Helper()
	h.game("gameplay")
	return h.next(t)
}

func TestDisabledNeverCreatesNorCallsASampler(t *testing.T) {
	b := bus.New()
	called := false
	svc := NewService(b, Defaults(), func() (Sampler, error) { called = true; return newFake(), nil })
	done := make(chan struct{})
	go func() { svc.Run(context.Background()); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run must return at once when sampling is disabled")
	}
	if called {
		t.Fatal("factory called while disabled")
	}
	if s := svc.Status(); s.Enabled || s.Layout != "auto" {
		t.Fatalf("status = %+v", s)
	}
}

func TestSamplerNeverCalledOutsideGameplay(t *testing.T) {
	h := start(t, enabled("4k"))
	for _, s := range []string{"menu", "loading", "results", "closed", "unknown"} {
		h.game(s)
	}
	h.b.Publish(bus.TypeMapStarted, scores.Map{Keys: 4})
	select {
	case h.ticks <- h.clock.now():
		t.Fatal("service takes ticks outside gameplay")
	case <-time.After(50 * time.Millisecond):
	}
	if n := h.fake.calls.Load(); n != 0 {
		t.Fatalf("sampler called %d times outside gameplay", n)
	}
}

func TestSamplesOnlyConfiguredLaneKeysWhilePlaying(t *testing.T) {
	cfg := enabled("4k")
	cfg.K4 = []Key{"F5", "F6", "F7", "F8"}
	h := start(t, cfg)
	h.begin(t)
	h.fake.set("F6", true)
	h.tick(t, 40*time.Millisecond)
	f := h.next(t)
	if !f.Down[1] || f.Presses[1] != 1 {
		t.Fatalf("frame = %+v", f)
	}
	h.fake.mu.Lock()
	asked := h.fake.asked
	h.fake.mu.Unlock()
	if len(asked) != 4 || asked[0] != "F5" || asked[1] != "F6" || asked[2] != "F7" || asked[3] != "F8" {
		t.Fatalf("sampler was asked for %v", asked)
	}
}

func TestRealTickerSamplesWithoutInjectedTicks(t *testing.T) {
	b := bus.New()
	fake := newFake()
	svc := NewService(b, enabled("4k"), func() (Sampler, error) { return fake, nil })
	svc.tick = time.Millisecond
	ch, unsub := b.Subscribe()
	defer unsub()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { svc.Run(ctx); close(done) }()
	defer func() { cancel(); <-done }()
	b.Publish(bus.TypeGameState, bus.GameState{State: "gameplay"})
	<-ch // game.state
	if e := <-ch; e.Type != bus.TypeKeysFrame {
		t.Fatalf("event = %+v", e)
	}
	fake.set("A", true)
	deadline := time.After(2 * time.Second)
	for {
		select {
		case e := <-ch:
			if f, ok := e.Data.(Frame); ok && f.Down[0] {
				return
			}
		case <-deadline:
			t.Fatal("no frame with the key down")
		}
	}
}

func TestLeavingGameplayStopsSamplingAndEmitsFinalFrame(t *testing.T) {
	h := start(t, enabled("4k"))
	h.begin(t)
	h.fake.set("A", true)
	h.tick(t, 40*time.Millisecond)
	if f := h.next(t); !f.Down[0] {
		t.Fatalf("frame = %+v", f)
	}
	h.game("results")
	f := h.next(t)
	if f.Down[0] || f.Presses[0] != 1 || f.Peak < 1 {
		t.Fatalf("final frame = %+v", f)
	}
	before := h.fake.calls.Load()
	select {
	case h.ticks <- h.clock.now():
		t.Fatal("service takes ticks after leaving gameplay")
	case <-time.After(50 * time.Millisecond):
	}
	if after := h.fake.calls.Load(); after != before {
		t.Fatalf("sampler polled after leaving gameplay: %d -> %d", before, after)
	}
}

func TestMapStartedResetsAndAutoLayoutFollowsKeyCount(t *testing.T) {
	h := start(t, enabled("auto"))
	h.begin(t)
	h.b.Publish(bus.TypeMapStarted, scores.Map{Keys: 7})
	f := h.next(t)
	if f.Layout != "7k" || f.Lanes != 7 || len(f.Down) != 7 || len(f.Presses) != 7 {
		t.Fatalf("frame = %+v", f)
	}
	h.fake.set("A", true)
	h.tick(t, 40*time.Millisecond)
	if f = h.next(t); f.Presses[0] != 1 {
		t.Fatalf("frame = %+v", f)
	}
	h.fake.set("A", false)
	h.b.Publish(bus.TypeMapStarted, scores.Map{Keys: 4})
	f = h.next(t)
	if f.Layout != "4k" || f.Lanes != 4 || f.Peak != 0 || f.Presses[0] != 0 {
		t.Fatalf("frame = %+v", f)
	}
}

func TestAutoLayoutHasNoLaneWhenUnknown(t *testing.T) {
	h := start(t, enabled("auto"))
	h.b.Publish(bus.TypeMapStarted, scores.Map{Keys: 0})
	if f := h.begin(t); f.Layout != "unknown" || f.Lanes != 0 || len(f.Down) != 0 {
		t.Fatalf("frame = %+v", f)
	}
}

func TestForcedLayoutIgnoresMapKeyCount(t *testing.T) {
	h := start(t, enabled("7k"))
	h.b.Publish(bus.TypeMapStarted, scores.Map{Keys: 4})
	if f := h.begin(t); f.Layout != "7k" || f.Lanes != 7 {
		t.Fatalf("frame = %+v", f)
	}
}

func TestNewRunWithoutMapStartedResetsCountersAndKeepsLastLayout(t *testing.T) {
	h := start(t, enabled("auto"))
	h.b.Publish(bus.TypeMapStarted, scores.Map{Keys: 7})
	h.begin(t)
	h.fake.set("A", true)
	h.tick(t, 40*time.Millisecond)
	if f := h.next(t); f.Presses[0] != 1 {
		t.Fatalf("frame = %+v", f)
	}
	h.fake.set("A", false)

	// A quick retry: gameplay again, with no map.started in between.
	h.game("gameplay")
	f := h.next(t)
	if f.Layout != "7k" || f.Lanes != 7 || f.Presses[0] != 0 || f.Peak != 0 || f.KPS != 0 {
		t.Fatalf("retry frame = %+v", f)
	}

	// And after leaving to the results screen first.
	h.fake.set("A", true)
	h.tick(t, 40*time.Millisecond)
	h.next(t)
	h.game("results")
	h.next(t)
	h.fake.set("A", false)
	h.game("gameplay")
	if f = h.next(t); f.Layout != "7k" || f.Presses[0] != 0 || f.Peak != 0 {
		t.Fatalf("new run frame = %+v", f)
	}
}

func TestKeyHeldWhenTheMapStartsIsNotAPress(t *testing.T) {
	h := start(t, enabled("4k"))
	h.begin(t)
	h.fake.set("A", true)
	h.tick(t, 40*time.Millisecond)
	if f := h.next(t); f.Presses[0] != 1 {
		t.Fatalf("frame = %+v", f)
	}
	h.b.Publish(bus.TypeMapStarted, scores.Map{Keys: 4})
	f := h.next(t)
	if !f.Down[0] || f.Presses[0] != 0 || f.KPS != 0 {
		t.Fatalf("held key counted at map start: %+v", f)
	}
	h.tick(t, 40*time.Millisecond)
	if got := h.drain(); len(got) != 0 {
		t.Fatalf("still held, nothing should change: %+v", got)
	}
	h.fake.set("A", false)
	h.tick(t, 40*time.Millisecond)
	h.fake.set("A", true)
	h.tick(t, 40*time.Millisecond)
	got := h.drain()
	if len(got) == 0 || got[len(got)-1].Presses[0] != 1 {
		t.Fatalf("a real press after release must count once: %+v", got)
	}
}

func TestFramesAreOnlySentOnChange(t *testing.T) {
	h := start(t, enabled("4k"))
	h.begin(t)
	for i := 0; i < 100; i++ {
		h.tick(t, time.Millisecond)
	}
	if got := h.drain(); len(got) != 0 {
		t.Fatalf("idle keys produced %d frames", len(got))
	}
}

func TestFramesAreThrottledToThirtyPerSecond(t *testing.T) {
	h := start(t, enabled("4k"))
	h.begin(t)
	for i := 0; i < 200; i++ {
		h.fake.set("A", i%2 == 0)
		h.tick(t, time.Millisecond)
	}
	// 200 ms of changes every millisecond: at most one frame per 33 ms.
	if n := len(h.drain()); n < 4 || n > 7 {
		t.Fatalf("%d frames in 200 ms, want 4 to 7", n)
	}
}

func TestTrailingFrameAfterAThrottledBurstIsDelivered(t *testing.T) {
	h := start(t, enabled("4k"))
	h.begin(t)
	h.fake.set("A", true)
	h.tick(t, 10*time.Millisecond)
	if got := h.drain(); len(got) != 0 {
		t.Fatalf("change inside the throttle gap was sent: %+v", got)
	}
	h.tick(t, 30*time.Millisecond)
	f := h.next(t)
	if !f.Down[0] || f.Presses[0] != 1 {
		t.Fatalf("trailing frame = %+v", f)
	}
}

func TestUnsupportedPlatformIsReportedAndNothingRuns(t *testing.T) {
	b := bus.New()
	svc := NewService(b, enabled("auto"), func() (Sampler, error) { return nil, ErrUnsupported })
	if s := svc.Status(); !s.Enabled || s.Supported {
		t.Fatalf("status = %+v", s)
	}
	ch, unsub := b.Subscribe()
	defer unsub()
	done := make(chan struct{})
	go func() { svc.Run(context.Background()); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run must return at once when the sampler is unsupported")
	}
	b.Publish(bus.TypeGameState, bus.GameState{State: "gameplay"})
	for len(ch) > 0 {
		if e := <-ch; e.Type == bus.TypeKeysFrame {
			t.Fatalf("frame emitted while unsupported: %+v", e)
		}
	}
	if !strings.Contains(ErrUnsupported.Error(), "not supported") {
		t.Fatalf("message = %q", ErrUnsupported)
	}
}

func TestStatusReportsSupportWhenSamplerExists(t *testing.T) {
	svc := NewService(bus.New(), enabled("7k"), func() (Sampler, error) { return newFake(), nil })
	if s := svc.Status(); !s.Enabled || !s.Supported || s.Layout != "7k" {
		t.Fatalf("status = %+v", s)
	}
}

func TestNoKeyIdentityInEventsOrLogs(t *testing.T) {
	var logs bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(old) })

	cfg := enabled("4k")
	cfg.K4 = []Key{"F9", "F10", "SHIFT_R", "CTRL_L"}
	h := start(t, cfg)
	h.begin(t)
	h.fake.set("F10", true)
	h.fake.set("SHIFT_R", true)
	h.tick(t, 40*time.Millisecond)
	h.game("menu")
	var all []string
	for deadline := time.After(100 * time.Millisecond); ; {
		select {
		case e := <-h.frames:
			raw, err := json.Marshal(e)
			if err != nil {
				t.Fatal(err)
			}
			all = append(all, string(raw))
			continue
		case <-deadline:
		}
		break
	}
	if len(all) < 2 {
		t.Fatalf("captured %d events", len(all))
	}
	blob := strings.Join(all, "\n") + logs.String()
	for _, name := range []string{"F9", "F10", "SHIFT", "CTRL", "shift", "ctrl"} {
		if strings.Contains(blob, name) {
			t.Fatalf("key identity %q leaked in events or logs:\n%s", name, blob)
		}
	}
	for _, e := range all {
		var m map[string]any
		_ = json.Unmarshal([]byte(e), &m)
		if d, _ := m["data"].(map[string]any); m["type"] == bus.TypeKeysFrame && len(d) != 7 {
			t.Fatalf("keys.frame data must be exactly layout, lanes, scratch, down, presses, kps, peak: %v", d)
		}
	}
}
