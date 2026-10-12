package keys

import (
	"reflect"
	"testing"
	"time"
)

var t0 = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

func at(ms int) time.Time { return t0.Add(time.Duration(ms) * time.Millisecond) }

func TestEngineCountsPressEdgesOnly(t *testing.T) {
	e := NewEngine("4k", 4)
	e.Update(at(0), []bool{true, false, false, false})
	e.Update(at(10), []bool{true, false, false, false})
	f := e.Update(at(20), []bool{true, false, false, false})
	if !reflect.DeepEqual(f.Presses, []int{1, 0, 0, 0}) || f.KPS != 1 {
		t.Fatalf("held key counted more than once: %+v", f)
	}
	e.Update(at(30), []bool{false, false, false, false})
	f = e.Update(at(40), []bool{true, false, false, false})
	if f.Presses[0] != 2 || f.KPS != 2 {
		t.Fatalf("release then press must count again: %+v", f)
	}
}

func TestEngineKeyAlreadyDownAtStartCountsAsPress(t *testing.T) {
	f := NewEngine("4k", 4).Update(at(0), []bool{false, true, false, false})
	if f.Presses[1] != 1 || !f.Down[1] {
		t.Fatalf("frame = %+v", f)
	}
}

func TestEngineKPSSlidingWindow(t *testing.T) {
	e := NewEngine("4k", 4)
	up := []bool{false, false, false, false}
	press := func(ms int) Frame {
		e.Update(at(ms), []bool{true, false, false, false})
		return e.Update(at(ms+1), up)
	}
	press(0)
	press(100)
	f := press(200)
	if f.KPS != 3 {
		t.Fatalf("kps = %d, want 3", f.KPS)
	}
	// A press exactly one second old has left the window.
	if f = e.Update(at(1000), up); f.KPS != 2 {
		t.Fatalf("kps at +1000 = %d, want 2", f.KPS)
	}
	if f = e.Update(at(1099), up); f.KPS != 2 {
		t.Fatalf("kps at +1099 = %d, want 2", f.KPS)
	}
	if f = e.Update(at(1100), up); f.KPS != 1 {
		t.Fatalf("kps at +1100 = %d, want 1", f.KPS)
	}
	if f = e.Update(at(1200), up); f.KPS != 0 {
		t.Fatalf("kps at +1200 = %d, want 0", f.KPS)
	}
	if f.Peak != 3 {
		t.Fatalf("peak = %d, want 3", f.Peak)
	}
}

func TestEngineBurstAcrossLanes(t *testing.T) {
	e := NewEngine("7k", 7)
	f := e.Update(at(0), []bool{true, true, true, true, true, true, true})
	if f.KPS != 7 || f.Peak != 7 || f.Lanes != 7 || f.Layout != "7k" {
		t.Fatalf("frame = %+v", f)
	}
}

func TestEnginePeakSurvivesKPSDropAndResetClearsEverything(t *testing.T) {
	e := NewEngine("4k", 4)
	e.Update(at(0), []bool{true, true, false, false})
	f := e.Update(at(5000), []bool{false, false, false, false})
	if f.KPS != 0 || f.Peak != 2 {
		t.Fatalf("frame = %+v", f)
	}
	e.Reset("7k", 7)
	f = e.Update(at(5001), make([]bool, 7))
	if f.Peak != 0 || f.KPS != 0 || len(f.Presses) != 7 || f.Layout != "7k" || f.Lanes != 7 {
		t.Fatalf("after reset: %+v", f)
	}
}

func TestEngineReleaseGivesAllUpFrameKeepingTotals(t *testing.T) {
	e := NewEngine("4k", 4)
	e.Update(at(0), []bool{true, false, true, false})
	f := e.Release(at(50))
	if !reflect.DeepEqual(f.Down, []bool{false, false, false, false}) || !reflect.DeepEqual(f.Presses, []int{1, 0, 1, 0}) || f.Peak != 2 {
		t.Fatalf("frame = %+v", f)
	}
}

func TestEngineFrameIsACopy(t *testing.T) {
	e := NewEngine("4k", 4)
	f := e.Update(at(0), []bool{true, false, false, false})
	f.Presses[0] = 99
	f.Down[0] = false
	g := e.Update(at(1), []bool{true, false, false, false})
	if g.Presses[0] != 1 || !g.Down[0] {
		t.Fatalf("engine state aliased: %+v", g)
	}
}

func TestEngineShortSamplerAnswerMeansUp(t *testing.T) {
	f := NewEngine("4k", 4).Update(at(0), []bool{true})
	if len(f.Down) != 4 || f.Presses[0] != 1 {
		t.Fatalf("frame = %+v", f)
	}
}
