package scores

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/testutil"
)

func open(t *testing.T, d *testutil.FakeDir) *Store {
	t.Helper()
	st, err := Open(d.Path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestLastPlayedMapReturnsMostRecent(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	d.AddMap(t, testutil.MapRow{MD5: "old", Title: "Old", Mode: 1, LastPlayedMS: 1000})
	d.AddMap(t, testutil.MapRow{MD5: "new", Artist: "Art", Title: "New", DiffName: "hard", Mode: 2, LastPlayedMS: 2000, Difficulty: 15.5})

	m, err := open(t, d).LastPlayedMap(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if m.MD5 != "new" || m.Title != "New" || m.Artist != "Art" || m.DifficultyName != "hard" || m.Keys != 7 || m.Difficulty != 15.5 || m.LastPlayedMS != 2000 {
		t.Fatalf("map = %+v", m)
	}
}

func TestLastPlayedMapEmptyTable(t *testing.T) {
	_, err := open(t, testutil.NewQuaverDir(t)).LastPlayedMap(context.Background())
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestScoreByIDJoinsMapAndNamesGrade(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	d.AddMap(t, testutil.MapRow{MD5: "m1", Artist: "A", Title: "T", DiffName: "insane", Mode: 1, LastPlayedMS: 5, Difficulty: 12.3})
	id := d.AddScore(t, testutil.ScoreRow{MapMD5: "m1", Total: 900000, Grade: 7, Accuracy: 98.1, MaxCombo: 600, Marv: 500, Perf: 90, Great: 8, Good: 2, Okay: 0, Miss: 0})

	r, err := open(t, d).ScoreByID(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if r.Score.Grade != "SS" || r.Score.Accuracy != 98.1 || r.Score.MaxCombo != 600 || r.Score.TotalScore != 900000 {
		t.Fatalf("score = %+v", r.Score)
	}
	if r.Score.Judgments != (Judgments{Marv: 500, Perf: 90, Great: 8, Good: 2}) {
		t.Fatalf("judgments = %+v", r.Score.Judgments)
	}
	if r.Map.Title != "T" || r.Map.Keys != 4 {
		t.Fatalf("map = %+v", r.Map)
	}
}

func TestScoreByIDNotFound(t *testing.T) {
	_, err := open(t, testutil.NewQuaverDir(t)).ScoreByID(context.Background(), 999)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestPersonalBestAndFirstPlay(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	d.AddMap(t, testutil.MapRow{MD5: "m1", Title: "T", Mode: 1, LastPlayedMS: 5})
	first := d.AddScore(t, testutil.ScoreRow{MapMD5: "m1", Total: 1000})
	better := d.AddScore(t, testutil.ScoreRow{MapMD5: "m1", Total: 1500})
	worse := d.AddScore(t, testutil.ScoreRow{MapMD5: "m1", Total: 1200})
	st := open(t, d)

	for _, tc := range []struct {
		id              int64
		isPB, firstPlay bool
	}{{first, false, true}, {better, true, false}, {worse, false, false}} {
		r, err := st.ScoreByID(context.Background(), tc.id)
		if err != nil {
			t.Fatal(err)
		}
		if r.IsPB != tc.isPB || r.FirstPlay != tc.firstPlay {
			t.Fatalf("score %d: IsPB=%v FirstPlay=%v, want %v %v", tc.id, r.IsPB, r.FirstPlay, tc.isPB, tc.firstPlay)
		}
	}
}

func TestFullComboMeansNoMiss(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	d.AddMap(t, testutil.MapRow{MD5: "m1", Mode: 1, LastPlayedMS: 5})
	clean := d.AddScore(t, testutil.ScoreRow{MapMD5: "m1", Marv: 10})
	missed := d.AddScore(t, testutil.ScoreRow{MapMD5: "m1", Marv: 9, Miss: 1})
	st := open(t, d)

	r1, _ := st.ScoreByID(context.Background(), clean)
	r2, _ := st.ScoreByID(context.Background(), missed)
	if !r1.FullCombo || r2.FullCombo {
		t.Fatalf("FullCombo = %v / %v", r1.FullCombo, r2.FullCombo)
	}
}

func TestScoreWithoutMapRow(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	id := d.AddScore(t, testutil.ScoreRow{MapMD5: "ghost", Total: 10})

	r, err := open(t, d).ScoreByID(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if r.Map.MD5 != "ghost" || r.Map.Title != "" {
		t.Fatalf("map = %+v", r.Map)
	}
}

func TestNullColumnsDoNotFail(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	d.Exec(t, `INSERT INTO Map (Md5Checksum, LastTimePlayed) VALUES ('m1', 7)`)
	d.Exec(t, `INSERT INTO Score (MapMd5) VALUES ('m1')`)
	st := open(t, d)

	if _, err := st.LastPlayedMap(context.Background()); err != nil {
		t.Fatalf("LastPlayedMap: %v", err)
	}
	if _, err := st.ScoreByID(context.Background(), 1); err != nil {
		t.Fatalf("ScoreByID: %v", err)
	}
}

func TestScoreRating(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	id := d.AddScore(t, testutil.ScoreRow{MapMD5: "m1", Rating: 12.5})
	d.Exec(t, `INSERT INTO Score (MapMd5) VALUES ('m1')`)
	st := open(t, d)

	r, err := st.ScoreByID(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if r.Score.Rating != 12.5 {
		t.Fatalf("rating = %v", r.Score.Rating)
	}
	r, err = st.ScoreByID(context.Background(), id+1)
	if err != nil {
		t.Fatal(err)
	}
	if r.Score.Rating != 0 {
		t.Fatalf("null rating = %v", r.Score.Rating)
	}
}

func TestStoreIsReadOnly(t *testing.T) {
	st := open(t, testutil.NewQuaverDir(t))
	if _, err := st.db.Exec(`DELETE FROM Score`); err == nil {
		t.Fatal("write to quaver.db must be refused")
	}
}

func TestOpenMissingDatabase(t *testing.T) {
	_, err := Open(t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "quaver.db") {
		t.Fatalf("err = %v", err)
	}
}

// Set QUAVER_TEST_DB to a real Quaver folder to check the queries and the
// accuracy units against real data. Skipped otherwise.
func TestRealDatabase(t *testing.T) {
	dir := os.Getenv("QUAVER_TEST_DB")
	if dir == "" {
		t.Skip("QUAVER_TEST_DB not set")
	}
	st, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	m, err := st.LastPlayedMap(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("last map: %+v", m)
}

func TestFailedPlayIsNeverFullComboOrPB(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	d.AddMap(t, testutil.MapRow{MD5: "m1", Mode: 1, LastPlayedMS: 5})
	d.AddScore(t, testutil.ScoreRow{MapMD5: "m1", Total: 100, Grade: 4})
	failed := d.AddScore(t, testutil.ScoreRow{MapMD5: "m1", Total: 900, Grade: 5, Marv: 10})
	st := open(t, d)

	r, err := st.ScoreByID(context.Background(), failed)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Failed || r.FullCombo || r.IsPB {
		t.Fatalf("Failed=%v FullCombo=%v IsPB=%v", r.Failed, r.FullCombo, r.IsPB)
	}
}

func TestFailedEarlierPlayIsIgnoredAsPrevious(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	d.AddMap(t, testutil.MapRow{MD5: "m1", Mode: 1, LastPlayedMS: 5})
	d.AddScore(t, testutil.ScoreRow{MapMD5: "m1", Total: 5000, Grade: 5})
	pass := d.AddScore(t, testutil.ScoreRow{MapMD5: "m1", Total: 1000, Grade: 4})
	st := open(t, d)

	r, err := st.ScoreByID(context.Background(), pass)
	if err != nil {
		t.Fatal(err)
	}
	if !r.FirstPlay || r.IsPB || r.Failed {
		t.Fatalf("FirstPlay=%v IsPB=%v Failed=%v", r.FirstPlay, r.IsPB, r.Failed)
	}
}

func TestScoreModsAndRate(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	d.AddMap(t, testutil.MapRow{MD5: "m1", Mode: 1, LastPlayedMS: 5})
	plain := d.AddScore(t, testutil.ScoreRow{MapMD5: "m1"})
	modded := d.AddScore(t, testutil.ScoreRow{MapMD5: "m1", Mods: 1<<34 | 1<<20})
	st := open(t, d)

	r, err := st.ScoreByID(context.Background(), plain)
	if err != nil {
		t.Fatal(err)
	}
	if r.Score.Mods == nil || len(r.Score.Mods) != 0 || r.Score.ModsRaw != 0 || r.Score.Rate != 1.0 {
		t.Fatalf("plain score = %+v", r.Score)
	}
	r, err = st.ScoreByID(context.Background(), modded)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Score.Mods) != 1 || r.Score.Mods[0] != "Paused" || r.Score.ModsRaw != 1<<34|1<<20 || r.Score.Rate != 1.15 {
		t.Fatalf("modded score = %+v", r.Score)
	}
}

func TestMapDifficultyAtPlayedRate(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	d.AddMap(t, testutil.MapRow{MD5: "m1", Mode: 1, LastPlayedMS: 5, Difficulty: 10})
	d.Exec(t, `UPDATE Map SET Difficulty115X = 12.5 WHERE Md5Checksum = 'm1'`)
	d.AddMap(t, testutil.MapRow{MD5: "m2", Mode: 1, LastPlayedMS: 6, Difficulty: 20})
	rated := d.AddScore(t, testutil.ScoreRow{MapMD5: "m1", Mods: 1 << 34})
	plain := d.AddScore(t, testutil.ScoreRow{MapMD5: "m1"})
	fallback := d.AddScore(t, testutil.ScoreRow{MapMD5: "m2", Mods: 1 << 34})
	st := open(t, d)

	for _, tc := range []struct {
		name string
		id   int64
		want float64
	}{{"rate column", rated, 12.5}, {"no mods", plain, 10}, {"null rate column falls back", fallback, 20}} {
		r, err := st.ScoreByID(context.Background(), tc.id)
		if err != nil {
			t.Fatal(err)
		}
		if r.Map.Difficulty != tc.want {
			t.Fatalf("%s: difficulty = %v, want %v", tc.name, r.Map.Difficulty, tc.want)
		}
	}
}

func TestMissingRateColumnFallsBackToBaseDifficulty(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	d.AddMap(t, testutil.MapRow{MD5: "m1", Mode: 1, LastPlayedMS: 5, Difficulty: 10})
	id := d.AddScore(t, testutil.ScoreRow{MapMD5: "m1", Mods: 1 << 36})
	st := open(t, d)

	r, err := st.ScoreByID(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if r.Map.Difficulty != 10 {
		t.Fatalf("difficulty = %v, want 10", r.Map.Difficulty)
	}
}

func TestPausedPlayIsNotPBAndNotPrevious(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	d.AddMap(t, testutil.MapRow{MD5: "m1", Mode: 1, LastPlayedMS: 5})
	d.AddScore(t, testutil.ScoreRow{MapMD5: "m1", Total: 1000, Grade: 4})
	paused := d.AddScore(t, testutil.ScoreRow{MapMD5: "m1", Total: 9000, Grade: 4, Marv: 10, Mods: 1 << 20})
	after := d.AddScore(t, testutil.ScoreRow{MapMD5: "m1", Total: 2000, Grade: 4})
	st := open(t, d)

	r, err := st.ScoreByID(context.Background(), paused)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Paused || r.IsPB || !r.FullCombo || r.Failed {
		t.Fatalf("Paused=%v IsPB=%v FullCombo=%v Failed=%v", r.Paused, r.IsPB, r.FullCombo, r.Failed)
	}
	r, err = st.ScoreByID(context.Background(), after)
	if err != nil {
		t.Fatal(err)
	}
	if !r.IsPB || r.Paused {
		t.Fatalf("after: IsPB=%v Paused=%v", r.IsPB, r.Paused)
	}
}

func result(t *testing.T, st *Store, id int64) Result {
	t.Helper()
	r, err := st.ScoreByID(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestPersonalBestIsPerRateAndModSet(t *testing.T) {
	const (
		rate115 = int64(1 << 34)
		noFail  = int64(1 << 21)
		noLN    = int64(1 << 22)
		all     = rate115 | noFail | noLN | 1<<20
	)
	for _, tc := range []struct {
		name              string
		prevMods, mods    int64
		wantPB, wantFirst bool
	}{
		{"faster rate vs 1.0x", 0, rate115, false, true},
		{"same rate", rate115, rate115, true, false},
		{"NoFail ignored", noFail, 0, true, false},
		{"no mod vs NoFail play", 0, noFail, true, false},
		{"NoLN is not comparable", 0, noLN, false, true},
		{"unknown bit counts", 1 << 50, 0, false, true},
		{"same unknown bit", 1 << 50, 1 << 50, true, false},
		{"NoPause ignored", 1 << 18, 0, true, false},
		{"paused never PB", 0, all, false, true},
		{"autoplay never PB", 0, 1 << 19, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := testutil.NewQuaverDir(t)
			d.AddMap(t, testutil.MapRow{MD5: "m1", Mode: 1, LastPlayedMS: 5})
			d.AddScore(t, testutil.ScoreRow{MapMD5: "m1", Total: 1000, Grade: 4, Mods: tc.prevMods})
			id := d.AddScore(t, testutil.ScoreRow{MapMD5: "m1", Total: 2000, Grade: 4, Mods: tc.mods})
			r := result(t, open(t, d), id)
			if r.IsPB != tc.wantPB || r.FirstPlay != tc.wantFirst {
				t.Fatalf("IsPB=%v FirstPlay=%v, want %v %v", r.IsPB, r.FirstPlay, tc.wantPB, tc.wantFirst)
			}
		})
	}
}

func TestAutoplayIsNotPreviousAndNotFullCombo(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	d.AddMap(t, testutil.MapRow{MD5: "m1", Mode: 1, LastPlayedMS: 5})
	d.AddScore(t, testutil.ScoreRow{MapMD5: "m1", Total: 1000, Grade: 4})
	auto := d.AddScore(t, testutil.ScoreRow{MapMD5: "m1", Total: 9000, Grade: 4, Marv: 10, Mods: 1 << 19})
	after := d.AddScore(t, testutil.ScoreRow{MapMD5: "m1", Total: 2000, Grade: 4})
	st := open(t, d)

	if r := result(t, st, auto); !r.Autoplay || r.FullCombo || r.IsPB {
		t.Fatalf("autoplay: Autoplay=%v FullCombo=%v IsPB=%v", r.Autoplay, r.FullCombo, r.IsPB)
	}
	if r := result(t, st, after); !r.IsPB {
		t.Fatal("play after autoplay should be a PB against the first play")
	}
}

func TestNoFailPlayIsFullCombo(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	d.AddMap(t, testutil.MapRow{MD5: "m1", Mode: 1, LastPlayedMS: 5})
	id := d.AddScore(t, testutil.ScoreRow{MapMD5: "m1", Marv: 10, Mods: 1 << 21})
	if r := result(t, open(t, d), id); !r.FullCombo || r.Autoplay {
		t.Fatalf("FullCombo=%v Autoplay=%v", r.FullCombo, r.Autoplay)
	}
}

func TestRecentNewestFirstMatchesScoreByID(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	d.AddMap(t, testutil.MapRow{MD5: "m1", Title: "T", Mode: 1, LastPlayedMS: 5})
	var ids []int64
	ids = append(ids, d.AddScore(t, testutil.ScoreRow{MapMD5: "m1", Total: 1000, Marv: 10}))
	ids = append(ids, d.AddScore(t, testutil.ScoreRow{MapMD5: "m1", Total: 1500, Marv: 9, Miss: 1}))
	ids = append(ids, d.AddScore(t, testutil.ScoreRow{MapMD5: "m1", Total: 900, Grade: 5}))
	ids = append(ids, d.AddScore(t, testutil.ScoreRow{MapMD5: "m1", Total: 2000, Marv: 10, Mods: 1 << 21}))
	st := open(t, d)

	got, err := st.Recent(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(ids) {
		t.Fatalf("len = %d", len(got))
	}
	for i, r := range got {
		id := ids[len(ids)-1-i]
		want := result(t, st, id)
		if r.Score.ID != id || !reflect.DeepEqual(r, want) {
			t.Fatalf("entry %d = %+v, want %+v", i, r, want)
		}
	}
}

func TestRecentLimitIsClamped(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	d.AddMap(t, testutil.MapRow{MD5: "m1", Mode: 1})
	for i := 0; i < 205; i++ {
		d.AddScore(t, testutil.ScoreRow{MapMD5: "m1", Total: int64(i)})
	}
	st := open(t, d)

	for _, tc := range []struct{ limit, want int }{{3, 3}, {0, 1}, {-5, 1}, {1000, 200}} {
		got, err := st.Recent(context.Background(), tc.limit)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != tc.want {
			t.Fatalf("limit %d: len = %d, want %d", tc.limit, len(got), tc.want)
		}
	}
}

func TestRecentEmptyTablesGiveEmptySlice(t *testing.T) {
	got, err := open(t, testutil.NewQuaverDir(t)).Recent(context.Background(), 5)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("got = %#v", got)
	}
}

func TestRecentScoreWithoutMapRow(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	id := d.AddScore(t, testutil.ScoreRow{MapMD5: "ghost", Total: 10})

	got, err := open(t, d).Recent(context.Background(), 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Score.ID != id || got[0].Map.MD5 != "ghost" || got[0].Map.Title != "" {
		t.Fatalf("got = %+v", got)
	}
}
