// Package session keeps the statistics of the current session in memory.
package session

import (
	"sync"
	"time"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/scores"
)

// Summary is the public view of the statistics.
type Summary struct {
	StartedAt    time.Time `json:"started_at"`
	Plays        int       `json:"plays"`
	Passes       int       `json:"passes"`
	Fails        int       `json:"fails"`
	Abandoned    int       `json:"abandoned"`
	AvgAccuracy  float64   `json:"avg_accuracy"`
	BestAccuracy float64   `json:"best_accuracy"`
	BestRating   float64   `json:"best_rating"`
	FullCombos   int       `json:"full_combos"`
	Streak       int       `json:"streak"`
	BestStreak   int       `json:"best_streak"`
}

// Stats counts the plays since it was created or last reset. It holds counters only, so its size is fixed.
type Stats struct {
	mu     sync.Mutex
	sum    Summary
	accSum float64
}

// New starts the statistics at now.
func New(now time.Time) *Stats {
	return &Stats{sum: Summary{StartedAt: now}}
}

// Saved counts a saved score. A failed play is a fail and stays out of the accuracy and rating figures; autoplay is ignored.
func (s *Stats) Saved(r scores.Result) {
	if r.Autoplay {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sum.Plays++
	if r.Failed {
		s.sum.Fails++
		s.sum.Streak = 0
		return
	}
	s.sum.Passes++
	s.accSum += r.Score.Accuracy
	s.sum.AvgAccuracy = s.accSum / float64(s.sum.Passes)
	s.sum.BestAccuracy = max(s.sum.BestAccuracy, r.Score.Accuracy)
	s.sum.BestRating = max(s.sum.BestRating, r.Score.Rating)
	if r.FullCombo {
		s.sum.FullCombos++
	}
	s.sum.Streak++
	s.sum.BestStreak = max(s.sum.BestStreak, s.sum.Streak)
}

// Abandoned counts a play that ended without a saved score and breaks the streak.
func (s *Stats) Abandoned() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sum.Abandoned++
	s.sum.Streak = 0
}

// Reset clears every figure and restarts the clock at now.
func (s *Stats) Reset(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sum, s.accSum = Summary{StartedAt: now}, 0
}

// Summary returns a copy of the figures.
func (s *Stats) Summary() Summary {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sum
}
