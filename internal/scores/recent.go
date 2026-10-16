package scores

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"
)

const maxRecent = 200

const scoreColumnsBase = `SELECT s.Id, COALESCE(s.MapMd5,''), COALESCE(s.Grade,0), COALESCE(s.Accuracy,0), COALESCE(s.MaxCombo,0),
		        COALESCE(s.CountMarv,0), COALESCE(s.CountPerf,0), COALESCE(s.CountGreat,0), COALESCE(s.CountGood,0), COALESCE(s.CountOkay,0), COALESCE(s.CountMiss,0),
		        COALESCE(s.TotalScore,0), COALESCE(s.DateTime,''), MAX(COALESCE(s.Mods,0),0), COALESCE(s.PerformanceRating,0), `

func (s *Store) scoreColumns() string { return scoreColumnsBase + s.mapCols() }

// scoreSelect reads a score with its map; callers add the WHERE clause.
func (s *Store) scoreSelect() string {
	return s.scoreColumns() + `
		 FROM Score s LEFT JOIN Map m ON m.Md5Checksum = s.MapMd5`
}

type scoreRow struct {
	r       Result
	mapMD5  string
	grade   int
	mapMode int
	scratch int
	extra   mapExtra
}

type scanner interface{ Scan(dest ...any) error }

func scanScore(sc scanner) (scoreRow, error) {
	var rw scoreRow
	j := &rw.r.Score.Judgments
	dest := []any{&rw.r.Score.ID, &rw.mapMD5, &rw.grade, &rw.r.Score.Accuracy, &rw.r.Score.MaxCombo,
		&j.Marv, &j.Perf, &j.Great, &j.Good, &j.Okay, &j.Miss,
		&rw.r.Score.TotalScore, &rw.r.Score.PlayedAt, &rw.r.Score.ModsRaw, &rw.r.Score.Rating,
		&rw.r.Map.MD5, &rw.r.Map.Artist, &rw.r.Map.Title, &rw.r.Map.DifficultyName, &rw.r.Map.Difficulty, &rw.mapMode, &rw.r.Map.LastPlayedMS, &rw.scratch}
	return rw, sc.Scan(append(dest, rw.extra.dests()...)...)
}

// result derives the flags that need nothing but the row itself.
func (rw scoreRow) result() Result {
	r := rw.r
	r.Score.PlayedAtRaw = r.Score.PlayedAt
	r.Score.PlayedAt = parsePlayedAt(r.Score.PlayedAtRaw, time.Local)
	if rw.grade >= 0 && rw.grade < len(gradeNames) {
		r.Score.Grade = gradeNames[rw.grade]
	}
	r.Score.Rate, r.Score.Mods = decodeMods(r.Score.ModsRaw)
	r.Map.Keys = keysFor(rw.mapMode)
	r.Map.Scratch = r.Map.Keys > 0 && rw.scratch != 0
	rw.extra.apply(&r.Map)
	if r.Map.MD5 == "" {
		r.Map.MD5 = rw.mapMD5
	}
	r.Failed = rw.grade == failedGrade
	r.Paused = r.Score.ModsRaw&pausedMod != 0
	r.Autoplay = r.Score.ModsRaw&autoplayMod != 0
	r.FullCombo = !r.Failed && !r.Autoplay && r.Score.Judgments.Miss == 0
	return r
}

// applyPrevious sets FirstPlay and IsPB from the earlier comparable plays of the map.
func applyPrevious(r *Result, prevCount int, prevMax int64) {
	r.FirstPlay = prevCount == 0
	r.IsPB = !r.Failed && r.Score.ModsRaw&disqualifyMask == 0 && prevCount > 0 && r.Score.TotalScore > prevMax
}

// Recent returns the latest plays, newest first. limit is clamped to 1..200. It reads the
// window, the earlier plays of its maps and the rate difficulties in a fixed number of queries.
func (s *Store) Recent(ctx context.Context, limit int) ([]Result, error) {
	out, err := s.recent(ctx, limit)
	return out, s.track(err)
}

func (s *Store) recent(ctx context.Context, limit int) ([]Result, error) {
	limit = min(max(limit, 1), maxRecent)
	rows, err := s.api.QueryContext(ctx, s.scoreColumns()+`
		 FROM (SELECT * FROM Score ORDER BY Id DESC LIMIT ?) s LEFT JOIN Map m ON m.Md5Checksum = s.MapMd5 ORDER BY s.Id DESC`, limit)
	if err != nil {
		return nil, fmt.Errorf("list recent scores: %w", err)
	}
	defer rows.Close()
	var window []scoreRow
	seen := map[int64]bool{}
	for rows.Next() {
		rw, err := scanScore(rows)
		if err != nil {
			return nil, fmt.Errorf("list recent scores: %w", err)
		}
		if !seen[rw.r.Score.ID] {
			seen[rw.r.Score.ID] = true
			window = append(window, rw)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list recent scores: %w", err)
	}
	rows.Close()

	out := make([]Result, len(window))
	for i, rw := range window {
		out[i] = rw.result()
	}
	s.applyRateDifficulties(ctx, out)
	if err := s.applyPreviousPlays(ctx, window, out); err != nil {
		return nil, err
	}
	if err := s.applyLocalRanks(ctx, window, out); err != nil {
		return nil, err
	}
	return out, nil
}

func placeholders(n int) string { return strings.TrimSuffix(strings.Repeat("?,", n), ",") }

// applyRateDifficulties is applyRateDifficulty for a whole window: one query per rate column.
// The column names come from a fixed table, never from input.
func (s *Store) applyRateDifficulties(ctx context.Context, out []Result) {
	byCol := map[string]map[string]bool{}
	for _, r := range out {
		if col := speedOf(r.Score.ModsRaw).col; col != "Difficulty10X" {
			if byCol[col] == nil {
				byCol[col] = map[string]bool{}
			}
			byCol[col][r.Map.MD5] = true
		}
	}
	diffs := map[string]map[string]float64{}
	for col, md5s := range byCol {
		args := make([]any, 0, len(md5s))
		for m := range md5s {
			args = append(args, m)
		}
		rows, err := s.api.QueryContext(ctx,
			`SELECT Md5Checksum, COALESCE(`+col+`,0) FROM Map WHERE Md5Checksum IN (`+placeholders(len(args))+`) ORDER BY Id`, args...)
		if err != nil {
			slog.Debug("rate difficulty unavailable", "column", col, "err", err)
			continue
		}
		diffs[col] = map[string]float64{}
		for rows.Next() {
			var md5 string
			var d float64
			if rows.Scan(&md5, &d) == nil {
				if _, dup := diffs[col][md5]; !dup {
					diffs[col][md5] = d
				}
			}
		}
		rows.Close()
	}
	for i, r := range out {
		col := speedOf(r.Score.ModsRaw).col
		if d := diffs[col][r.Map.MD5]; col != "Difficulty10X" && d != 0 {
			out[i].Map.Difficulty = d
		}
	}
}

type play struct {
	id, total, mods int64
}

// applyPreviousPlays sets FirstPlay and IsPB with the same rule as ScoreByID, from one read of
// the comparable plays of every map in the window.
func (s *Store) applyPreviousPlays(ctx context.Context, window []scoreRow, out []Result) error {
	md5s := map[string]bool{}
	for _, rw := range window {
		md5s[rw.mapMD5] = true
	}
	args := make([]any, 0, len(md5s)+1)
	for m := range md5s {
		args = append(args, m)
	}
	args = append(args, disqualifyMask)
	rows, err := s.api.QueryContext(ctx,
		`SELECT MapMd5, Id, COALESCE(TotalScore,0), MAX(COALESCE(Mods,0),0) FROM Score WHERE MapMd5 IN (`+placeholders(len(md5s))+`) AND Grade <> `+strconv.Itoa(failedGrade)+` AND MAX(COALESCE(Mods,0),0) & ? = 0`, args...)
	if err != nil {
		return fmt.Errorf("read previous scores: %w", err)
	}
	defer rows.Close()
	plays := map[string][]play{}
	for rows.Next() {
		var md5 string
		var p play
		if err := rows.Scan(&md5, &p.id, &p.total, &p.mods); err != nil {
			return fmt.Errorf("read previous scores: %w", err)
		}
		plays[md5] = append(plays[md5], p)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read previous scores: %w", err)
	}
	for i, rw := range window {
		var count int
		var best int64
		for _, p := range plays[rw.mapMD5] {
			if p.id < rw.r.Score.ID && p.mods&keyMask == rw.r.Score.ModsRaw&keyMask {
				count++
				best = max(best, p.total)
			}
		}
		applyPrevious(&out[i], count, best)
	}
	return nil
}
