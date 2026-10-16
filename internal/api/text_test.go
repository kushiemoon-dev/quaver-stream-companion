package api

import (
	"net/http"
	"testing"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/scores"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/state"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/textout"
)

func withText(m *scores.Map, r *scores.Result) opt {
	return func(d *Deps) {
		d.Snapshot = func() state.Snapshot { return state.Snapshot{Game: "menu", Map: m, LastScore: r} }
	}
}

func TestTextEndpointServesEveryFile(t *testing.T) {
	m := &scores.Map{Artist: "Ann", Title: "Song", DifficultyName: "Hard", Creator: "Mia"}
	e := newEnv(t, withText(m, &scores.Result{Score: scores.Score{Grade: "A", Accuracy: 91.5}}))
	want := map[string]string{"title.txt": "Song", "artist.txt": "Ann", "difficulty.txt": "Hard", "mapper.txt": "Mia",
		"nowplaying.txt": "Ann - Song [Hard]", "last_grade.txt": "A", "last_accuracy.txt": "91.50%"}
	if len(want) != len(textout.Names) {
		t.Fatal("the test must cover every file")
	}
	for name, body := range want {
		w := e.get("/api/text/" + name)
		if w.Code != 200 || w.Body.String() != body {
			t.Errorf("%s: code %d body %q, want %q", name, w.Code, w.Body.String(), body)
		}
		h := w.Header()
		if h.Get("Content-Type") != "text/plain; charset=utf-8" || h.Get("Cache-Control") != "no-store" || h.Get("X-Content-Type-Options") != "nosniff" {
			t.Errorf("%s: headers = %v", name, h)
		}
	}
}

func TestTextEndpointIsEmptyWithoutAMap(t *testing.T) {
	e := newEnv(t, withText(nil, nil))
	w := e.get("/api/text/nowplaying.txt")
	if w.Code != 200 || w.Body.Len() != 0 {
		t.Fatalf("code %d body %q", w.Code, w.Body.String())
	}
}

func TestTextEndpointRejectsUnknownNamesMethodsAndHosts(t *testing.T) {
	e := newEnv(t, withText(&scores.Map{Title: "Song"}, nil))
	for _, p := range []string{"/api/text/", "/api/text/title", "/api/text/TITLE.TXT", "/api/text/title.txt/", "/api/text/..%2Fconfig.json", "/api/text/%2e%2e/config.json", "/api/text/session_plays.txt", "/api/text/title.txt/x"} {
		if w := e.get(p); w.Code != http.StatusNotFound {
			t.Errorf("%s gave %d", p, w.Code)
		}
	}
	if w := do(e.h, "POST", "127.0.0.1:5555", "/api/text/title.txt", "", map[string]string{"X-Requested-With": "qsc", "Content-Type": "application/json"}); w.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST gave %d", w.Code)
	}
	if w := do(e.h, "GET", "evil.example:5555", "/api/text/title.txt", "", nil); w.Code != http.StatusForbidden {
		t.Errorf("foreign host gave %d", w.Code)
	}
	if w := do(e.h, "HEAD", "127.0.0.1:5555", "/api/text/title.txt", "", nil); w.Code != 200 || w.Body.Len() != 0 {
		t.Errorf("HEAD gave %d with %d bytes", w.Code, w.Body.Len())
	}
}

func TestTextEndpointStripsControlCharacters(t *testing.T) {
	e := newEnv(t, withText(&scores.Map{Title: "A\r\nB\x00C"}, nil))
	if got := e.get("/api/text/title.txt").Body.String(); got != "ABC" {
		t.Fatalf("title = %q", got)
	}
}
