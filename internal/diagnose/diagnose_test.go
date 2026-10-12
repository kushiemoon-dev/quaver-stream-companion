package diagnose

import (
	"context"
	"net"
	"os"
	"strings"
	"testing"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/keys"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/testutil"
)

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func wantIDs(t *testing.T, checks []Check) {
	t.Helper()
	ids := []string{"quaver_dir", "log", "database", "port"}
	if len(checks) != len(ids) {
		t.Fatalf("got %d checks: %+v", len(checks), checks)
	}
	for i, id := range ids {
		if checks[i].ID != id {
			t.Fatalf("check %d = %q, want %q", i, checks[i].ID, id)
		}
	}
}

func TestAllOK(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	checks := Run(context.Background(), Options{QuaverDir: d.Path, Port: freePort(t)})
	wantIDs(t, checks)
	for _, c := range checks {
		if !c.OK {
			t.Fatalf("not ok: %+v", c)
		}
	}
}

func TestInvalidDir(t *testing.T) {
	checks := Run(context.Background(), Options{QuaverDir: t.TempDir(), Port: freePort(t)})
	wantIDs(t, checks)
	if checks[0].OK || !strings.Contains(checks[0].Detail, "Quaver.exe") {
		t.Fatalf("quaver_dir = %+v", checks[0])
	}
	for _, c := range checks[1:3] {
		if c.OK || c.Detail != "Quaver folder not found" {
			t.Fatalf("check = %+v", c)
		}
	}
	if !checks[3].OK {
		t.Fatalf("port = %+v", checks[3])
	}
}

func TestMissingLog(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	if err := os.Remove(d.LogPath()); err != nil {
		t.Fatal(err)
	}
	checks := Run(context.Background(), Options{QuaverDir: d.Path, Port: freePort(t)})
	if !checks[0].OK || checks[1].OK || !checks[2].OK {
		t.Fatalf("checks = %+v", checks)
	}
}

func TestMissingDatabase(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	if err := os.Remove(d.Path + "/quaver.db"); err != nil {
		t.Fatal(err)
	}
	checks := Run(context.Background(), Options{QuaverDir: d.Path, Port: freePort(t)})
	if !checks[0].OK || !checks[1].OK || checks[2].OK {
		t.Fatalf("checks = %+v", checks)
	}
}

func TestPortBusyAndOwned(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	busy := Run(context.Background(), Options{QuaverDir: d.Path, Port: port})
	if busy[3].OK {
		t.Fatalf("busy port reported ok: %+v", busy[3])
	}
	owned := Run(context.Background(), Options{QuaverDir: d.Path, Port: port, PortOwnedByUs: true})
	if !owned[3].OK || owned[3].Detail != "in use by this qsc" {
		t.Fatalf("owned = %+v", owned[3])
	}
}

func TestKeysCheckOnlyWhenEnabled(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	checks := Run(context.Background(), Options{QuaverDir: d.Path, Port: freePort(t)})
	wantIDs(t, checks)

	checks = Run(context.Background(), Options{QuaverDir: d.Path, Port: freePort(t), KeysEnabled: true})
	if len(checks) != 5 || checks[4].ID != "keys" || checks[4].Detail == "" {
		t.Fatalf("checks = %+v", checks)
	}
	if checks[4].OK != keys.Supported() {
		t.Fatalf("keys check ok = %v, platform support = %v", checks[4].OK, keys.Supported())
	}
}
