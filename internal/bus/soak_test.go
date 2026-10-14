package bus

import (
	"runtime"
	"sync"
	"testing"
	"time"
)

// TestSoakReconnectingSubscribersLeaveNoGoroutines publishes 30 simulated minutes of events,
// one batch per simulated second, while subscribers keep connecting and leaving, and some
// never read so that the bus has to cut them off.
func TestSoakReconnectingSubscribersLeaveNoGoroutines(t *testing.T) {
	if testing.Short() {
		t.Skip("soak test")
	}
	b := New()
	time.Sleep(20 * time.Millisecond)
	baseline := runtime.NumGoroutine()

	const seconds = 30 * 60
	stop := make(chan struct{})
	var wg sync.WaitGroup
	connect := func(read int) {
		ch, unsub := b.Subscribe()
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer unsub()
			if read == 0 {
				<-stop // a page that never reads
				return
			}
			for i := 0; read < 0 || i < read; i++ {
				select {
				case _, ok := <-ch:
					if !ok {
						return
					}
				case <-stop:
					return
				}
			}
		}()
	}

	for s := 0; s < seconds; s++ {
		switch s % 4 {
		case 0:
			connect(3) // a page that reloads soon
		case 1:
			connect(0) // a page that never reads
		case 2:
			connect(100) // a page that reads for a while
		}
		b.Publish(TypeGameState, GameState{State: "gameplay"})
		for range 30 {
			b.Publish(TypeKeysFrame, struct{}{})
		}
		b.Publish(TypeScoreSaved, struct{}{})
	}
	if b.Dropped() == 0 {
		t.Fatal("the subscribers that never read should have been dropped")
	}

	close(stop)
	wg.Wait()
	deadline := time.Now().Add(10 * time.Second)
	for runtime.NumGoroutine() > baseline && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := runtime.NumGoroutine(); got > baseline {
		t.Fatalf("goroutines = %d, baseline %d", got, baseline)
	}
	if n := b.Subscribers(); n != 0 {
		t.Fatalf("%d subscribers left on the bus", n)
	}
}
