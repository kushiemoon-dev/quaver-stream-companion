package keys

import "time"

const kpsWindow = time.Second

// Frame is the state of the lanes at one instant, as published on keys.frame.
type Frame struct {
	Layout  string `json:"layout"`
	Lanes   int    `json:"lanes"`
	Down    []bool `json:"down"`
	Presses []int  `json:"presses"`
	KPS     int    `json:"kps"`
	Peak    int    `json:"peak"`
}

// Engine turns successive key states into press counters, KPS and the peak KPS of the map.
// It is pure: the caller supplies the time and the key states.
type Engine struct {
	layout  string
	lanes   int
	prev    []bool
	presses []int
	stamps  []time.Time
	peak    int
}

// NewEngine returns an engine for the given layout name and lane count.
func NewEngine(layout string, lanes int) *Engine {
	e := &Engine{}
	e.Reset(layout, lanes)
	return e
}

// Reset clears every counter and switches layout.
func (e *Engine) Reset(layout string, lanes int) {
	*e = Engine{layout: layout, lanes: lanes, prev: make([]bool, lanes), presses: make([]int, lanes)}
}

// Update folds the key states seen at now into the counters and returns the new frame.
// A missing entry in down means the key is up.
func (e *Engine) Update(now time.Time, down []bool) Frame {
	cur := make([]bool, e.lanes)
	copy(cur, down)
	for i, d := range cur {
		if d && !e.prev[i] {
			e.presses[i]++
			e.stamps = append(e.stamps, now)
		}
	}
	e.prev = cur
	return e.frame(now)
}

// Release returns the frame for a moment where every key is up, keeping the totals.
func (e *Engine) Release(now time.Time) Frame {
	return e.Update(now, nil)
}

func (e *Engine) frame(now time.Time) Frame {
	n := 0
	for n < len(e.stamps) && now.Sub(e.stamps[n]) >= kpsWindow {
		n++
	}
	e.stamps = e.stamps[n:]
	kps := len(e.stamps)
	e.peak = max(e.peak, kps)
	return Frame{
		Layout: e.layout, Lanes: e.lanes,
		Down:    append([]bool(nil), e.prev...),
		Presses: append([]int(nil), e.presses...),
		KPS:     kps, Peak: e.peak,
	}
}

// Seed records the key states without counting them as presses, for keys already held
// when a map starts.
func (e *Engine) Seed(down []bool) {
	cur := make([]bool, e.lanes)
	copy(cur, down)
	e.prev = cur
}
