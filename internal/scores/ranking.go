package scores

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
)

// localBoardSize is how many plays Quaver keeps on a map's local leaderboard.
const localBoardSize = 50

// boardPlay is one stored play as the local leaderboard sees it.
type boardPlay struct {
	id, total        int64
	failed           bool
	rating, accuracy float64
}

// standing is the place of a play on the local leaderboard right after it was played.
type standing struct {
	rank, total int
	best        bool
}

// boardStandings ranks plays the way Quaver orders its local leaderboard of a map, as it stood
// right after each wanted play (ScoreDatabaseCache.FetchMapScores). Quaver mixes every local
// profile, play speed and mod set. The 50 plays with the highest total score are kept (the oldest
// play wins a tie), then failed plays go last, then the performance rating and the accuracy
// decide, highest first, keeping the earlier order for equal plays. A play is best when it leads a
// board that holds an earlier play and it did not fail. The plays are walked once in id order,
// so the cost does not grow with the number of wanted plays.
func boardStandings(plays []boardPlay, want map[int64]bool) map[int64]standing {
	sorted := append([]boardPlay(nil), plays...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].id < sorted[j].id })
	out := make(map[int64]standing, len(want))
	top := make([]boardPlay, 0, localBoardSize+1)
	for _, p := range sorted {
		pos := sort.Search(len(top), func(i int) bool { return top[i].total < p.total })
		if pos < localBoardSize {
			top = append(top, boardPlay{})
			copy(top[pos+1:], top[pos:])
			top[pos] = p
			top = top[:min(len(top), localBoardSize)]
		}
		if want[p.id] {
			out[p.id] = standingIn(top, p)
		}
	}
	return out
}

// standingIn orders the kept plays and finds the place of p, rank 0 when it was not kept.
func standingIn(top []boardPlay, p boardPlay) standing {
	board := append([]boardPlay(nil), top...)
	sort.SliceStable(board, func(i, j int) bool {
		a, b := board[i], board[j]
		if a.failed != b.failed {
			return !a.failed
		}
		if a.rating != b.rating {
			return a.rating > b.rating
		}
		return a.accuracy > b.accuracy
	})
	s := standing{total: len(board)}
	for i, q := range board {
		if q.id == p.id {
			s.rank = i + 1
		}
	}
	s.best = s.rank == 1 && len(board) > 1 && !p.failed
	return s
}

func applyLocalRank(r *Result, s standing) {
	r.LocalRank, r.LocalTotal, r.IsLocalBest = s.rank, s.total, s.best
}

// boardPlays reads every play of the given maps with what the local leaderboard orders by.
func (s *Store) boardPlays(ctx context.Context, db *sql.DB, query string, args []any) (map[string][]boardPlay, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT MapMd5, Id, COALESCE(TotalScore,0), COALESCE(Grade,0), COALESCE(PerformanceRating,0), COALESCE(Accuracy,0) FROM Score WHERE `+query, args...)
	if err != nil {
		return nil, fmt.Errorf("read local leaderboard: %w", err)
	}
	defer rows.Close()
	plays := map[string][]boardPlay{}
	for rows.Next() {
		var md5 string
		var p boardPlay
		var grade int
		if err := rows.Scan(&md5, &p.id, &p.total, &grade, &p.rating, &p.accuracy); err != nil {
			return nil, fmt.Errorf("read local leaderboard: %w", err)
		}
		p.failed = grade == failedGrade
		plays[md5] = append(plays[md5], p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read local leaderboard: %w", err)
	}
	return plays, nil
}

// applyLocalRanks sets the local leaderboard fields of a whole window from one read of the plays
// of its maps.
func (s *Store) applyLocalRanks(ctx context.Context, window []scoreRow, out []Result) error {
	wanted := map[string]map[int64]bool{}
	for _, rw := range window {
		if wanted[rw.mapMD5] == nil {
			wanted[rw.mapMD5] = map[int64]bool{}
		}
		wanted[rw.mapMD5][rw.r.Score.ID] = true
	}
	args := make([]any, 0, len(wanted))
	for m := range wanted {
		args = append(args, m)
	}
	board, err := s.boardPlays(ctx, s.api, `MapMd5 IN (`+placeholders(len(args))+`)`, args)
	if err != nil {
		return err
	}
	stands := map[string]map[int64]standing{}
	for md5, w := range wanted {
		stands[md5] = boardStandings(board[md5], w)
	}
	for i, rw := range window {
		applyLocalRank(&out[i], stands[rw.mapMD5][rw.r.Score.ID])
	}
	return nil
}
