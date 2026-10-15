// Package scores reads maps and scores from Quaver's quaver.db, read-only.
package scores

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

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
	// Keys is the lane count of the mode without the scratch lane, 0 for an unknown mode.
	Keys int `json:"keys"`
	// Scratch tells that the map has an extra scratch lane next to its Keys lanes.
	Scratch      bool  `json:"scratch"`
	LastPlayedMS int64 `json:"last_played_ms"`
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
	db       *sql.DB
	api      *sql.DB // dashboard reads: gives up on a locked database sooner than db
	now      func() time.Time
	path     string
	ident    os.FileInfo
	failures atomic.Int32
	// warnedFuture is the newest future map date already reported.
	warnedFuture atomic.Int64
	scratch      scratchFlag
}

// Open opens quaverDir/quaver.db read-only.
func Open(quaverDir string) (*Store, error) {
	path := paths.DBPath(quaverDir)
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("quaver.db not found in %s: %w", quaverDir, err)
	}
	dsn, err := readOnlyDSN(path, lookupBusy)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open quaver.db: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("open quaver.db read-only: %w", err)
	}
	apiDSN, _ := readOnlyDSN(path, apiBusy)
	api, err := sql.Open("sqlite", apiDSN)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("open quaver.db: %w", err)
	}
	db.SetConnMaxIdleTime(idleTime)
	api.SetConnMaxIdleTime(idleTime)
	ident, _ := fileIdentity(path)
	st := &Store{db: db, api: api, now: time.Now, path: path, ident: ident, scratch: scratchFlag{probe: scratchColumn}}
	st.mapCols()
	return st, nil
}

// readOnlyDSN builds a file: URL so characters such as #, %, ? and spaces in the folder name stay part of the path.
func readOnlyDSN(path string, busy time.Duration) (string, error) {
	if paths.IsUNC(path) {
		return "", fmt.Errorf("quaver.db path %q is a network path", path)
	}
	p := filepath.ToSlash(path)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	q := url.Values{"mode": {"ro"}, "_pragma": {fmt.Sprintf("busy_timeout(%d)", busy.Milliseconds())}}
	return (&url.URL{Scheme: "file", Path: p, RawQuery: q.Encode()}).String(), nil
}

// Close closes the database.
func (s *Store) Close() error { return errors.Join(s.db.Close(), s.api.Close()) }

const mapColsBase = `COALESCE(m.Md5Checksum,''), COALESCE(m.Artist,''), COALESCE(m.Title,''), COALESCE(m.DifficultyName,''), COALESCE(m.Difficulty10X,0), COALESCE(m.Mode,0), COALESCE(m.LastTimePlayed,0)`

// mapCols lists the map columns that are read for a score or a map; the scratch flag comes last.
func (s *Store) mapCols() string { return mapColsBase + ", " + s.scratch.expr(s.api, s.now()) }

// scratchRecheck is how long to wait before asking again when the column check failed.
const scratchRecheck = time.Minute

// scratchFlag finds out once whether Map has a HasScratchKey column. A failed check (the
// database was busy) is retried at most once per scratchRecheck instead of disabling the flag for good.
type scratchFlag struct {
	mu      sync.Mutex
	probe   func(*sql.DB) (expr string, err error)
	value   string
	known   bool
	checked time.Time
}

// expr returns the SQL that reads the flag: 0 while the column is missing or unknown.
func (f *scratchFlag) expr(db *sql.DB, now time.Time) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.known && (f.checked.IsZero() || now.Sub(f.checked) >= scratchRecheck) {
		f.checked = now
		if e, err := f.probe(db); err == nil {
			f.value, f.known = e, true
		}
	}
	if f.value == "" {
		return "0"
	}
	return f.value
}

// scratchColumn returns the expression that reads Map.HasScratchKey, or 0 when the column is missing.
func scratchColumn(db *sql.DB) (string, error) {
	rows, err := db.Query(`SELECT name FROM pragma_table_info('Map') WHERE name = 'HasScratchKey'`)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	found := rows.Next()
	if err := rows.Err(); err != nil {
		return "", err
	}
	if found {
		return "COALESCE(m.HasScratchKey,0)", nil
	}
	return "0", nil
}

// modeKeys maps a Quaver GameMode to its lane count: 1 and 2 are 4K and 7K, 3 to 10 are the
// later 1K, 2K, 3K, 5K, 6K, 8K, 9K and 10K modes.
var modeKeys = [...]int{0, 4, 7, 1, 2, 3, 5, 6, 8, 9, 10}

// keysFor returns the lane count of a mode, or 0 when the mode is unknown.
func keysFor(mode int) int {
	if mode < 0 || mode >= len(modeKeys) {
		return 0
	}
	return modeKeys[mode]
}

// lookupBusy and apiBusy are how long a read waits for Quaver's write lock: score lookups wait
// longer than the dashboard, which can simply be asked again.
const (
	lookupBusy = 3 * time.Second
	apiBusy    = time.Second
)

// maxClockSkew is how far ahead of the clock a LastTimePlayed may be before the row is ignored.
const maxClockSkew = 5 * time.Minute

// LastPlayedMap returns the most recently played map, or ErrNotFound. Rows dated more than
// maxClockSkew in the future are ignored.
func (s *Store) LastPlayedMap(ctx context.Context) (Map, error) {
	m, err := s.lastPlayedMap(ctx)
	return m, s.track(err)
}

func (s *Store) lastPlayedMap(ctx context.Context) (Map, error) {
	var m Map
	var mode, scratch int
	limit := s.now().Add(maxClockSkew).UnixMilli()
	s.warnFutureMap(ctx, limit)
	err := s.db.QueryRowContext(ctx,
		`SELECT `+s.mapCols()+` FROM Map m WHERE COALESCE(m.LastTimePlayed,0) <= ? ORDER BY m.LastTimePlayed DESC LIMIT 1`, limit,
	).Scan(&m.MD5, &m.Artist, &m.Title, &m.DifficultyName, &m.Difficulty, &mode, &m.LastPlayedMS, &scratch)
	if errors.Is(err, sql.ErrNoRows) {
		return Map{}, ErrNotFound
	}
	if err != nil {
		return Map{}, fmt.Errorf("read last played map: %w", err)
	}
	m.Keys = keysFor(mode)
	m.Scratch = m.Keys > 0 && scratch != 0
	return m, nil
}

func (s *Store) warnFutureMap(ctx context.Context, limit int64) {
	var newest int64
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(LastTimePlayed),0) FROM Map`).Scan(&newest); err == nil && newest > limit && s.warnedFuture.Swap(newest) != newest {
		slog.Warn("quaver.db has a map dated in the future; ignoring it", "ahead_s", (newest-limit)/1000+int64(maxClockSkew/time.Second))
	}
}

// NewestScore returns the latest saved score, skipping rows whose map is dated in the future,
// or ErrNotFound.
func (s *Store) NewestScore(ctx context.Context) (Result, error) {
	r, err := s.newestScore(ctx)
	return r, s.track(err)
}

func (s *Store) newestScore(ctx context.Context) (Result, error) {
	var id int64
	err := s.db.QueryRowContext(ctx,
		`SELECT s.Id FROM Score s LEFT JOIN Map m ON m.Md5Checksum = s.MapMd5
		 WHERE COALESCE(m.LastTimePlayed,0) <= ? ORDER BY s.Id DESC LIMIT 1`, s.now().Add(maxClockSkew).UnixMilli(),
	).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return Result{}, ErrNotFound
	}
	if err != nil {
		return Result{}, fmt.Errorf("read newest score: %w", err)
	}
	return s.ScoreByID(ctx, id)
}

// MaxScoreID returns the highest score id, or 0 when there is none. Unlike NewestScore it does
// not skip rows of maps dated in the future.
func (s *Store) MaxScoreID(ctx context.Context) (int64, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(Id),0) FROM Score`).Scan(&id)
	if err != nil {
		err = fmt.Errorf("read highest score id: %w", s.track(err))
	} else {
		s.track(nil)
	}
	return id, err
}

// ScoreByID returns a score with its map, or ErrNotFound.
func (s *Store) ScoreByID(ctx context.Context, id int64) (Result, error) {
	r, err := s.scoreByID(ctx, id)
	return r, s.track(err)
}

func (s *Store) scoreByID(ctx context.Context, id int64) (Result, error) {
	rw, err := scanScore(s.db.QueryRowContext(ctx, s.scoreSelect()+` WHERE s.Id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Result{}, ErrNotFound
	}
	if err != nil {
		return Result{}, fmt.Errorf("read score %d: %w", id, err)
	}
	r := rw.result()
	s.applyRateDifficulty(ctx, &r)

	var prevCount int
	var prevMax int64
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*), COALESCE(MAX(TotalScore),0) FROM Score WHERE MapMd5 = ? AND Id < ? AND Grade <> `+strconv.Itoa(failedGrade)+` AND COALESCE(Mods,0) & ? = 0 AND COALESCE(Mods,0) & ? = ?`,
		rw.mapMD5, id, disqualifyMask, keyMask, r.Score.ModsRaw&keyMask,
	).Scan(&prevCount, &prevMax); err != nil {
		return Result{}, fmt.Errorf("read previous scores of %q: %w", rw.mapMD5, err)
	}
	applyPrevious(&r, prevCount, prevMax)
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
