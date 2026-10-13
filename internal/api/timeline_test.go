package api

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/scores"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/testutil"
)

func timelineEnv(t *testing.T) (env, int64, int64) {
	t.Helper()
	q := testutil.NewQuaverDir(t)
	q.AddMap(t, testutil.MapRow{MD5: "m1", Mode: 1})
	ok := q.AddScore(t, testutil.ScoreRow{MapMD5: "m1", Breakdown: testutil.EncodeBreakdown("0001125")})
	none := q.AddScore(t, testutil.ScoreRow{MapMD5: "m1"})
	st, err := scores.Open(q.Path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return newEnv(t, func(d *Deps) { d.Timeline = st.Timeline }), ok, none
}

func TestTimelineEndpoint(t *testing.T) {
	e, id, _ := timelineEnv(t)
	w := e.get("/api/scores/" + strconv.FormatInt(id, 10) + "/timeline")
	if w.Code != 200 {
		t.Fatalf("code %d body %s", w.Code, w.Body.String())
	}
	var raw map[string]json.RawMessage
	decode(t, w, &raw)
	for _, k := range []string{"score_id", "total", "points", "misses", "counts"} {
		if _, ok := raw[k]; !ok {
			t.Errorf("missing %q in %s", k, w.Body.String())
		}
	}
	var got scores.Timeline
	decode(t, w, &got)
	if got.ScoreID != id || got.Total != 7 || len(got.Points) != 7 || len(got.Misses) != 1 || got.Misses[0] != 6 {
		t.Fatalf("timeline = %+v", got)
	}
}

func TestTimelineEndpointErrors(t *testing.T) {
	e, _, none := timelineEnv(t)
	for _, c := range []struct {
		method, path string
		want         int
	}{
		{"GET", "/api/scores/9999/timeline", 404},
		{"GET", "/api/scores/" + strconv.FormatInt(none, 10) + "/timeline", 404},
		{"GET", "/api/scores/abc/timeline", 404},
		{"GET", "/api/scores/0/timeline", 404},
		{"GET", "/api/scores/-1/timeline", 404},
		{"GET", "/api/scores/99999999999999999999/timeline", 404},
		{"GET", "/api/scores/1", 404},
		{"GET", "/api/scores/1/timeline/", 404},
		{"GET", "/api/scores//timeline", 404},
		{"POST", "/api/scores/1/timeline", 405},
	} {
		w := do(e.h, c.method, "127.0.0.1:5555", c.path, "{}", map[string]string{"Content-Type": "application/json", "X-Requested-With": "qsc"})
		var er errBody
		decode(t, w, &er)
		if w.Code != c.want || er.Error == "" {
			t.Errorf("%s %s: code %d body %s", c.method, c.path, w.Code, w.Body.String())
		}
	}
}

func TestTimelineEndpointIsSameOriginOnly(t *testing.T) {
	e, id, _ := timelineEnv(t)
	w := do(e.h, "GET", "evil.example:5555", "/api/scores/"+strconv.FormatInt(id, 10)+"/timeline", "", nil)
	if w.Code != 403 {
		t.Fatalf("code %d", w.Code)
	}
}

func TestTimelineUnavailable(t *testing.T) {
	e := newEnv(t, func(d *Deps) { d.Timeline = nil })
	if w := e.get("/api/scores/1/timeline"); w.Code != 500 {
		t.Fatalf("code %d", w.Code)
	}
}
