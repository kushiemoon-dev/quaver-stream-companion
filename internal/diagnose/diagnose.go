// Package diagnose runs the health checks shared by `qsc doctor` and the dashboard.
package diagnose

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/paths"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/scores"
)

// Check is the result of one diagnostic.
type Check struct {
	ID     string `json:"id"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

// Options selects what to inspect.
type Options struct {
	QuaverDir     string
	SteamRoots    []string
	Port          int
	PortOwnedByUs bool
}

const dirNotFound = "Quaver folder not found"

// Run returns the checks quaver_dir, log, database and port, in that order.
func Run(ctx context.Context, o Options) []Check {
	dir, err := paths.Find(o.QuaverDir, o.SteamRoots)
	checks := []Check{result("quaver_dir", err, dir)}
	if err != nil {
		checks = append(checks,
			Check{ID: "log", Detail: dirNotFound},
			Check{ID: "database", Detail: dirNotFound})
	} else {
		logPath := filepath.Join(dir, "Logs", "runtime.log")
		_, statErr := os.Stat(logPath)
		checks = append(checks,
			result("log", statErr, logPath),
			result("database", checkDatabase(ctx, dir), "readable"))
	}
	return append(checks, checkPort(o))
}

func result(id string, err error, okDetail string) Check {
	if err != nil {
		return Check{ID: id, Detail: err.Error()}
	}
	return Check{ID: id, OK: true, Detail: okDetail}
}

func checkDatabase(ctx context.Context, dir string) error {
	st, err := scores.Open(dir)
	if err != nil {
		return err
	}
	defer st.Close()
	if _, err := st.LastPlayedMap(ctx); err != nil && !errors.Is(err, scores.ErrNotFound) {
		return err
	}
	return nil
}

func checkPort(o Options) Check {
	if o.PortOwnedByUs {
		return Check{ID: "port", OK: true, Detail: "in use by this qsc"}
	}
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", o.Port))
	if err == nil {
		ln.Close()
	}
	return result("port", err, "free")
}
