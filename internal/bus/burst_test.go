package bus

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// reader drains a subscription like an SSE handler and records what it saw.
type reader struct {
	mu   sync.Mutex
	got  []int
	seen atomic.Int64
	done chan struct{}
}

func startReader(ch <-chan Event) *reader {
	r := &reader{done: make(chan struct{})}
	go func() {
		defer close(r.done)
		for e := range ch {
			r.mu.Lock()
			r.got = append(r.got, e.Data.(int))
			r.mu.Unlock()
			r.seen.Add(1)
		}
	}()
	return r
}

// pace keeps the publisher at most 100 events ahead of the readers, as a healthy SSE client would
// keep up; a scheduler hiccup must not look like a stuck reader.
func pace(published int, readers ...*reader) {
	for _, r := range readers {
		for int64(published)-r.seen.Load() > 100 {
			runtime.Gosched()
		}
	}
}

func mixedType(i int) string {
	return [...]string{TypeGameState, TypeMapStarted, TypeGameState, TypeScoreSaved, TypeKeysFrame}[i%5]
}

func TestReaderDuringABurstSeesEveryEventInOrder(t *testing.T) {
	b := New()
	ch, unsub := b.Subscribe()
	r := startReader(ch)

	for i := 0; i < 3000; i++ {
		b.Publish(mixedType(i), i)
		pace(i+1, r)
	}
	if b.Subscribers() != 1 || b.Dropped() != 0 {
		t.Fatalf("subscribers = %d, dropped = %d: a reader that keeps reading must stay", b.Subscribers(), b.Dropped())
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		r.mu.Lock()
		n := len(r.got)
		r.mu.Unlock()
		if n == 3000 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	unsub()
	<-r.done
	if len(r.got) != 3000 {
		t.Fatalf("delivered %d of 3000", len(r.got))
	}
	for i, v := range r.got {
		if v != i {
			t.Fatalf("event %d arrived at position %d: order was not kept", v, i)
		}
	}
}

func TestManyLiveReadersSurviveAScreenLineBurst(t *testing.T) {
	b := New()
	var readers []*reader
	for range 50 {
		ch, unsub := b.Subscribe()
		defer unsub()
		readers = append(readers, startReader(ch))
	}
	for i := 0; i < 4000; i++ {
		b.Publish(TypeGameState, i)
		pace(i+1, readers...)
	}
	if b.Subscribers() != 50 || b.Dropped() != 0 {
		t.Fatalf("subscribers = %d, dropped = %d after the burst", b.Subscribers(), b.Dropped())
	}
}

func TestOnlyAStuckReaderIsClosedDuringABurst(t *testing.T) {
	b := New()
	live, unsubLive := b.Subscribe()
	defer unsubLive()
	r := startReader(live)
	stuck, unsubStuck := b.Subscribe()
	defer unsubStuck()

	for i := 0; i < 3000; i++ {
		b.Publish(mixedType(i), i)
		pace(i+1, r)
	}
	if b.Subscribers() != 1 || b.Dropped() != 1 {
		t.Fatalf("subscribers = %d, dropped = %d: only the stuck reader should go", b.Subscribers(), b.Dropped())
	}
	n := 0
	for range stuck {
		n++
	}
	if n != subscriberQueue {
		t.Fatalf("the stuck reader was handed %d queued events, want %d", n, subscriberQueue)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := 1; i < len(r.got); i++ {
		if r.got[i] != r.got[i-1]+1 {
			t.Fatalf("live reader order broken: %s", fmt.Sprint(r.got[i-1], r.got[i]))
		}
	}
}

func TestLaggingReaderNeverSeesEventsOutOfOrder(t *testing.T) {
	kinds := [...]string{TypeGameState, TypeGameState, TypeMapStarted, TypeGameState, TypeScoreSaved}
	for run := 0; run < 40; run++ {
		b := New()
		ch, unsub := b.Subscribe()
		last, bad := -1, -1
		done := make(chan struct{})
		go func() {
			defer close(done)
			for e := range ch {
				v := e.Data.(int)
				if v <= last && bad < 0 {
					bad = v
				}
				last = v
				if v%3 == 0 {
					time.Sleep(20 * time.Microsecond)
				}
			}
		}()
		for i := 0; i < 3000; i++ {
			b.Publish(kinds[i%len(kinds)], i)
		}
		unsub()
		<-done
		if bad >= 0 {
			t.Fatalf("run %d: event %d was delivered after a later one", run, bad)
		}
	}
}
