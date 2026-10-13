package scores

import (
	"context"
	"encoding/base64"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/testutil"
)

func TestDecodeBreakdownValid(t *testing.T) {
	got, err := DecodeBreakdown(testutil.EncodeBreakdown("0012345"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "\x00\x00\x01\x02\x03\x04\x05" {
		t.Fatalf("got %v", got)
	}
}

func TestDecodeBreakdownRejectsBadData(t *testing.T) {
	good := testutil.EncodeBreakdown("0123")
	raw, _ := base64.StdEncoding.DecodeString(good)
	wrongLen := append([]byte{9, 0, 0, 0}, raw[4:]...)
	overCap := append([]byte{0xA1, 0x86, 0x01, 0x00}, raw[4:]...)
	oversize := strings.Repeat("0", maxBreakdown+1)
	for name, in := range map[string]string{
		"empty":        "",
		"garbage":      "!!!not base64!!!",
		"short header": base64.StdEncoding.EncodeToString([]byte{1, 0}),
		"truncated":    base64.StdEncoding.EncodeToString(raw[:len(raw)-5]),
		"wrong length": base64.StdEncoding.EncodeToString(wrongLen),
		"length cap":   base64.StdEncoding.EncodeToString(overCap),
		"not gzip":     base64.StdEncoding.EncodeToString(append([]byte{4, 0, 0, 0}, "0123"...)),
		"wrong digits": testutil.EncodeBreakdown("0126"),
		"letters":      testutil.EncodeBreakdown("01a3"),
		"oversize":     testutil.EncodeBreakdown(oversize),
		"zero length":  testutil.EncodeBreakdown(""),
	} {
		if got, err := DecodeBreakdown(in); err == nil {
			t.Errorf("%s: want an error, got %v", name, got)
		}
	}
}

func TestBuildTimelineSeries(t *testing.T) {
	// Marv, Perf, Great, Good, Okay, Miss in order: cumulative weights 100, 198.25, 263.25, 288.25, 188.25, 138.25
	tl := BuildTimeline(7, []byte{0, 1, 2, 3, 4, 5})
	if tl.ScoreID != 7 || tl.Total != 6 {
		t.Fatalf("timeline = %+v", tl)
	}
	want := []float64{100, 99.13, 87.75, 72.06, 37.65, 23.04}
	if len(tl.Points) != len(want) {
		t.Fatalf("points = %v", tl.Points)
	}
	for i := range want {
		if math.Abs(tl.Points[i]-want[i]) > 0.01 {
			t.Errorf("point %d = %v, want %v", i, tl.Points[i], want[i])
		}
	}
	if len(tl.Misses) != 1 || tl.Misses[0] != 5 {
		t.Errorf("misses = %v", tl.Misses)
	}
	if len(tl.Counts) != 6 || tl.Counts[5] != [6]int{0, 0, 0, 0, 0, 1} {
		t.Errorf("counts = %v", tl.Counts)
	}
}

func TestBuildTimelineFloorsAtZero(t *testing.T) {
	tl := BuildTimeline(1, []byte{4, 4, 5, 0})
	for i, p := range tl.Points {
		if p < 0 || p > 100 {
			t.Errorf("point %d = %v out of range", i, p)
		}
	}
	if tl.Points[0] != 0 {
		t.Errorf("first point = %v, want 0", tl.Points[0])
	}
}

func TestBuildTimelineFinalMatchesStoredFormula(t *testing.T) {
	seq := make([]byte, 0, 1000)
	for i := 0; i < 1000; i++ {
		seq = append(seq, []byte{0, 0, 0, 1, 1, 2, 3, 0, 1, 5}[i%10])
	}
	tl := BuildTimeline(1, seq)
	sum := 0.0
	for _, d := range seq {
		sum += weights[d]
	}
	if got, want := tl.Points[len(tl.Points)-1], math.Round(sum/float64(len(seq))*100)/100; got != want {
		t.Errorf("final = %v, want %v", got, want)
	}
}

func TestBuildTimelineDownsamplesWithinBounds(t *testing.T) {
	for _, n := range []int{1, 7, 199, 200, 201, 5000, maxBreakdown} {
		seq := make([]byte, n)
		for i := range seq {
			seq[i] = byte(i % 6)
		}
		tl := BuildTimeline(1, seq)
		if len(tl.Points) > maxPoints || len(tl.Points) == 0 || len(tl.Points) != len(tl.Counts) {
			t.Fatalf("n=%d: %d points, %d counts", n, len(tl.Points), len(tl.Counts))
		}
		if len(tl.Misses) > maxMisses {
			t.Fatalf("n=%d: %d misses", n, len(tl.Misses))
		}
		var total [6]int
		for _, c := range tl.Counts {
			for k, v := range c {
				total[k] += v
			}
		}
		sum := 0
		for _, v := range total {
			sum += v
		}
		if sum != n || tl.Total != n {
			t.Fatalf("n=%d: counts sum to %d, total %d", n, sum, tl.Total)
		}
	}
}

func TestBuildTimelineMissIndexesAreCappedAndOrdered(t *testing.T) {
	seq := make([]byte, 3000)
	for i := range seq {
		seq[i] = 5
	}
	tl := BuildTimeline(1, seq)
	if len(tl.Misses) != maxMisses {
		t.Fatalf("misses = %d, want %d", len(tl.Misses), maxMisses)
	}
	for i, m := range tl.Misses {
		if m < 0 || m >= len(seq) || (i > 0 && m <= tl.Misses[i-1]) {
			t.Fatalf("miss %d = %d", i, m)
		}
	}
}

func TestStoreTimeline(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	d.AddMap(t, testutil.MapRow{MD5: "m1", Mode: 1})
	ok := d.AddScore(t, testutil.ScoreRow{MapMD5: "m1", Breakdown: testutil.EncodeBreakdown("00015")})
	none := d.AddScore(t, testutil.ScoreRow{MapMD5: "m1"})
	bad := d.AddScore(t, testutil.ScoreRow{MapMD5: "m1", Breakdown: "garbage"})
	st := open(t, d)
	ctx := context.Background()

	tl, err := st.Timeline(ctx, ok)
	if err != nil || tl.ScoreID != ok || tl.Total != 5 || len(tl.Misses) != 1 {
		t.Fatalf("timeline = %+v, err %v", tl, err)
	}
	for _, id := range []int64{none, bad} {
		if _, err := st.Timeline(ctx, id); !errors.Is(err, ErrNoTimeline) {
			t.Errorf("score %d: err = %v, want ErrNoTimeline", id, err)
		}
	}
	if _, err := st.Timeline(ctx, 999); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown score: err = %v", err)
	}
}

func TestStoreTimelineWithoutBreakdownColumn(t *testing.T) {
	d := testutil.NewLegacyQuaverDir(t)
	d.Exec(t, `INSERT INTO Score (MapMd5, TotalScore) VALUES ('m1', 5)`)
	if _, err := open(t, d).Timeline(context.Background(), 1); !errors.Is(err, ErrNoTimeline) {
		t.Fatalf("err = %v, want ErrNoTimeline", err)
	}
	if _, err := open(t, d).Timeline(context.Background(), 99); !errors.Is(err, ErrNoTimeline) {
		t.Fatalf("unknown id on a legacy db: err = %v", err)
	}
}
