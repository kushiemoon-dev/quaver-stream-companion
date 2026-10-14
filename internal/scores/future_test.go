package scores

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/testutil"
)

func TestLastPlayedMapIgnoresRowsFarInTheFuture(t *testing.T) {
	now := time.UnixMilli(1_800_000_000_000)
	d := testutil.NewQuaverDir(t)
	d.AddMap(t, testutil.MapRow{MD5: "real", Title: "Real", LastPlayedMS: now.UnixMilli() - 1000})
	d.AddMap(t, testutil.MapRow{MD5: "soon", Title: "Soon", LastPlayedMS: now.Add(4 * time.Minute).UnixMilli()})
	d.AddMap(t, testutil.MapRow{MD5: "bogus", Title: "Bogus", LastPlayedMS: now.Add(24 * time.Hour).UnixMilli()})
	st := open(t, d)
	st.now = func() time.Time { return now }

	m, err := st.LastPlayedMap(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if m.MD5 != "soon" {
		t.Fatalf("map = %+v, want the newest row within 5 minutes", m)
	}
}

func TestLastPlayedMapWithOnlyFutureRowsIsNotFound(t *testing.T) {
	now := time.UnixMilli(1_800_000_000_000)
	d := testutil.NewQuaverDir(t)
	d.AddMap(t, testutil.MapRow{MD5: "bogus", LastPlayedMS: now.Add(time.Hour).UnixMilli()})
	st := open(t, d)
	st.now = func() time.Time { return now }

	if _, err := st.LastPlayedMap(context.Background()); err == nil {
		t.Fatal("a future-only table must not give a map")
	}
}

func TestNewestScoreSkipsRowsOfFutureMaps(t *testing.T) {
	now := time.UnixMilli(1_800_000_000_000)
	d := testutil.NewQuaverDir(t)
	d.AddMap(t, testutil.MapRow{MD5: "ok", Title: "Ok", Mode: 1, LastPlayedMS: now.UnixMilli() - 5000})
	d.AddMap(t, testutil.MapRow{MD5: "future", Title: "Future", Mode: 1, LastPlayedMS: now.Add(time.Hour).UnixMilli()})
	good := d.AddScore(t, testutil.ScoreRow{MapMD5: "ok", Total: 1000})
	d.AddScore(t, testutil.ScoreRow{MapMD5: "future", Total: 2000})
	st := open(t, d)
	st.now = func() time.Time { return now }

	r, err := st.NewestScore(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if r.Score.ID != good || r.Map.Title != "Ok" {
		t.Fatalf("result = %+v", r)
	}
}

func TestNewestScoreOnEmptyTableIsNotFound(t *testing.T) {
	if _, err := open(t, testutil.NewQuaverDir(t)).NewestScore(context.Background()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
}

type lineCounter struct {
	mu sync.Mutex
	n  int
}

func (c *lineCounter) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.n += strings.Count(string(p), "dated in the future")
	return len(p), nil
}

func TestFutureMapIsWarnedAboutOncePerRow(t *testing.T) {
	counter := &lineCounter{}
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(counter, nil)))
	t.Cleanup(func() { slog.SetDefault(old) })
	now := time.UnixMilli(1_800_000_000_000)
	d := testutil.NewQuaverDir(t)
	d.AddMap(t, testutil.MapRow{MD5: "ok", LastPlayedMS: now.UnixMilli() - 1000})
	d.AddMap(t, testutil.MapRow{MD5: "bogus", LastPlayedMS: now.Add(time.Hour).UnixMilli()})
	st := open(t, d)
	st.now = func() time.Time { return now }

	for range 5 {
		st.LastPlayedMap(context.Background())
	}
	if counter.n != 1 {
		t.Fatalf("%d warnings for one row", counter.n)
	}
	d.AddMap(t, testutil.MapRow{MD5: "bogus2", LastPlayedMS: now.Add(2 * time.Hour).UnixMilli()})
	st.LastPlayedMap(context.Background())
	if counter.n != 2 {
		t.Fatalf("%d warnings after a second distinct row", counter.n)
	}
}

func TestMaxScoreIDIgnoresTheDateOfTheMap(t *testing.T) {
	now := time.UnixMilli(1_800_000_000_000)
	d := testutil.NewQuaverDir(t)
	d.AddMap(t, testutil.MapRow{MD5: "ok", Mode: 1, LastPlayedMS: now.UnixMilli() - 5000})
	d.AddMap(t, testutil.MapRow{MD5: "future", Mode: 1, LastPlayedMS: now.Add(time.Hour).UnixMilli()})
	d.AddScore(t, testutil.ScoreRow{MapMD5: "ok", Total: 1})
	want := d.AddScore(t, testutil.ScoreRow{MapMD5: "future", Total: 2})
	st := open(t, d)
	st.now = func() time.Time { return now }

	got, err := st.MaxScoreID(context.Background())
	if err != nil || got != want {
		t.Fatalf("max = %d, err = %v, want %d", got, err, want)
	}
	if empty, err := open(t, testutil.NewQuaverDir(t)).MaxScoreID(context.Background()); err != nil || empty != 0 {
		t.Fatalf("empty table: %d, %v", empty, err)
	}
}
