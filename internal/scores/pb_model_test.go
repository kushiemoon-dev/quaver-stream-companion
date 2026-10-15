package scores

import (
	"context"
	"math/rand/v2"
	"testing"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/testutil"
)

type modelScore struct {
	id    int64
	md5   string
	total int64
	grade int
	mods  int64
}

// modelFlags is the documented PB rule, written without the package masks: NoPause (18) and
// NoFail (21) are ignored, Autoplay (19) and Paused (20) disqualify, every other bit must match.
func modelFlags(all []modelScore, s modelScore) (firstPlay, isPB bool) {
	const ignored, disq = int64(1<<18 | 1<<21), int64(1<<19 | 1<<20)
	count, best := 0, int64(0)
	for _, p := range all {
		if p.md5 == s.md5 && p.id < s.id && p.grade != failedGrade && p.mods&disq == 0 &&
			p.mods&^(ignored|disq) == s.mods&^(ignored|disq) {
			count++
			best = max(best, p.total)
		}
	}
	return count == 0, s.grade != failedGrade && s.mods&disq == 0 && count > 0 && s.total > best
}

// TestPBModelRecentAndScoreByID builds random play histories and checks that Recent and ScoreByID
// both agree with the model on FirstPlay, IsPB, Failed, Paused, Autoplay and FullCombo.
func TestPBModelRecentAndScoreByID(t *testing.T) {
	modsPool := []int64{0, 0, 0, 1 << 18, 1 << 21, 1 << 19, 1 << 20, 1 << 5, 1<<5 | 1<<18, 1 << 23, 1 << 0, 1 << 46}
	for seed := uint64(1); seed <= 6; seed++ {
		r := rand.New(rand.NewPCG(seed, 99))
		d := testutil.NewQuaverDir(t)
		md5s := []string{"m1", "m2", "m3"}
		for i, m := range md5s {
			d.AddMap(t, testutil.MapRow{MD5: m, Title: m, Mode: 1 + i%2, LastPlayedMS: 1000, Difficulty: 10})
		}
		var all []modelScore
		for i := 0; i < 40; i++ {
			s := modelScore{md5: md5s[r.IntN(len(md5s))], total: int64(r.IntN(5)) * 100000, grade: r.IntN(9), mods: modsPool[r.IntN(len(modsPool))]}
			miss := r.IntN(2)
			s.id = d.AddScore(t, testutil.ScoreRow{MapMD5: s.md5, Total: s.total, Grade: s.grade, Mods: s.mods, Miss: miss})
			all = append(all, s)
		}
		st := open(t, d)
		recent, err := st.Recent(context.Background(), 200)
		if err != nil {
			t.Fatal(err)
		}
		if len(recent) != len(all) {
			t.Fatalf("seed %d: recent has %d rows, want %d", seed, len(recent), len(all))
		}
		for i, got := range recent {
			s := all[len(all)-1-i]
			if got.Score.ID != s.id {
				t.Fatalf("seed %d: recent[%d] id %d, want %d (newest first)", seed, i, got.Score.ID, s.id)
			}
			one, err := st.ScoreByID(context.Background(), s.id)
			if err != nil {
				t.Fatal(err)
			}
			first, pb := modelFlags(all, s)
			for name, res := range map[string]Result{"recent": got, "byID": one} {
				if res.FirstPlay != first || res.IsPB != pb {
					t.Fatalf("seed %d %s id %d (mods %b grade %d total %d): first=%v pb=%v, model first=%v pb=%v",
						seed, name, s.id, s.mods, s.grade, s.total, res.FirstPlay, res.IsPB, first, pb)
				}
				failed := s.grade == failedGrade
				autoplay := s.mods&(1<<19) != 0
				if res.Failed != failed || res.Autoplay != autoplay || res.Paused != (s.mods&(1<<20) != 0) ||
					res.FullCombo != (!failed && !autoplay && res.Score.Judgments.Miss == 0) {
					t.Fatalf("seed %d %s id %d: flags %+v", seed, name, s.id, res)
				}
			}
			if one.Map.Difficulty != got.Map.Difficulty || one.Score.Rate != got.Score.Rate {
				t.Fatalf("seed %d id %d: difficulty/rate differ: byID %v@%v, recent %v@%v", seed, s.id,
					one.Map.Difficulty, one.Score.Rate, got.Map.Difficulty, got.Score.Rate)
			}
		}
	}
}
