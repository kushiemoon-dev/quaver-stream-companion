package keys

import (
	"context"
	"log/slog"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/bus"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/scores"
)

const (
	sampleEvery = 2 * time.Millisecond
	frameGap    = time.Second / 30
	// quietLimit is how long gameplay may last without any activity (a log line or an event)
	// before sampling stops; it starts again at the next game state change.
	quietLimit = 15 * time.Minute
)

// Status tells the dashboard and doctor what the sampler is doing.
type Status struct {
	Enabled   bool   `json:"enabled"`
	Supported bool   `json:"supported"`
	Layout    string `json:"layout"`
}

// Service samples the lane keys while the game is in gameplay and publishes keys.frame.
type Service struct {
	cfg     Config
	status  Status
	sampler Sampler
	events  <-chan bus.Event
	unsub   func()
	bus     *bus.Bus

	now  func() time.Time
	tick time.Duration
	gap  time.Duration
	// ticks replaces the ticker in tests.
	ticks <-chan time.Time

	name    string
	lanes   int
	scratch bool

	eng        *Engine
	mapKeys    int
	mapScratch bool
	playing    bool
	last       Frame
	lastPub    time.Time

	activity atomic.Int64
}

// NewService builds the service. When the settings are disabled the factory is never called
// and Run does nothing. An unsupported platform leaves the service idle and reported in Status.
func NewService(b *bus.Bus, c Config, newSampler func() (Sampler, error)) *Service {
	c = c.Normalized()
	s := &Service{cfg: c, bus: b, now: time.Now, tick: sampleEvery, gap: frameGap}
	s.status = Status{Enabled: c.Enabled, Supported: Supported(), Layout: c.Layout}
	if !c.Enabled {
		return s
	}
	sm, err := newSampler()
	if err != nil {
		s.status.Supported = false
		slog.Warn("lane key sampling is unavailable", "err", err)
		return s
	}
	s.status.Supported = true
	s.sampler = sm
	s.events, s.unsub = b.Subscribe()
	s.name, s.lanes, s.scratch = s.layout()
	s.eng = NewEngine(s.name, s.lanes, s.scratch)
	return s
}

// Touch records activity of the game, such as a runtime.log line, so a long play is not
// taken for a game that went silent.
func (s *Service) Touch() { s.activity.Store(s.now().UnixNano()) }

// Status returns the enabled flag, platform support and configured layout.
func (s *Service) Status() Status { return s.status }

// Run follows game state and map events until ctx is done.
func (s *Service) Run(ctx context.Context) {
	if s.sampler == nil {
		return
	}
	defer s.unsub()
	var ticker *time.Ticker
	var tickC <-chan time.Time
	stop := func() {
		if ticker != nil {
			ticker.Stop()
			ticker = nil
		}
		tickC = nil
	}
	defer stop()
	for {
		select {
		case <-ctx.Done():
			return
		case e, ok := <-s.events:
			if !ok {
				return
			}
			if e.Type != bus.TypeKeysFrame {
				s.Touch()
			}
			switch d := e.Data.(type) {
			case bus.GameState:
				if d.State == "gameplay" {
					s.playing = true
					s.restart()
					if tickC == nil {
						if s.ticks != nil {
							tickC = s.ticks
						} else {
							ticker = time.NewTicker(s.tick)
							tickC = ticker.C
						}
					}
				} else if s.playing {
					s.playing = false
					stop()
					s.publish(s.eng.Release(s.now()))
				}
			case scores.Map:
				s.mapKeys, s.mapScratch = d.Keys, d.Scratch
				s.name, s.lanes, s.scratch = s.layout()
				s.restart()
			}
		case <-tickC:
			now := s.now()
			if now.Sub(time.Unix(0, s.activity.Load())) > quietLimit {
				slog.Warn("no activity for 15 minutes in gameplay; lane key sampling paused until the game state changes")
				stop()
				s.publish(s.eng.Release(now))
				continue
			}
			f := s.eng.Update(now, s.sampler.Down(s.laneKeys()))
			if now.Sub(s.lastPub) >= s.gap && !reflect.DeepEqual(f, s.last) {
				s.publish(f)
			}
		}
	}
}

// restart clears the counters for a new run. While playing it also seeds the key states, so a
// key already held is not a fresh press, and publishes the starting frame.
func (s *Service) restart() {
	s.eng.Reset(s.name, s.lanes, s.scratch)
	if !s.playing {
		return
	}
	down := s.sampler.Down(s.laneKeys())
	s.eng.Seed(down)
	s.publish(s.eng.Update(s.now(), down))
}

func (s *Service) publish(f Frame) {
	s.last, s.lastPub = f, s.now()
	s.bus.Publish(bus.TypeKeysFrame, f)
}

// layout returns the layout name, the lane count and whether the first lane is the scratch lane.
// With the auto layout an unknown map mode gives no lane at all: nothing is sampled.
// A fixed layout always has exactly its own keys, whatever the map has.
func (s *Service) layout() (string, int, bool) {
	if l := s.cfg.Layout; l != "auto" {
		n, _ := strconv.Atoi(strings.TrimSuffix(l, "k"))
		if !validLayout(l) {
			return "unknown", 0, false
		}
		return l, n, false
	}
	if s.mapKeys < 1 || s.mapKeys > MaxKeys {
		return "unknown", 0, false
	}
	// A scratch key that is also a lane key of the layout cannot be a lane of its own: the lane list wins.
	scratch := s.mapScratch && !slices.Contains(s.cfg.Lanes(s.mapKeys), s.cfg.ScratchKey())
	lanes := s.mapKeys
	if scratch {
		lanes++
	}
	return LayoutName(s.mapKeys), lanes, scratch
}

func (s *Service) laneKeys() []Key {
	n := s.lanes
	if s.scratch {
		n--
	}
	l := s.cfg.Lanes(n)
	if s.scratch {
		return append([]Key{s.cfg.ScratchKey()}, l...)
	}
	return l
}
