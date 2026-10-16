package state

import (
	"testing"
	"time"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/bus"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/scores"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/session"
)

func saved(failed bool) bus.Event {
	return bus.Event{Type: bus.TypeScoreSaved, Data: scores.Result{Score: scores.Score{Accuracy: 95, Rating: 12}, Failed: failed}}
}

func TestSessionStartsEmptyInTheSnapshot(t *testing.T) {
	s := New().Snapshot().Session
	if s.Plays != 0 || s.StartedAt.IsZero() {
		t.Fatalf("session = %+v", s)
	}
}

func TestSessionFollowsScoresAndEndedPlays(t *testing.T) {
	st := New()
	st.Apply(saved(false))
	st.Apply(saved(true))
	st.Apply(bus.Event{Type: bus.TypePlayEnded, Data: bus.PlayEnded{Saved: false}})
	s := st.Snapshot().Session
	if s.Plays != 2 || s.Passes != 1 || s.Fails != 1 || s.Abandoned != 1 || s.AvgAccuracy != 95 {
		t.Fatalf("session = %+v", s)
	}
}

func next(t *testing.T, ch <-chan bus.Event) bus.Event {
	t.Helper()
	select {
	case e := <-ch:
		return e
	case <-time.After(time.Second):
		t.Fatal("no event")
		return bus.Event{}
	}
}

func TestHandleSendsSessionUpdatedRightAfterScoreSaved(t *testing.T) {
	b := bus.New()
	ch, unsub := b.SubscribeFiltered(func(typ string) bool { return typ == bus.TypeScoreSaved || typ == bus.TypeSessionUpdated })
	defer unsub()
	st := New()
	e := b.Publish(bus.TypeScoreSaved, saved(false).Data)
	st.Handle(b, e)
	if got := next(t, ch); got.Type != bus.TypeScoreSaved {
		t.Fatalf("first event = %s", got.Type)
	}
	got := next(t, ch)
	sum, ok := got.Data.(session.Summary)
	if got.Type != bus.TypeSessionUpdated || !ok || sum.Plays != 1 {
		t.Fatalf("second event = %+v", got)
	}
}

func TestHandleSendsNothingForOtherEvents(t *testing.T) {
	b := bus.New()
	ch, unsub := b.Subscribe()
	defer unsub()
	New().Handle(b, bus.Event{Type: bus.TypeGameState, Data: bus.GameState{State: "menu"}})
	select {
	case e := <-ch:
		t.Fatalf("unexpected %s", e.Type)
	default:
	}
}

func TestResetSessionClearsAndAnnounces(t *testing.T) {
	b := bus.New()
	ch, unsub := b.Subscribe()
	defer unsub()
	st := New()
	st.Apply(saved(false))
	st.ResetSession(b)
	got := next(t, ch)
	sum, ok := got.Data.(session.Summary)
	if got.Type != bus.TypeSessionUpdated || !ok || sum.Plays != 0 || st.Snapshot().Session.Plays != 0 {
		t.Fatalf("event = %+v", got)
	}
}
