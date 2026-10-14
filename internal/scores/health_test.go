package scores

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/testutil"
)

func lockDatabase(t *testing.T, d *testutil.FakeDir) {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(filepath.Join(d.Path, "quaver.db")))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("BEGIN EXCLUSIVE"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Exec("ROLLBACK"); db.Close() })
}

func TestBusyDatabaseIsRecognizedAndNotCountedAsFailure(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	d.AddMap(t, testutil.MapRow{MD5: "m", Title: "T", Mode: 1, LastPlayedMS: 1})
	id := d.AddScore(t, testutil.ScoreRow{MapMD5: "m", Total: 1})
	st := open(t, d)
	st.db.SetMaxOpenConns(1)
	if _, err := st.db.Exec("PRAGMA busy_timeout=0"); err != nil {
		t.Fatal(err)
	}
	lockDatabase(t, d)

	for range 5 {
		_, err := st.ScoreByID(context.Background(), id)
		if !IsBusy(err) {
			t.Fatalf("err = %v, want a busy error", err)
		}
	}
	if !st.Healthy() {
		t.Fatal("a database that Quaver is writing is busy, not broken")
	}
	if IsBusy(nil) || IsBusy(ErrNotFound) {
		t.Fatal("IsBusy matched an error that is not a lock")
	}
}

func TestStoreTurnsUnhealthyAfterThreeReadErrorsInARow(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	d.AddMap(t, testutil.MapRow{MD5: "m", Title: "T", Mode: 1, LastPlayedMS: 1})
	st := open(t, d)
	ctx := context.Background()

	st.db.Close()
	for i := 0; i < 2; i++ {
		st.LastPlayedMap(ctx)
	}
	if !st.Healthy() {
		t.Fatal("two errors are not enough")
	}
	st.LastPlayedMap(ctx)
	if st.Healthy() {
		t.Fatal("three errors in a row must report the store as unhealthy")
	}
}

func TestSuccessfulReadsResetTheErrorCount(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	d.AddMap(t, testutil.MapRow{MD5: "m", Title: "T", Mode: 1, LastPlayedMS: 1})
	st := open(t, d)
	st.failures.Store(2)

	if _, err := st.LastPlayedMap(context.Background()); err != nil {
		t.Fatal(err)
	}
	st.failures.Add(1)
	if !st.Healthy() {
		t.Fatal("a good read must have started the count over")
	}
}

func TestStoreNoticesAReplacedDatabaseFile(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	d.AddMap(t, testutil.MapRow{MD5: "m", Title: "T", Mode: 1, LastPlayedMS: 1})
	st := open(t, d)
	st.db.SetMaxIdleConns(0) // nothing keeps the file open, as after the idle timeout
	if _, err := st.LastPlayedMap(context.Background()); err != nil {
		t.Fatal(err)
	}
	if st.Replaced() {
		t.Fatal("the file was not replaced yet")
	}

	other := testutil.NewQuaverDir(t)
	other.AddMap(t, testutil.MapRow{MD5: "n", Title: "Restored", Mode: 1, LastPlayedMS: 2})
	if err := os.Rename(filepath.Join(other.Path, "quaver.db"), filepath.Join(d.Path, "quaver.db")); err != nil {
		t.Skipf("cannot replace the database on this system: %v", err)
	}

	if !st.Replaced() {
		t.Fatal("a database file swapped for another one must be noticed")
	}
	fresh := open(t, d)
	if m, err := fresh.LastPlayedMap(context.Background()); err != nil || m.Title != "Restored" {
		t.Fatalf("map = %+v, err = %v", m, err)
	}
}

func TestIdleConnectionsAreClosed(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	d.AddMap(t, testutil.MapRow{MD5: "m", Title: "T", Mode: 1, LastPlayedMS: 1})
	old := idleTime
	idleTime = 50 * time.Millisecond
	t.Cleanup(func() { idleTime = old })
	st := open(t, d)
	if _, err := st.LastPlayedMap(context.Background()); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for st.db.Stats().OpenConnections > 0 {
		if time.Now().After(deadline) {
			t.Fatalf("%d connections still open while idle", st.db.Stats().OpenConnections)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestApiReadsGiveUpOnALockedDatabaseAfterAboutOneSecond(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	d.AddMap(t, testutil.MapRow{MD5: "m", Title: "T", Mode: 1, LastPlayedMS: 1})
	id := d.AddScore(t, testutil.ScoreRow{MapMD5: "m", Total: 1})
	st := open(t, d)
	lockDatabase(t, d)

	start := time.Now()
	_, err := st.Recent(context.Background(), 10)
	took := time.Since(start)
	if !IsBusy(err) || took < 700*time.Millisecond || took > 2*time.Second {
		t.Fatalf("Recent: err = %v after %v, want a busy error after about 1s", err, took)
	}
	start = time.Now()
	_, err = st.Timeline(context.Background(), id)
	took = time.Since(start)
	if !IsBusy(err) || took > 2*time.Second {
		t.Fatalf("Timeline: err = %v after %v", err, took)
	}
}

func TestScoreLookupsStillWaitThreeSecondsForALock(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	d.AddMap(t, testutil.MapRow{MD5: "m", Title: "T", Mode: 1, LastPlayedMS: 1})
	id := d.AddScore(t, testutil.ScoreRow{MapMD5: "m", Total: 1})
	st := open(t, d)
	lockDatabase(t, d)

	start := time.Now()
	_, err := st.ScoreByID(context.Background(), id)
	took := time.Since(start)
	if !IsBusy(err) || took < 2500*time.Millisecond || took > 5*time.Second {
		t.Fatalf("ScoreByID: err = %v after %v, want a busy error after about 3s", err, took)
	}
}
