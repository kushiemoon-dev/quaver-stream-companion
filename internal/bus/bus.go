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

type Bus struct {
	mu   sync.Mutex
	next int
	subs map[int]chan Event
}

func New() *Bus {
	return &Bus{subs: map[int]chan Event{}}
}

// Subscribe returns a channel of events and a function that unsubscribes
// and closes it. The unsubscribe function is safe to call more than once.
func (b *Bus) Subscribe() (<-chan Event, func()) {
	b.mu.Lock()
	defer b.mu.Unlock()
	id := b.next
	b.next++
	ch := make(chan Event, subscriberQueue)
	b.subs[id] = ch
	return ch, func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if c, ok := b.subs[id]; ok {
			delete(b.subs, id)
			close(c)
		}
	}
}

// Publish delivers an event to every subscriber. A subscriber whose queue is
// full misses the event rather than blocking the publisher.
func (b *Bus) Publish(typ string, data any) Event {
	e := Event{V: SchemaVersion, Type: typ, Time: time.Now().UTC(), Data: data}
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, ch := range b.subs {
		select {
		case ch <- e:
		default:
		}
	}
	return e
}
