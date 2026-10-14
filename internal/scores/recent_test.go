package scores

import (
	"context"
	"fmt"
	"math/rand"
	"reflect"
	"testing"
	"time"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/testutil"
)

// TestRecentMatchesScoreByIDOnARandomDatabase is the golden check: whatever way Recent reads
// the rows, each entry must equal what ScoreByID gives for the same id.
func TestRecentMatchesScoreByIDOnARandomDatabase(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	d := testutil.NewQuaverDir(t)
	mods := []int64{0, 0, 0, 1 << 5, 1 << 6, 1 << 34, 1 << 21, 1 << 19, 1 << 20, 1 << 31, 1<<5 | 1<<31, 1 << 18, 1 << 42}
	for m := 0; m < 25; m++ {
		d.AddMap(t, testutil.MapRow{MD5: fmt.Sprintf("md5-%d", m), Artist: "A", Title: fmt.Sprintf("T%d", m), DiffName: "d", Mode: 1 + m%2, LastPlayedMS: int64(m), Difficulty: float64(5 + m)})
	}
	d.Exec(t, `UPDATE Map SET Difficulty09X = Difficulty10X - 1, Difficulty115X = Difficulty10X + 2 WHERE Id % 3 <> 0`)
	for i := 0; i < 400; i++ {
		md5 := fmt.Sprintf("md5-%d", rng.Intn(27))
		d.AddScore(t, testutil.ScoreRow{
			MapMD5: md5, Total: int64(rng.Intn(1_000_000)), Grade: rng.Intn(9), Accuracy: rng.Float64() * 100,
			MaxCombo: rng.Intn(900), Marv: rng.Intn(500), Perf: rng.Intn(50), Great: rng.Intn(20), Miss: rng.Intn(4),
			Mods: mods[rng.Intn(len(mods))],
		})
	}
	d.Exec(t, `UPDATE Score SET Mods = NULL WHERE Id % 37 = 0`)
	d.Exec(t, `UPDATE Score SET Grade = NULL WHERE Id % 41 = 0`)
	st := open(t, d)
	ctx := context.Background()

	got, err := st.Recent(ctx, 200)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 200 {
		t.Fatalf("len = %d", len(got))
	}
	for i, r := range got {
		want, err := st.ScoreByID(ctx, r.Score.ID)
		if err != nil {
			t.Fatal(err)
		}
		if r.Score.ID != 400-int64(i) || !reflect.DeepEqual(r, want) {
			t.Fatalf("entry %d:\n got  %+v\n want %+v", i, r, want)
		}
	}
}

func TestRecentOnALargeDatabaseIsFast(t *testing.T) {
	if testing.Short() {
		t.Skip("builds 20000 scores")
	}
	d := testutil.NewQuaverDir(t)
	d.Exec(t, `WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i + 1 FROM n WHERE i < 800)
		INSERT INTO Map (Md5Checksum, Artist, Title, DifficultyName, LastTimePlayed, Mode, Difficulty10X) SELECT 'md5-' || i, 'A', 'T' || i, 'd', i, 1, 10 FROM n`)
	d.Exec(t, `WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i + 1 FROM n WHERE i < 20000)
		INSERT INTO Score (MapMd5, TotalScore, Grade, Accuracy, MaxCombo, CountMarv, CountPerf, CountGreat, CountGood, CountOkay, CountMiss, Mods, DateTime, PerformanceRating)
		SELECT 'md5-' || (i * 7 % 800 + 1), (i * 7919) % 1000000, i % 9, 90.5, 100, 90, 5, 3, 1, 0, i % 4, CASE WHEN i % 5 = 0 THEN 32 ELSE 0 END, '2026-10-12 13:06:06', 12.5 FROM n`)
	st := open(t, d)
	ctx := context.Background()
	if _, err := st.Recent(ctx, 5); err != nil {
		t.Fatal(err)
	}

	// The reference is what the history cost before: one ScoreByID per entry. Comparing with it in
	// the same run keeps the check independent of how fast the machine is.
	start := time.Now()
	for id := int64(20000); id > 19800; id-- {
		if _, err := st.ScoreByID(ctx, id); err != nil {
			t.Fatal(err)
		}
	}
	perEntry := time.Since(start)

	start = time.Now()
	got, err := st.Recent(ctx, 200)
	took := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 200 {
		t.Fatalf("len = %d", len(got))
	}
	t.Logf("Recent(200) took %v, 200 single lookups took %v", took, perEntry)
	if took*3 > perEntry {
		t.Fatalf("Recent took %v, one lookup per entry takes %v: it must be at least 3 times faster", took, perEntry)
	}
}
