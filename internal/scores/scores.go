// Package scores reads maps and scores from Quaver's quaver.db, read-only.
package scores

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"

	_ "modernc.org/sqlite"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/paths"
)

// ErrNotFound is returned when the requested row does not exist.
var ErrNotFound = errors.New("not found in quaver.db")

// failedGrade is the grade Quaver stores for failed or quit plays.
const failedGrade = 5

var gradeNames = [...]string{"-", "A", "B", "C", "D", "F", "S", "SS", "X"}

// Map is a Quaver map difficulty.
type Map struct {
	MD5            string  `json:"md5"`
	Artist         string  `json:"artist"`
	Title          string  `json:"title"`
	DifficultyName string  `json:"difficulty_name"`
	Difficulty     float64 `json:"difficulty"`
	Keys           int     `json:"keys"`
	LastPlayedMS   int64   `json:"last_played_ms"`
}

// Judgments counts hits per judgment.
type Judgments struct {
	Marv  int `json:"marv"`
	Perf  int `json:"perf"`
	Great int `json:"great"`
	Good  int `json:"good"`
	Okay  int `json:"okay"`
	Miss  int `json:"miss"`
}

// Score is one play.
type Score struct {
	ID         int64     `json:"id"`
	Grade      string    `json:"grade"`
	Accuracy   float64   `json:"accuracy"`
	MaxCombo   int       `json:"max_combo"`
	Judgments  Judgments `json:"judgments"`
	TotalScore int64     `json:"total_score"`
	PlayedAt   string    `json:"played_at"`
	Mods       []string  `json:"mods"`
	ModsRaw    int64     `json:"mods_raw"`
	Rate       float64   `json:"rate"`
	Rating     float64   `json:"rating"`
}

// Result is a score with its map and derived flags.
type Result struct {
	Score     Score `json:"score"`
	Map       Map   `json:"map"`
	IsPB      bool  `json:"is_pb"`
	FirstPlay bool  `json:"first_play"`
	FullCombo bool  `json:"full_combo"`
	Failed    bool  `json:"failed"`
	Paused    bool  `json:"paused"`
	Autoplay  bool  `json:"autoplay"`
}

// Store reads quaver.db. The database is always opened read-only.
type Store struct {
	db *sql.DB
}

// Open opens quaverDir/quaver.db read-only.
func Open(quaverDir string) (*Store, error) {
	path := paths.DBPath(quaverDir)
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("quaver.db not found in %s: %w", quaverDir, err)
	}
	dsn := "file:" + filepath.ToSlash(path) + "?mode=ro&_pragma=busy_timeout(3000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open quaver.db: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("open quaver.db read-only: %w", err)
	}
	return &Store{db: db}, nil
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

const mapCols = `COALESCE(m.Md5Checksum,''), COALESCE(m.Artist,''), COALESCE(m.Title,''), COALESCE(m.DifficultyName,''), COALESCE(m.Difficulty10X,0), COALESCE(m.Mode,0), COALESCE(m.LastTimePlayed,0)`

func keysFor(mode int) int {
	switch mode {
	case 1:
		return 4
	case 2:
		return 7
	}
	return 0
}

// LastPlayedMap returns the most recently played map, or ErrNotFound.
func (s *Store) LastPlayedMap(ctx context.Context) (Map, error) {
	var m Map
	var mode int
	err := s.db.QueryRowContext(ctx,
		`SELECT `+mapCols+` FROM Map m ORDER BY m.LastTimePlayed DESC LIMIT 1`,
	).Scan(&m.MD5, &m.Artist, &m.Title, &m.DifficultyName, &m.Difficulty, &mode, &m.LastPlayedMS)
	if errors.Is(err, sql.ErrNoRows) {
		return Map{}, ErrNotFound
	}
	if err != nil {
		return Map{}, fmt.Errorf("read last played map: %w", err)
	}
	m.Keys = keysFor(mode)
	return m, nil
}

// ScoreByID returns a score with its map, or ErrNotFound.
func (s *Store) ScoreByID(ctx context.Context, id int64) (Result, error) {
	var r Result
	var mapMD5 string
	var grade, mode int
	j := &r.Score.Judgments
	err := s.db.QueryRowContext(ctx,
		`SELECT s.Id, COALESCE(s.MapMd5,''), COALESCE(s.Grade,0), COALESCE(s.Accuracy,0), COALESCE(s.MaxCombo,0),
		        COALESCE(s.CountMarv,0), COALESCE(s.CountPerf,0), COALESCE(s.CountGreat,0), COALESCE(s.CountGood,0), COALESCE(s.CountOkay,0), COALESCE(s.CountMiss,0),
		        COALESCE(s.TotalScore,0), COALESCE(s.DateTime,''), COALESCE(s.Mods,0), COALESCE(s.PerformanceRating,0), `+mapCols+`
		 FROM Score s LEFT JOIN Map m ON m.Md5Checksum = s.MapMd5 WHERE s.Id = ?`, id,
	).Scan(&r.Score.ID, &mapMD5, &grade, &r.Score.Accuracy, &r.Score.MaxCombo,
		&j.Marv, &j.Perf, &j.Great, &j.Good, &j.Okay, &j.Miss,
		&r.Score.TotalScore, &r.Score.PlayedAt, &r.Score.ModsRaw, &r.Score.Rating,
		&r.Map.MD5, &r.Map.Artist, &r.Map.Title, &r.Map.DifficultyName, &r.Map.Difficulty, &mode, &r.Map.LastPlayedMS)
	if errors.Is(err, sql.ErrNoRows) {
		return Result{}, ErrNotFound
	}
	if err != nil {
		return Result{}, fmt.Errorf("read score %d: %w", id, err)
	}

	if grade >= 0 && grade < len(gradeNames) {
		r.Score.Grade = gradeNames[grade]
	}
	r.Score.Rate, r.Score.Mods = decodeMods(r.Score.ModsRaw)
	r.Map.Keys = keysFor(mode)
	if r.Map.MD5 == "" {
		r.Map.MD5 = mapMD5
	}
	r.Failed = grade == failedGrade
	r.Paused = r.Score.ModsRaw&pausedMod != 0
	r.Autoplay = r.Score.ModsRaw&autoplayMod != 0
	s.applyRateDifficulty(ctx, &r)
	r.FullCombo = !r.Failed && !r.Autoplay && j.Miss == 0

	var prevCount int
	var prevMax int64
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*), COALESCE(MAX(TotalScore),0) FROM Score WHERE MapMd5 = ? AND Id < ? AND Grade <> `+strconv.Itoa(failedGrade)+` AND COALESCE(Mods,0) & ? = 0 AND COALESCE(Mods,0) & ? = ?`,
		mapMD5, id, disqualifyMask, keyMask, r.Score.ModsRaw&keyMask,
	).Scan(&prevCount, &prevMax); err != nil {
		return Result{}, fmt.Errorf("read previous scores of %q: %w", mapMD5, err)
	}
	r.FirstPlay = prevCount == 0
	r.IsPB = !r.Failed && r.Score.ModsRaw&disqualifyMask == 0 && prevCount > 0 && r.Score.TotalScore > prevMax
	return r, nil
}

// applyRateDifficulty replaces the 1.0x difficulty with the one at the played rate.
// The column name comes from a fixed table, never from input.
func (s *Store) applyRateDifficulty(ctx context.Context, r *Result) {
	col := speedOf(r.Score.ModsRaw).col
	if col == "Difficulty10X" {
		return
	}
	var d float64
	err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(`+col+`,0) FROM Map WHERE Md5Checksum = ?`, r.Map.MD5,
	).Scan(&d)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			slog.Debug("rate difficulty unavailable", "column", col, "err", err)
		}
		return
	}
	if d != 0 {
		r.Map.Difficulty = d
	}
}

const maxRecent = 200

// Recent returns the latest plays, newest first. limit is clamped to 1..200.
func (s *Store) Recent(ctx context.Context, limit int) ([]Result, error) {
	limit = min(max(limit, 1), maxRecent)
	rows, err := s.db.QueryContext(ctx, `SELECT Id FROM Score ORDER BY Id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list recent scores: %w", err)
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, fmt.Errorf("list recent scores: %w", err)
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, fmt.Errorf("list recent scores: %w", err)
	}

	out := make([]Result, 0, len(ids))
	for _, id := range ids {
		r, err := s.ScoreByID(ctx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}
