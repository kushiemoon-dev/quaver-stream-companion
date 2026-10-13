// Package diagnose runs the health checks shared by `qsc doctor` and the dashboard.
package diagnose

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/keys"
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
	// KeysEnabled adds the keys check; it is left out while lane key sampling is off.
	KeysEnabled bool
}

const dirNotFound = "Quaver folder not found"

// Run returns the checks quaver_dir, log, database and port, in that order, then keys when enabled.
func Run(ctx context.Context, o Options) []Check {
	_, checks := folderChecks(ctx, o.QuaverDir, o.SteamRoots)
	checks = append(checks, checkPort(o))
	if o.KeysEnabled {
		checks = append(checks, checkKeys())
	}
	return checks
}

// Folder runs the quaver_dir, log and database checks. It returns the Quaver folder only
// when all three pass, which is when the log follower and the score reader can start.
func Folder(ctx context.Context, override string, steamRoots []string) (string, []Check) {
	dir, checks := folderChecks(ctx, override, steamRoots)
	for _, c := range checks {
		if !c.OK {
			return "", checks
		}
	}
	return dir, checks
}

func folderChecks(ctx context.Context, override string, steamRoots []string) (string, []Check) {
	dir, err := paths.Find(override, steamRoots)
	checks := []Check{result("quaver_dir", err, dir)}
	if err != nil {
		return "", append(checks,
			Check{ID: "log", Detail: dirNotFound},
			Check{ID: "database", Detail: dirNotFound})
	}
	logPath := paths.LogPath(dir)
	_, statErr := os.Stat(logPath)
	return dir, append(checks,
		result("log", statErr, logPath),
		result("database", checkDatabase(ctx, dir), "readable"))
}

// Advice lists the steps that fix a missing Quaver folder, for `qsc doctor` and the startup message.
func Advice() []string {
	return []string{
		`Steam usually installs Quaver in C:\Program Files (x86)\Steam\steamapps\common\Quaver on Windows,`,
		"and in ~/.local/share/Steam/steamapps/common/Quaver on Linux, where Quaver runs through Proton.",
		"Flatpak and snap Steam use their own Steam library folders under ~/.var/app/com.valvesoftware.Steam and ~/snap/steam.",
		"Where Proton keeps Quaver files outside the game folder is not verified: qsc reads the game folder only.",
		"Install and start Quaver once, or set its folder in the dashboard Settings,",
		"with quaver_dir in the config file, or with --quaver-dir <folder>.",
	}
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

func checkKeys() Check {
	ok := keys.Supported()
	detail := "enabled"
	if !ok {
		_, err := keys.NewSampler()
		detail = err.Error()
	}
	return Check{ID: "keys", OK: ok, Detail: detail}
}
