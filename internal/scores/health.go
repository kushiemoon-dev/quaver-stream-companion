package scores

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"modernc.org/sqlite"
)

const (
	// maxFailures is how many read errors in a row mark a store as unhealthy.
	maxFailures = 3
	sqliteBusy  = 5
	sqliteLock  = 6
)

// idleTime is how long an unused connection stays open. Closing idle ones keeps quaver.db free
// for a backup restore on Windows while nothing is reading.
var idleTime = 30 * time.Second

// ErrBusy marks errors caused by Quaver holding a write lock on the database.
var ErrBusy = errors.New("quaver.db is busy")

// IsBusy tells that err comes from Quaver holding a write lock on the database.
func IsBusy(err error) bool {
	if errors.Is(err, ErrBusy) {
		return true
	}
	var se *sqlite.Error
	if !errors.As(err, &se) {
		return false
	}
	code := se.Code() & 0xff
	return code == sqliteBusy || code == sqliteLock
}

// track counts consecutive read errors; a missing row, a lock and a canceled request do not count.
func (s *Store) track(err error) error {
	switch {
	case err == nil || errors.Is(err, ErrNotFound) || errors.Is(err, ErrNoTimeline):
		s.failures.Store(0)
	case IsBusy(err):
		return fmt.Errorf("%w: %w", ErrBusy, err)
	case errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded):
	default:
		s.failures.Add(1)
	}
	return err
}

// Healthy is false after three read errors in a row; the store should then be reopened.
func (s *Store) Healthy() bool { return s.failures.Load() < maxFailures }

// Replaced tells that quaver.db is now another file than the one this store opened, as after a
// backup restore.
func (s *Store) Replaced() bool {
	cur, err := fileIdentity(s.path)
	return err == nil && s.ident != nil && !os.SameFile(s.ident, cur)
}

// fileIdentity stats an open handle: on Windows only that gives a stable file identity.
func fileIdentity(path string) (os.FileInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return f.Stat()
}
