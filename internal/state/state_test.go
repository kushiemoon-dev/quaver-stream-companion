package state

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/bus"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/keys"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/scores"
)

func TestInitialSnapshot(t *testing.T) {
	s := New().Snapshot()
	if s.Game != "unknown" || s.Map != nil || s.LastScore != nil {
		t.Fatalf("snapshot = %+v", s)
	}
}

func TestApplyUpdatesSnapshot(t *testing.T) {
	st := New()
	st.Apply(bus.Event{Type: bus.TypeGameState, Data: bus.GameState{State: "gameplay"}})
	st.Apply(bus.Event{Type: bus.TypeMapStarted, Data: scores.Map{Title: "T"}})
	st.Apply(bus.Event{Type: bus.TypeScoreSaved, Data: scores.Result{FullCombo: true}})

	s := st.Snapshot()
	if s.Game != "gameplay" || s.Map == nil || s.Map.Title != "T" || s.LastScore == nil || !s.LastScore.FullCombo {
		t.Fatalf("snapshot = %+v", s)
	}
}

func TestSnapshotIsNotAffectedByLaterEvents(t *testing.T) {
	st := New()
	st.Apply(bus.Event{Type: bus.TypeMapStarted, Data: scores.Map{Title: "A"}})
	before := st.Snapshot()
	st.Apply(bus.Event{Type: bus.TypeMapStarted, Data: scores.Map{Title: "B"}})
	if before.Map.Title != "A" {
		t.Fatalf("earlier snapshot changed: %+v", before.Map)
	}
}

func TestRunAppliesBusEvents(t *testing.T) {
	b := bus.New()
	st := New()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go st.Run(ctx, b)
	time.Sleep(30 * time.Millisecond)

	b.Publish(bus.TypeGameState, bus.GameState{State: "results"})

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if st.Snapshot().Game == "results" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("state never reached results")
}

func TestSnapshotKeysIsNullUntilAFrameArrives(t *testing.T) {
	st := New()
	raw, err := json.Marshal(st.Snapshot())
	if err != nil || !strings.Contains(string(raw), `"keys":null`) {
		t.Fatalf("json = %s err = %v", raw, err)
	}
	f := keys.Frame{Layout: "4k", Lanes: 4, Down: []bool{true, false, false, false}, Presses: []int{1, 0, 0, 0}, KPS: 1, Peak: 1}
	st.Apply(bus.Event{Type: bus.TypeKeysFrame, Data: f})
	s := st.Snapshot()
	if s.Keys == nil || s.Keys.KPS != 1 || s.Keys.Layout != "4k" {
		t.Fatalf("snapshot = %+v", s)
	}
	raw, _ = json.Marshal(s)
	var back struct {
		Keys map[string]any `json:"keys"`
	}
	if err := json.Unmarshal(raw, &back); err != nil || len(back.Keys) != 6 {
		t.Fatalf("keys shape = %v err = %v", back.Keys, err)
	}
}
