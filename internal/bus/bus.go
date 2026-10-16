package bus

import (
	"log/slog"
	"sync"
	"time"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/scores"
)

const (
	SchemaVersion = 1

	TypeSnapshot       = "snapshot"
	TypeGameState      = "game.state"
	TypeMapStarted     = "map.started"
	TypeScoreSaved     = "score.saved"
	TypePlayEnded      = "play.ended"
	TypeSessionUpdated = "session.updated"
	TypeKeysFrame      = "keys.frame"
	subscriberQueue    = 256
)

type Event struct {
	V    int       `json:"v"`
	Type string    `json:"type"`
	Time time.Time `json:"time"`
	Data any       `json:"data"`
}

// GameState is the data of a game.state event. Screen is the raw lower case Quaver screen name,
// empty when the state does not come from a screen (closed).
type GameState struct {
	State  string `json:"state"`
	Screen string `json:"screen,omitempty"`
}

// PlayEnded is the data of a play.ended event: a play that left gameplay without a saved score.
type PlayEnded struct {
	Map   scores.Map `json:"map"`
	Saved bool       `json:"saved"`
}

type subscriber struct {
	ch     chan Event
	accept func(typ string) bool
}

type Bus struct {
	mu   sync.Mutex
	next int
	subs map[int]subscriber

	dropped uint64
}

func New() *Bus {
	return &Bus{subs: map[int]subscriber{}}
}

// Subscribe returns a channel of events and a function that unsubscribes
// and closes it. The unsubscribe function is safe to call more than once.
func (b *Bus) Subscribe() (<-chan Event, func()) {
	return b.SubscribeFiltered(nil)
}

// SubscribeFiltered is Subscribe for a subscriber that only wants the event types accept
// returns true for; the others never enter its queue. A nil accept takes everything.
func (b *Bus) SubscribeFiltered(accept func(typ string) bool) (<-chan Event, func()) {
	b.mu.Lock()
	defer b.mu.Unlock()
	id := b.next
	b.next++
	ch := make(chan Event, subscriberQueue)
	b.subs[id] = subscriber{ch: ch, accept: accept}
	return ch, func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if s, ok := b.subs[id]; ok {
			delete(b.subs, id)
			close(s.ch)
		}
	}
}

// Publish delivers an event to every subscriber. A subscriber whose queue is
// full is removed and its channel closed instead of blocking the publisher, so
// its reader notices and reconnects for a fresh snapshot.
func (b *Bus) Publish(typ string, data any) Event {
	e := Event{V: SchemaVersion, Type: typ, Time: time.Now().UTC(), Data: data}
	b.mu.Lock()
	defer b.mu.Unlock()
	for id, s := range b.subs {
		if s.accept != nil && !s.accept(typ) {
			continue
		}
		select {
		case s.ch <- e:
		default:
			b.dropped++
			delete(b.subs, id)
			close(s.ch)
			slog.Warn("event subscriber closed: queue full", "type", typ, "dropped_total", b.dropped)
		}
	}
	return e
}

// Dropped returns how many subscribers were closed because their queue was full.
func (b *Bus) Dropped() uint64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.dropped
}

// Subscribers returns the number of live subscribers.
func (b *Bus) Subscribers() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.subs)
}
