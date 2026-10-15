package scores

import (
	"context"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/testutil"
)

const (
	gradeS = 6
	gradeA = 1
	speed  = int64(1 << 6)
)

func standingOf(r Result) standing { return standing{r.LocalRank, r.LocalTotal, r.IsLocalBest} }

func addMap(t *testing.T, d *testutil.FakeDir, md5 string) {
	t.Helper()
	d.AddMap(t, testutil.MapRow{MD5: md5, Title: md5, Mode: 1, LastPlayedMS: 1000, Difficulty: 10})
}

func addPlay(t *testing.T, d *testutil.FakeDir, total int64, rating, acc float64) int64 {
	t.Helper()
	return d.AddScore(t, testutil.ScoreRow{MapMD5: "m", Total: total, Grade: gradeA, Rating: rating, Accuracy: acc, Profile: 1})
}

func checkStanding(t *testing.T, st *Store, id int64, want standing) {
	t.Helper()
	if got := standingOf(result(t, st, id)); got != want {
		t.Fatalf("ScoreByID(%d): got %+v, want %+v", id, got, want)
	}
	recent, err := st.Recent(context.Background(), 200)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range recent {
		if r.Score.ID == id {
			if got := standingOf(r); got != want {
				t.Fatalf("Recent id %d: got %+v, want %+v", id, got, want)
			}
			return
		}
	}
	t.Fatalf("id %d missing from Recent", id)
}

func TestLocalRankFollowsTotalScoreWithEqualRatings(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	addMap(t, d, "m")
	a, b, c := addPlay(t, d, 700, 10, 90), addPlay(t, d, 800, 10, 90), addPlay(t, d, 900, 10, 90)
	st := open(t, d)
	checkStanding(t, st, a, standing{1, 1, false})
	checkStanding(t, st, b, standing{1, 2, true})
	checkStanding(t, st, c, standing{1, 3, true})
}

func TestLocalRankPrefersRatingThenAccuracyOverTotal(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	addMap(t, d, "m")
	a := addPlay(t, d, 900, 10, 90)
	b := addPlay(t, d, 800, 20, 80)
	c := addPlay(t, d, 700, 20, 95)
	st := open(t, d)
	checkStanding(t, st, a, standing{1, 1, false})
	checkStanding(t, st, b, standing{1, 2, true})
	checkStanding(t, st, c, standing{1, 3, true})
}

func TestLocalRankTiesKeepTheEarlierPlayFirst(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	addMap(t, d, "m")
	addPlay(t, d, 800, 10, 90)
	b := addPlay(t, d, 800, 10, 90)
	checkStanding(t, open(t, d), b, standing{2, 2, false})
}

func TestLocalRankPutsFailedPlaysLastAndNeverBest(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	addMap(t, d, "m")
	a := addPlay(t, d, 500, 5, 70)
	f := d.AddScore(t, testutil.ScoreRow{MapMD5: "m", Total: 900, Grade: failedGrade, Accuracy: 99, Profile: 1})
	b := addPlay(t, d, 100, 1, 50)
	st := open(t, d)
	checkStanding(t, st, a, standing{1, 1, false})
	checkStanding(t, st, f, standing{2, 2, false})
	checkStanding(t, st, b, standing{2, 3, false})
}

func TestLocalRankCountsEverySpeedWhilePBStaysPerSpeed(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	addMap(t, d, "m")
	addPlay(t, d, 700, 10, 90)
	b := d.AddScore(t, testutil.ScoreRow{MapMD5: "m", Total: 900, Grade: gradeS, Rating: 12, Accuracy: 95, Mods: speed, Profile: 1})
	st := open(t, d)
	checkStanding(t, st, b, standing{1, 2, true})
	if r := result(t, st, b); r.IsPB || !r.FirstPlay {
		t.Fatalf("same speed rule changed: IsPB=%v FirstPlay=%v", r.IsPB, r.FirstPlay)
	}
}

func TestLocalRankMixesEveryLocalProfile(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	addMap(t, d, "m")
	a := d.AddScore(t, testutil.ScoreRow{MapMD5: "m", Total: 990, Grade: gradeS, Rating: 30, Accuracy: 99, Profile: 1})
	b := d.AddScore(t, testutil.ScoreRow{MapMD5: "m", Total: 500, Grade: gradeA, Rating: 5, Accuracy: 80, Profile: 2})
	c := d.AddScore(t, testutil.ScoreRow{MapMD5: "m", Total: 600, Grade: gradeA, Rating: 6, Accuracy: 81, Profile: 3})
	st := open(t, d)
	checkStanding(t, st, a, standing{1, 1, false})
	checkStanding(t, st, b, standing{2, 2, false})
	checkStanding(t, st, c, standing{2, 3, false})
}

func TestLocalRankOfAFailedLeaderIsNotBest(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	addMap(t, d, "m")
	d.AddScore(t, testutil.ScoreRow{MapMD5: "m", Total: 100, Grade: failedGrade, Rating: 0, Accuracy: 50})
	b := d.AddScore(t, testutil.ScoreRow{MapMD5: "m", Total: 200, Grade: failedGrade, Rating: 1, Accuracy: 60})
	checkStanding(t, open(t, d), b, standing{1, 2, false})
}

func TestLocalRankTieBreakIsTheIdWhateverTheRowOrder(t *testing.T) {
	plays := []boardPlay{{id: 9, total: 10, rating: 1, accuracy: 1}, {id: 4, total: 10, rating: 1, accuracy: 1}, {id: 6, total: 10, rating: 1, accuracy: 1}}
	for _, order := range [][]int{{0, 1, 2}, {2, 1, 0}, {1, 2, 0}} {
		var shuffled []boardPlay
		for _, i := range order {
			shuffled = append(shuffled, plays[i])
		}
		got := boardStandings(shuffled, map[int64]bool{4: true, 6: true, 9: true})
		if got[4] != (standing{1, 1, false}) || got[6] != (standing{2, 2, false}) || got[9] != (standing{3, 3, false}) {
			t.Fatalf("order %v: %+v", order, got)
		}
	}
	d := testutil.NewQuaverDir(t)
	addMap(t, d, "m")
	for _, id := range []int{9, 4} {
		d.Exec(t, "INSERT INTO Score (Id, MapMd5, TotalScore, Grade, Accuracy, PerformanceRating) VALUES (?, 'm', 800, 1, 90, 10)", id)
	}
	checkStanding(t, open(t, d), 9, standing{2, 2, false})
}

func TestLocalRankReadsNullRatingAsZero(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	addMap(t, d, "m")
	a := addPlay(t, d, 900, 10, 90)
	b := addPlay(t, d, 800, 10, 85)
	d.Exec(t, `UPDATE Score SET PerformanceRating = NULL WHERE Id = ?`, a)
	checkStanding(t, open(t, d), b, standing{1, 2, true})
}

func TestLocalRankHasFiftyEntries(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	addMap(t, d, "m")
	var ids []int64
	for i := 0; i < 51; i++ {
		ids = append(ids, addPlay(t, d, int64(10000-i), 10, 90))
	}
	st := open(t, d)
	checkStanding(t, st, ids[49], standing{50, 50, false})
	checkStanding(t, st, ids[50], standing{0, 50, false})

	e := testutil.NewQuaverDir(t)
	addMap(t, e, "m")
	for i := 0; i < 51; i++ {
		ids[i] = addPlay(t, e, int64(100+i), 10, 90)
	}
	checkStanding(t, open(t, e), ids[50], standing{1, 50, true})
}

type refPlay struct {
	id, total int64
	grade     int
	rating    float64
	acc       float64
}

func refBefore(a, b refPlay) bool {
	af, bf := a.grade == failedGrade, b.grade == failedGrade
	if af != bf {
		return !af
	}
	if a.rating != b.rating {
		return a.rating > b.rating
	}
	return a.acc > b.acc
}

// refStanding is the leaderboard written as plainly as possible: take the best of the remaining
// plays 50 times, then insert them one by one into an ordered list after the plays that are not worse.
func refStanding(all []refPlay, self refPlay) standing {
	var pool []refPlay
	for _, p := range all {
		if p.id <= self.id {
			pool = append(pool, p)
		}
	}
	var top []refPlay
	for len(top) < 50 && len(pool) > 0 {
		bi := 0
		for i, p := range pool {
			if p.total > pool[bi].total || p.total == pool[bi].total && p.id < pool[bi].id {
				bi = i
			}
		}
		top = append(top, pool[bi])
		pool = append(pool[:bi], pool[bi+1:]...)
	}
	var board []refPlay
	for _, p := range top {
		pos := len(board)
		for pos > 0 && refBefore(p, board[pos-1]) {
			pos--
		}
		board = append(board[:pos], append([]refPlay{p}, board[pos:]...)...)
	}
	out := standing{total: len(board)}
	for i, p := range board {
		if p.id == self.id {
			out.rank = i + 1
		}
	}
	out.best = out.rank == 1 && len(board) > 1 && self.grade != failedGrade
	return out
}

func TestLocalRankMatchesAPlainReferenceOnRandomDatabases(t *testing.T) {
	for seed := uint64(1); seed <= 3; seed++ {
		r := rand.New(rand.NewPCG(seed, 7))
		d := testutil.NewQuaverDir(t)
		md5s := []string{"m1", "m2", "m3"}
		for _, m := range md5s {
			addMap(t, d, m)
		}
		byMap := map[string][]refPlay{}
		idMap := map[int64]string{}
		for i := 0; i < 260; i++ {
			md5 := md5s[r.IntN(len(md5s))]
			if i < 120 {
				md5 = md5s[0]
			}
			p := refPlay{total: int64(r.IntN(30)) * 1000, grade: []int{0, 1, 2, 5, 6, 8}[r.IntN(6)], rating: float64(r.IntN(6)), acc: float64(90 + r.IntN(5))}
			mods := []int64{0, 0, speed, 1 << 7}[r.IntN(4)]
			p.id = d.AddScore(t, testutil.ScoreRow{MapMD5: md5, Total: p.total, Grade: p.grade, Rating: p.rating, Accuracy: p.acc, Mods: mods, Profile: r.IntN(3)})
			if r.IntN(8) == 0 {
				p.rating = 0
				d.Exec(t, `UPDATE Score SET PerformanceRating = NULL WHERE Id = ?`, p.id)
			}
			byMap[md5] = append(byMap[md5], p)
			idMap[p.id] = md5
		}
		st := open(t, d)
		recent, err := st.Recent(context.Background(), 200)
		if err != nil {
			t.Fatal(err)
		}
		for _, got := range recent {
			var self refPlay
			for _, p := range byMap[idMap[got.Score.ID]] {
				if p.id == got.Score.ID {
					self = p
				}
			}
			want := refStanding(byMap[idMap[got.Score.ID]], self)
			if standingOf(got) != want {
				t.Fatalf("seed %d Recent id %d: got %+v, want %+v", seed, got.Score.ID, standingOf(got), want)
			}
			if one := result(t, st, got.Score.ID); got.Score.ID%4 == 0 && standingOf(one) != want {
				t.Fatalf("seed %d ScoreByID %d: got %+v, want %+v", seed, got.Score.ID, standingOf(one), want)
			}
		}
	}
}

func TestLocalRankOfAWholeWindowIsComputedOncePerMap(t *testing.T) {
	r := rand.New(rand.NewPCG(5, 5))
	plays := make([]boardPlay, 5000)
	want := map[int64]bool{}
	for i := range plays {
		plays[i] = boardPlay{id: int64(i + 1), total: int64(r.IntN(1000000)), rating: float64(r.IntN(30)), accuracy: float64(r.IntN(100)), failed: r.IntN(9) == 0}
		if i >= len(plays)-200 {
			want[plays[i].id] = true
		}
	}
	var all map[int64]standing
	batch := time.Hour
	for range 5 {
		start := time.Now()
		all = boardStandings(plays, want)
		batch = min(batch, time.Since(start))
	}
	start := time.Now()
	for id := range want {
		if one := boardStandings(plays, map[int64]bool{id: true}); one[id] != all[id] {
			t.Fatalf("id %d: alone %+v, in a batch %+v", id, one[id], all[id])
		}
	}
	alone := time.Since(start)
	t.Logf("200 standings in one pass %v, one pass each %v", batch, alone)
	if batch*3 > alone {
		t.Fatalf("one pass took %v, one pass per play took %v: the batch must be at least 3 times faster", batch, alone)
	}
}

func TestLocalRankOnALargeDatabaseIsFast(t *testing.T) {
	if testing.Short() {
		t.Skip("builds 25000 scores")
	}
	d := testutil.NewQuaverDir(t)
	d.Exec(t, `WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i + 1 FROM n WHERE i < 800)
		INSERT INTO Map (Md5Checksum, Artist, Title, DifficultyName, LastTimePlayed, Mode, Difficulty10X) SELECT 'md5-' || i, 'A', 'T' || i, 'd', i, 1, 10 FROM n UNION ALL SELECT 'big', 'A', 'Big', 'd', 1, 1, 10`)
	d.Exec(t, `WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i + 1 FROM n WHERE i < 20000)
		INSERT INTO Score (MapMd5, TotalScore, Grade, Accuracy, MaxCombo, CountMarv, CountPerf, CountGreat, CountGood, CountOkay, CountMiss, Mods, DateTime, PerformanceRating, LocalProfileId)
		SELECT 'md5-' || (i * 7 % 800 + 1), (i * 7919) % 1000000, i % 9, 90.5, 100, 90, 5, 3, 1, 0, i % 4, CASE WHEN i % 5 = 0 THEN 32 ELSE 0 END, '2026-10-12 13:06:06', 12.5, i % 2 FROM n`)
	d.Exec(t, `WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i + 1 FROM n WHERE i < 5000)
		INSERT INTO Score (MapMd5, TotalScore, Grade, Accuracy, MaxCombo, CountMarv, CountPerf, CountGreat, CountGood, CountOkay, CountMiss, Mods, DateTime, PerformanceRating, LocalProfileId)
		SELECT 'big', (i * 7919) % 1000000, i % 9, 90.5, 100, 90, 5, 3, 1, 0, 0, 0, '2026-10-12 13:06:06', i % 30, 1 FROM n`)
	st := open(t, d)
	ctx := context.Background()
	if _, err := st.Recent(ctx, 5); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	for id := int64(25000); id > 24900; id-- {
		if _, err := st.ScoreByID(ctx, id); err != nil {
			t.Fatal(err)
		}
	}
	perEntry := time.Since(start) * 2
	start = time.Now()
	got, err := st.Recent(ctx, 200)
	took := time.Since(start)
	if err != nil || len(got) != 200 {
		t.Fatalf("len %d err %v", len(got), err)
	}
	t.Logf("Recent(200) over a map with 5000 plays took %v, 200 single lookups would take about %v", took, perEntry)
	if took*3 > perEntry {
		t.Fatalf("Recent took %v, one lookup per entry takes about %v: it must be at least 3 times faster", took, perEntry)
	}
}
