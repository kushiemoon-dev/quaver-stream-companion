package scores

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/testutil"
)

func TestKeysForEveryMode(t *testing.T) {
	want := map[int]int{1: 4, 2: 7, 3: 1, 4: 2, 5: 3, 6: 5, 7: 6, 8: 8, 9: 9, 10: 10}
	for mode, keys := range want {
		if got := keysFor(mode); got != keys {
			t.Errorf("keysFor(%d) = %d, want %d", mode, got, keys)
		}
	}
}

func TestKeysForUnknownModeIsZero(t *testing.T) {
	for _, mode := range []int{0, -1, 11, 12, 100, math.MaxInt32, math.MinInt32, math.MaxInt, math.MinInt} {
		if got := keysFor(mode); got != 0 {
			t.Errorf("keysFor(%d) = %d, want 0", mode, got)
		}
	}
}

func TestLastPlayedMapReportsKeysAndScratch(t *testing.T) {
	cases := []struct {
		name    string
		mode    int
		scratch bool
		keys    int
		want    bool
	}{
		{"4k", 1, false, 4, false},
		{"7k plus one", 2, true, 7, true},
		{"10k", 10, false, 10, false},
		{"10k plus one", 10, true, 10, true},
		{"unknown mode ignores the flag", 99, true, 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := testutil.NewQuaverDir(t)
			d.AddMap(t, testutil.MapRow{MD5: "m", Mode: c.mode, Scratch: c.scratch, LastPlayedMS: 1})
			m, err := open(t, d).LastPlayedMap(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if m.Keys != c.keys || m.Scratch != c.want {
				t.Fatalf("keys=%d scratch=%v, want %d %v", m.Keys, m.Scratch, c.keys, c.want)
			}
		})
	}
}

func TestScoreAndRecentCarryScratch(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	d.AddMap(t, testutil.MapRow{MD5: "m", Mode: 2, Scratch: true, LastPlayedMS: 1})
	id := d.AddScore(t, testutil.ScoreRow{MapMD5: "m", Total: 1, Grade: 1})
	st := open(t, d)
	r, err := st.ScoreByID(context.Background(), id)
	if err != nil || r.Map.Keys != 7 || !r.Map.Scratch {
		t.Fatalf("score map = %+v, err %v", r.Map, err)
	}
	list, err := st.Recent(context.Background(), 5)
	if err != nil || len(list) != 1 || list[0].Map.Keys != 7 || !list[0].Map.Scratch {
		t.Fatalf("recent = %+v, err %v", list, err)
	}
}

func TestMissingScratchColumnMeansNoScratch(t *testing.T) {
	d := testutil.NewNoScratchQuaverDir(t)
	d.AddMap(t, testutil.MapRow{MD5: "m", Mode: 2, LastPlayedMS: 1})
	id := d.AddScore(t, testutil.ScoreRow{MapMD5: "m", Total: 1, Grade: 1})
	st := open(t, d)
	m, err := st.LastPlayedMap(context.Background())
	if err != nil || m.Keys != 7 || m.Scratch {
		t.Fatalf("map = %+v, err %v", m, err)
	}
	if r, err := st.ScoreByID(context.Background(), id); err != nil || r.Map.Keys != 7 {
		t.Fatalf("score = %+v, err %v", r.Map, err)
	}
}

func TestScratchFlagRetriesAFailedCheckOncePerMinute(t *testing.T) {
	calls := 0
	fail := true
	f := &scratchFlag{probe: func(*sql.DB) (string, error) {
		calls++
		if fail {
			return "", errors.New("database is locked")
		}
		return "COALESCE(m.HasScratchKey,0)", nil
	}}
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	if got := f.expr(nil, now); got != "0" || calls != 1 {
		t.Fatalf("first check: %q after %d calls", got, calls)
	}
	if got := f.expr(nil, now.Add(59*time.Second)); got != "0" || calls != 1 {
		t.Fatalf("within the minute: %q after %d calls", got, calls)
	}
	fail = false
	if got := f.expr(nil, now.Add(time.Minute)); got != "COALESCE(m.HasScratchKey,0)" || calls != 2 {
		t.Fatalf("after the minute: %q after %d calls", got, calls)
	}
	fail = true
	if got := f.expr(nil, now.Add(10*time.Hour)); got != "COALESCE(m.HasScratchKey,0)" || calls != 2 {
		t.Fatalf("a known answer is final: %q after %d calls", got, calls)
	}
}

func TestScratchFlagTreatsAMissingColumnAsFinal(t *testing.T) {
	calls := 0
	f := &scratchFlag{probe: func(*sql.DB) (string, error) { calls++; return "0", nil }}
	now := time.Now()
	for i := 0; i < 3; i++ {
		if got := f.expr(nil, now.Add(time.Duration(i)*time.Hour)); got != "0" {
			t.Fatalf("got %q", got)
		}
	}
	if calls != 1 {
		t.Fatalf("probed %d times", calls)
	}
}

func TestStoreReadsTheScratchFlagAfterATransientCheckFailure(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	d.AddMap(t, testutil.MapRow{MD5: "m", Mode: 2, Scratch: true, LastPlayedMS: 1})
	st := open(t, d)
	clock := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	st.now = func() time.Time { return clock }
	st.scratch = scratchFlag{probe: func(*sql.DB) (string, error) { return "", errors.New("database is locked") }}
	if m, err := st.LastPlayedMap(context.Background()); err != nil || m.Scratch {
		t.Fatalf("while the check fails: %+v, %v", m, err)
	}
	st.scratch.probe = scratchColumn
	clock = clock.Add(2 * time.Minute)
	if m, err := st.LastPlayedMap(context.Background()); err != nil || !m.Scratch || m.Keys != 7 {
		t.Fatalf("after the retry: %+v, %v", m, err)
	}
}

func TestUnknownModeNeverHasAScratchLane(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	d.AddMap(t, testutil.MapRow{MD5: "m", Mode: 99, Scratch: true, LastPlayedMS: 1})
	id := d.AddScore(t, testutil.ScoreRow{MapMD5: "m", Total: 1, Grade: 1})
	st := open(t, d)
	r, err := st.ScoreByID(context.Background(), id)
	if err != nil || r.Map.Keys != 0 || r.Map.Scratch {
		t.Fatalf("score map = %+v, err %v", r.Map, err)
	}
	list, err := st.Recent(context.Background(), 5)
	if err != nil || len(list) != 1 || list[0].Map.Keys != 0 || list[0].Map.Scratch {
		t.Fatalf("recent = %+v, err %v", list, err)
	}
}
