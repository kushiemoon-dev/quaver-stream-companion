// Package testutil builds a fake Quaver folder for tests.
package testutil

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

const (
	mapTable   = `CREATE TABLE Map (Id integer primary key autoincrement not null, Md5Checksum varchar, Artist varchar, Title varchar, DifficultyName varchar, LastTimePlayed integer, Mode integer, Difficulty10X float, Difficulty09X float, Difficulty115X float)`
	scoreTable = `CREATE TABLE Score (Id integer primary key autoincrement not null, MapMd5 varchar, TotalScore integer, Grade integer, Accuracy float, MaxCombo integer, CountMarv integer, CountPerf integer, CountGreat integer, CountGood integer, CountOkay integer, CountMiss integer, Mods integer, DateTime varchar, PerformanceRating float, JudgementBreakdown varchar)`
)

// FakeDir is a temporary Quaver folder with a synthetic quaver.db.
type FakeDir struct{ Path string }

// MapRow is a row of the fake Map table.
type MapRow struct {
	MD5, Artist, Title, DiffName string
	Mode                         int
	LastPlayedMS                 int64
	Difficulty                   float64
}

// ScoreRow is a row of the fake Score table.
type ScoreRow struct {
	MapMD5                                        string
	Total                                         int64
	Grade                                         int
	Accuracy, Rating                              float64
	MaxCombo, Marv, Perf, Great, Good, Okay, Miss int
	Mods                                          int64
	Breakdown                                     string
}

// NewQuaverDir creates Quaver.exe, an empty Logs/runtime.log and quaver.db.
func NewQuaverDir(t testing.TB) *FakeDir { return newDir(t, scoreTable) }

// NewLegacyQuaverDir is like NewQuaverDir but its Score table has no JudgementBreakdown column.
func NewLegacyQuaverDir(t testing.TB) *FakeDir {
	return newDir(t, strings.Replace(scoreTable, ", JudgementBreakdown varchar", "", 1))
}

func newDir(t testing.TB, scoreSchema string) *FakeDir {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Quaver.exe"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "Logs"), 0o755); err != nil {
		t.Fatal(err)
	}
	d := &FakeDir{Path: dir}
	if err := os.WriteFile(d.LogPath(), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	d.Exec(t, mapTable)
	d.Exec(t, scoreSchema)
	return d
}

// LogPath returns the path of the fake runtime log.
func (d *FakeDir) LogPath() string { return filepath.Join(d.Path, "Logs", "runtime.log") }

// AppendLog appends lines to the fake runtime log.
func (d *FakeDir) AppendLog(t testing.TB, lines ...string) {
	t.Helper()
	f, err := os.OpenFile(d.LogPath(), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	for _, l := range lines {
		if _, err := f.WriteString(l + "\n"); err != nil {
			t.Fatal(err)
		}
	}
}

func (d *FakeDir) exec(t testing.TB, query string, args ...any) sql.Result {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(filepath.Join(d.Path, "quaver.db")))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	res, err := db.Exec(query, args...)
	if err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
	return res
}

// Exec runs an arbitrary statement against the fake quaver.db.
func (d *FakeDir) Exec(t testing.TB, query string, args ...any) { d.exec(t, query, args...) }

// AddMap inserts a map row.
func (d *FakeDir) AddMap(t testing.TB, m MapRow) {
	t.Helper()
	d.exec(t, `INSERT INTO Map (Md5Checksum, Artist, Title, DifficultyName, LastTimePlayed, Mode, Difficulty10X) VALUES (?,?,?,?,?,?,?)`,
		m.MD5, m.Artist, m.Title, m.DiffName, m.LastPlayedMS, m.Mode, m.Difficulty)
}

// AddScore inserts a score row and returns its id.
func (d *FakeDir) AddScore(t testing.TB, s ScoreRow) int64 {
	t.Helper()
	res := d.exec(t, `INSERT INTO Score (MapMd5, TotalScore, Grade, Accuracy, MaxCombo, CountMarv, CountPerf, CountGreat, CountGood, CountOkay, CountMiss, Mods, DateTime, PerformanceRating, JudgementBreakdown) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,'2026-10-12 13:06:06',?,?)`,
		s.MapMD5, s.Total, s.Grade, s.Accuracy, s.MaxCombo, s.Marv, s.Perf, s.Great, s.Good, s.Okay, s.Miss, s.Mods, s.Rating, s.Breakdown)
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return id
}
