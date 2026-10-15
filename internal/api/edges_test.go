package api

import (
	"context"
	"strings"
	"testing"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/scores"
)

// The smallest and largest history limits are accepted; TestHistoryBadLimit pins the refusals.
func TestHistoryLimitAcceptsTheBounds(t *testing.T) {
	var got []int
	e := newEnv(t, func(d *Deps) {
		d.History = func(ctx context.Context, limit int) ([]scores.Result, error) {
			got = append(got, limit)
			return nil, nil
		}
	})
	for _, q := range []string{"1", "200"} {
		if w := e.get("/api/history?limit=" + q); w.Code != 200 {
			t.Errorf("limit=%s: code %d", q, w.Code)
		}
	}
	if len(got) != 2 || got[0] != 1 || got[1] != 200 {
		t.Fatalf("limits passed on: %v", got)
	}
}

// A body of exactly maxBody bytes is not refused as too large.
func TestBodyOfExactlyMaxBodyIsAccepted(t *testing.T) {
	e := newEnv(t)
	body := `{"language":"` + strings.Repeat("a", maxBody-len(`{"language":""}`)) + `"}`
	if len(body) != maxBody {
		t.Fatalf("test body is %d bytes", len(body))
	}
	if w := e.put(body, nil); w.Code == 413 {
		t.Fatalf("a %d-byte body was refused as too large", maxBody)
	}
}

// Trailing data after the settings object is named as such.
func TestPutRejectsTrailingDataWithAClearMessage(t *testing.T) {
	for _, body := range []string{`{"port":5555} {}`, `{"port":5555} x`} {
		_, err := parseSettings([]byte(body))
		if err == nil || !strings.Contains(err.Error(), "invalid JSON") {
			t.Fatalf("%q: err %v", body, err)
		}
	}
	if _, err := parseSettings([]byte(`{"port":5555} {}`)); !strings.Contains(err.Error(), "unexpected data after the JSON object") {
		t.Fatalf("got %v", err)
	}
}

// An Origin that does not parse is foreign: the diagnostics report is refused, never a crash.
func TestDiagnosticsRefusesAnUnparsableOrigin(t *testing.T) {
	called := false
	e := newEnv(t, func(d *Deps) {
		d.Diagnostics = func(ctx context.Context) string { called = true; return "report" }
	})
	for _, origin := range []string{"http://[::1", "http://%zz", "null"} {
		w := do(e.h, "GET", "127.0.0.1:5555", "/api/diagnostics", "", map[string]string{"Origin": origin})
		if w.Code != 403 || called {
			t.Fatalf("Origin %q: code %d, report built=%v", origin, w.Code, called)
		}
	}
}

// Score id 0 and an overflowing id never reach the timeline source.
func TestTimelineRejectsIDZeroAndOverflow(t *testing.T) {
	var ids []int64
	e := newEnv(t, func(d *Deps) {
		d.Timeline = func(ctx context.Context, id int64) (scores.Timeline, error) {
			ids = append(ids, id)
			return scores.Timeline{}, scores.ErrNotFound
		}
	})
	for _, p := range []string{"/api/scores/0/timeline", "/api/scores/99999999999999999999/timeline", "/api/scores/000/timeline"} {
		if w := e.get(p); w.Code != 404 {
			t.Errorf("%s: code %d", p, w.Code)
		}
	}
	if len(ids) != 0 {
		t.Fatalf("timeline source called with %v", ids)
	}
}
