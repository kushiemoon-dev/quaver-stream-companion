package session

import (
	"sync"
	"testing"
	"time"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/scores"
)

var t0 = time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

func play(acc, rating float64, failed, fc bool) scores.Result {
	return scores.Result{Score: scores.Score{Accuracy: acc, Rating: rating}, Failed: failed, FullCombo: fc}
}

func TestEmptySummary(t *testing.T) {
	s := New(t0).Summary()
	if s.Plays != 0 || s.AvgAccuracy != 0 || !s.StartedAt.Equal(t0) {
		t.Fatalf("summary = %+v", s)
	}
}

func TestStreaksAndAverages(t *testing.T) {
	st := New(t0)
	st.Saved(play(90, 10, false, false))
	st.Saved(play(100, 20, false, true))
	st.Saved(play(20, 30, true, false))
	st.Saved(play(80, 5, false, false))
	s := st.Summary()
	if s.Plays != 4 || s.Passes != 3 || s.Fails != 1 {
		t.Fatalf("counts = %+v", s)
	}
	if s.AvgAccuracy != 90 || s.BestAccuracy != 100 {
		t.Fatalf("accuracy avg %v best %v, want 90 and 100 (failed play excluded)", s.AvgAccuracy, s.BestAccuracy)
	}
	if s.BestRating != 20 || s.FullCombos != 1 {
		t.Fatalf("rating %v fc %d", s.BestRating, s.FullCombos)
	}
	if s.Streak != 1 || s.BestStreak != 2 {
		t.Fatalf("streak %d best %d", s.Streak, s.BestStreak)
	}
}

func TestOnlyFailsKeepAveragesAtZero(t *testing.T) {
	st := New(t0)
	st.Saved(play(30, 50, true, false))
	s := st.Summary()
	if s.Passes != 0 || s.AvgAccuracy != 0 || s.BestAccuracy != 0 || s.BestRating != 0 {
		t.Fatalf("summary = %+v", s)
	}
}

func TestAutoplayIsIgnored(t *testing.T) {
	st := New(t0)
	r := play(100, 99, false, true)
	r.Autoplay = true
	st.Saved(r)
	if s := st.Summary(); s.Plays != 0 || s.FullCombos != 0 {
		t.Fatalf("summary = %+v", s)
	}
}

func TestAbandonedBreaksTheStreak(t *testing.T) {
	st := New(t0)
	st.Saved(play(90, 1, false, false))
	st.Abandoned()
	st.Saved(play(90, 1, false, false))
	s := st.Summary()
	if s.Abandoned != 1 || s.Plays != 2 || s.Streak != 1 || s.BestStreak != 1 {
		t.Fatalf("summary = %+v", s)
	}
}

func TestResetClearsEverythingAndRestartsTheClock(t *testing.T) {
	st := New(t0)
	st.Saved(play(90, 1, false, true))
	st.Abandoned()
	later := t0.Add(time.Hour)
	st.Reset(later)
	s := st.Summary()
	if s.Plays != 0 || s.Abandoned != 0 || s.BestStreak != 0 || s.FullCombos != 0 || !s.StartedAt.Equal(later) {
		t.Fatalf("summary = %+v", s)
	}
}

func TestSummaryIsACopy(t *testing.T) {
	st := New(t0)
	before := st.Summary()
	st.Saved(play(90, 1, false, false))
	if before.Plays != 0 {
		t.Fatal("an earlier summary changed")
	}
}

func TestConcurrentEvents(t *testing.T) {
	st := New(t0)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				st.Saved(play(90, 1, j%5 == 0, false))
				st.Abandoned()
				_ = st.Summary()
			}
		}()
	}
	wg.Wait()
	if s := st.Summary(); s.Plays != 1600 || s.Passes+s.Fails != 1600 || s.Abandoned != 1600 {
		t.Fatalf("summary = %+v", s)
	}
}
