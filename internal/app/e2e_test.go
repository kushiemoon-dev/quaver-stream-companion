package app

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/bus"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/gamestate"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/scores"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/testutil"
)

func TestEndToEndWithFakeQuaverFolder(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	st, err := scores.Open(d.Path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })

	b := bus.New()
	events, unsub := b.Subscribe()
	t.Cleanup(unsub)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	lines := make(chan string, 16)
	go gamestate.Tail(ctx, d.LogPath(), 10*time.Millisecond, lines)
	a := New(b, st)
	a.retry = 10 * time.Millisecond
	go a.Run(ctx, lines)
	time.Sleep(60 * time.Millisecond) // let Tail take its baseline

	d.AddMap(t, testutil.MapRow{MD5: "m1", Artist: "A", Title: "T", DiffName: "hard", Mode: 1, LastPlayedMS: time.Now().UnixMilli(), Difficulty: 15.7})
	d.AppendLog(t, lineGameplay)

	if gs := next(t, events, bus.TypeGameState).Data.(bus.GameState); gs.State != "gameplay" {
		t.Fatalf("state = %q", gs.State)
	}
	if m := next(t, events, bus.TypeMapStarted).Data.(scores.Map); m.Title != "T" || m.Keys != 4 {
		t.Fatalf("map = %+v", m)
	}

	id := d.AddScore(t, testutil.ScoreRow{MapMD5: "m1", Total: 900000, Grade: 7, Accuracy: 97.5, MaxCombo: 500, Marv: 400, Perf: 90, Great: 10})
	d.AppendLog(t, "[1:06:06] - RUNTIME - IMPORTANT: Successfully saved score to database with id: "+strconv.FormatInt(id, 10))

	r := next(t, events, bus.TypeScoreSaved).Data.(scores.Result)
	if !r.FirstPlay || !r.FullCombo || r.Score.Grade != "SS" || r.Map.Title != "T" {
		t.Fatalf("result = %+v", r)
	}
}
