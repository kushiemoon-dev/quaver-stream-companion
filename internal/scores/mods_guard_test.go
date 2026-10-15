package scores

import (
	"context"
	"reflect"
	"testing"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/testutil"
)

func TestNegativeModsDecodeToNoMods(t *testing.T) {
	for _, mods := range []int64{-1, -2, -1 << 62} {
		rate, names := decodeMods(mods)
		if rate != 1.0 || !reflect.DeepEqual(names, []string{}) {
			t.Errorf("decodeMods(%d) = %v %v, want no mods", mods, rate, names)
		}
	}
}

func TestNegativeModsRowBehavesLikeNoMods(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	d.AddMap(t, testutil.MapRow{MD5: "m1", Mode: 1, LastPlayedMS: 5})
	d.AddScore(t, testutil.ScoreRow{MapMD5: "m1", Total: 1000, Grade: 4, Mods: -1})
	next := d.AddScore(t, testutil.ScoreRow{MapMD5: "m1", Total: 2000, Grade: 4})
	none := d.AddScore(t, testutil.ScoreRow{MapMD5: "m1", Total: 3000, Grade: 4, Mods: -1})
	st := open(t, d)

	r := result(t, st, none)
	if r.Paused || r.Autoplay || r.Score.Rate != 1.0 || len(r.Score.Mods) != 0 || r.Score.ModsRaw != 0 {
		t.Fatalf("Paused=%v Autoplay=%v Rate=%v Mods=%v Raw=%d", r.Paused, r.Autoplay, r.Score.Rate, r.Score.Mods, r.Score.ModsRaw)
	}
	if !r.IsPB || r.FirstPlay {
		t.Fatalf("a -1 row must compare with the plays without mods: IsPB=%v FirstPlay=%v", r.IsPB, r.FirstPlay)
	}
	if r := result(t, st, next); !r.IsPB || r.FirstPlay {
		t.Fatalf("the earlier -1 row must count as a previous play: IsPB=%v FirstPlay=%v", r.IsPB, r.FirstPlay)
	}
	recent, err := st.Recent(context.Background(), 5)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range recent {
		if r.Paused || r.Autoplay || r.Score.ModsRaw != 0 {
			t.Fatalf("recent row %d: Paused=%v Autoplay=%v Raw=%d", r.Score.ID, r.Paused, r.Autoplay, r.Score.ModsRaw)
		}
	}
}

func TestUnknownModBitsStayInRawAndInThePBKey(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	d.AddMap(t, testutil.MapRow{MD5: "m1", Mode: 1, LastPlayedMS: 5})
	d.AddScore(t, testutil.ScoreRow{MapMD5: "m1", Total: 9000, Grade: 4, Mods: 1 << 46})
	plain := d.AddScore(t, testutil.ScoreRow{MapMD5: "m1", Total: 100, Grade: 4})
	odd := d.AddScore(t, testutil.ScoreRow{MapMD5: "m1", Total: 200, Grade: 4, Mods: 1<<46 | 1<<31})
	st := open(t, d)

	if r := result(t, st, plain); !r.FirstPlay {
		t.Fatal("a play without the unknown bit must not compare with one that has it")
	}
	r := result(t, st, odd)
	if r.Score.ModsRaw != 1<<46|1<<31 || !reflect.DeepEqual(r.Score.Mods, []string{"Mirror"}) || r.Score.Rate != 1.0 {
		t.Fatalf("Raw=%d Mods=%v Rate=%v", r.Score.ModsRaw, r.Score.Mods, r.Score.Rate)
	}
}

func TestRecentPreviousPlaysTreatNegativeModsAsNone(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	d.AddMap(t, testutil.MapRow{MD5: "m1", Mode: 1, LastPlayedMS: 5})
	first := d.AddScore(t, testutil.ScoreRow{MapMD5: "m1", Total: 1000, Grade: 4, Mods: -1})
	second := d.AddScore(t, testutil.ScoreRow{MapMD5: "m1", Total: 2000, Grade: 4})
	third := d.AddScore(t, testutil.ScoreRow{MapMD5: "m1", Total: 3000, Grade: 4, Mods: -1})
	st := open(t, d)

	recent, err := st.Recent(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	got := map[int64]Result{}
	for _, r := range recent {
		got[r.Score.ID] = r
	}
	if !got[first].FirstPlay || got[first].IsPB {
		t.Errorf("first: FirstPlay=%v IsPB=%v", got[first].FirstPlay, got[first].IsPB)
	}
	for _, id := range []int64{second, third} {
		if got[id].FirstPlay || !got[id].IsPB {
			t.Errorf("score %d: FirstPlay=%v IsPB=%v", id, got[id].FirstPlay, got[id].IsPB)
		}
	}
}
