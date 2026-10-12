package bus

import (
	"sync"
	"time"
)

const (
	SchemaVersion = 1

	TypeSnapshot    = "snapshot"
	TypeGameState   = "game.state"
	TypeMapStarted  = "map.started"
	TypeScoreSaved  = "score.saved"
	TypeKeysFrame   = "keys.frame"
	subscriberQueue = 64
)

type Event struct {
	V    int       `json:"v"`
	Type string    `json:"type"`
	Time time.Time `json:"time"`
	Data any       `json:"data"`
}

type GameState struct {
	State string `json:"state"`
}

type subscriber struct {
	ch     chan Event
	accept func(typ string) bool
}

type Bus struct {
	mu   sync.Mutex
	next int
	subs map[int]subscriber
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
// full misses the event rather than blocking the publisher.
func (b *Bus) Publish(typ string, data any) Event {
	e := Event{V: SchemaVersion, Type: typ, Time: time.Now().UTC(), Data: data}
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, s := range b.subs {
		if s.accept != nil && !s.accept(typ) {
			continue
		}
		select {
		case s.ch <- e:
		default:
		}
	}
	return e
}
