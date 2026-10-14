package bus

import (
	"encoding/json"
	"testing"
	"time"
)

func recv(t *testing.T, ch <-chan Event) Event {
	t.Helper()
	select {
	case e, ok := <-ch:
		if !ok {
			t.Fatal("channel closed")
		}
		return e
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
		return Event{}
	}
}

func TestPublishReachesSubscriber(t *testing.T) {
	b := New()
	ch, unsub := b.Subscribe()
	defer unsub()

	b.Publish(TypeGameState, GameState{State: "menu"})

	e := recv(t, ch)
	if e.Type != TypeGameState || e.V != SchemaVersion {
		t.Fatalf("got %+v", e)
	}
	if gs, ok := e.Data.(GameState); !ok || gs.State != "menu" {
		t.Fatalf("data = %#v", e.Data)
	}
}

func TestUnsubscribeClosesChannel(t *testing.T) {
	b := New()
	ch, unsub := b.Subscribe()
	unsub()
	if _, ok := <-ch; ok {
		t.Fatal("channel should be closed after unsubscribe")
	}
	unsub() // second call must not panic
}

func TestSlowSubscriberDoesNotBlockPublisher(t *testing.T) {
	b := New()
	_, unsub := b.Subscribe() // never read
	defer unsub()

	done := make(chan struct{})
	go func() {
		for i := 0; i < 500; i++ {
			b.Publish(TypeGameState, GameState{State: "menu"})
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Publish blocked on a slow subscriber")
	}
}

func TestEventJSONShape(t *testing.T) {
	b := New()
	e := b.Publish(TypeGameState, GameState{State: "select"})
	raw, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got["v"].(float64) != 1 || got["type"] != "game.state" {
		t.Fatalf("envelope = %s", raw)
	}
	data := got["data"].(map[string]any)
	if data["state"] != "select" {
		t.Fatalf("data = %v", data)
	}
	if _, ok := got["time"].(string); !ok {
		t.Fatalf("time missing in %s", raw)
	}
}

func TestFilteredSubscriberNeverQueuesRejectedTypes(t *testing.T) {
	b := New()
	ch, unsub := b.SubscribeFiltered(func(typ string) bool { return typ != TypeKeysFrame })
	defer unsub()
	for i := 0; i < 3*subscriberQueue; i++ {
		b.Publish(TypeKeysFrame, i)
	}
	b.Publish(TypeMapStarted, "m")
	if e := recv(t, ch); e.Type != TypeMapStarted {
		t.Fatalf("got %+v, want map.started (rejected frames must not fill the queue)", e)
	}
}

func TestFullQueueClosesTheSubscriberAndCountsTheDrop(t *testing.T) {
	b := New()
	ch, unsub := b.Subscribe() // never read
	defer unsub()
	for i := 0; i < 2*subscriberQueue; i++ {
		b.Publish(TypeMapStarted, i)
	}
	if got := b.Dropped(); got != 1 {
		t.Fatalf("Dropped = %d, want 1", got)
	}
	if b.Subscribers() != 0 {
		t.Fatalf("Subscribers = %d, the overflowing one must be removed", b.Subscribers())
	}
	n := 0
	for range ch {
		n++
	}
	if n < subscriberQueue {
		t.Fatalf("drained %d queued events before the close, want at least %d", n, subscriberQueue)
	}
	unsub() // still safe
}

func TestHealthySubscriberIsNeverClosed(t *testing.T) {
	b := New()
	ch, unsub := b.Subscribe()
	defer unsub()
	for i := 0; i < 10*subscriberQueue; i++ {
		b.Publish(TypeGameState, GameState{State: "menu"})
		<-ch
	}
	if b.Dropped() != 0 || b.Subscribers() != 1 {
		t.Fatalf("dropped=%d subscribers=%d", b.Dropped(), b.Subscribers())
	}
}
