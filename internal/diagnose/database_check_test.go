package diagnose

import (
	"context"
	"testing"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/testutil"
)

func TestDatabaseCheckFailsWhenTheMapTableIsMissing(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	d.Exec(t, "DROP TABLE Map")

	for _, c := range Run(context.Background(), Options{QuaverDir: d.Path, Port: freePort(t)}) {
		if c.ID == "database" && (c.OK || c.Detail == "") {
			t.Fatalf("database check = %+v, want a failure with a reason", c)
		}
	}
}

func TestCutRunesToZeroBytesNeverIndexesBelowTheStart(t *testing.T) {
	if got := cutRunes("\x80abc", 0); got != " [cut]" {
		t.Fatalf("got %q, want only the cut marker", got)
	}
}
