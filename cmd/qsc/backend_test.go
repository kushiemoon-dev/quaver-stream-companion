package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/scores"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/testutil"
)

func TestBackendReopensAReplacedDatabase(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	d.AddMap(t, testutil.MapRow{MD5: "m", Title: "Before", Mode: 1, LastPlayedMS: 1})
	store, err := scores.Open(d.Path)
	if err != nil {
		t.Fatal(err)
	}
	be := &backend{}
	be.set(d.Path, store)
	forgot := false
	be.onReplaced = func() { forgot = true }
	defer be.close()
	if be.problem() != "" {
		t.Fatalf("problem = %q on a fresh store", be.problem())
	}

	if err := be.replaceFile(t, d); err != nil {
		t.Skipf("cannot replace the database on this system: %v", err)
	}
	if be.problem() == "" {
		t.Fatal("a replaced database file must be reported")
	}
	if err := be.reopen(); err != nil {
		t.Fatal(err)
	}
	if be.problem() != "" {
		t.Fatalf("problem = %q after reopening", be.problem())
	}
	if !forgot {
		t.Fatal("the app must be told that the database was replaced")
	}
	m, err := be.LastPlayedMap(context.Background())
	if err != nil || m.Title != "After" {
		t.Fatalf("map = %+v, err = %v", m, err)
	}
}

// replaceFile swaps quaver.db for another database after closing the old connections.
func (b *backend) replaceFile(t *testing.T, d *testutil.FakeDir) error {
	t.Helper()
	_, s := b.current()
	s.Close()
	other := testutil.NewQuaverDir(t)
	other.AddMap(t, testutil.MapRow{MD5: "n", Title: "After", Mode: 1, LastPlayedMS: 2})
	return os.Rename(filepath.Join(other.Path, "quaver.db"), filepath.Join(d.Path, "quaver.db"))
}

func TestBackendWithoutStoreIsUnavailable(t *testing.T) {
	be := &backend{}
	if _, err := be.LastPlayedMap(context.Background()); err == nil {
		t.Fatal("no store must be an error")
	}
	if be.problem() != "" {
		t.Fatal("nothing to supervise yet")
	}
}

func TestRelayPassesLinesAndReportsEachOne(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	in, out := make(chan string), make(chan string, 4)
	seen := 0
	go relay(ctx, in, out, func() { seen++ })

	in <- "a"
	in <- "b"
	if got := <-out + <-out; got != "ab" {
		t.Fatalf("got %q", got)
	}
	if seen != 2 {
		t.Fatalf("seen = %d", seen)
	}
}
