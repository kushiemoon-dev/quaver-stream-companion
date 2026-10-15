package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/config"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/themes"
)

const fuzzPort = 5555

// FuzzGuardHost: the wrapped handler runs only for this server's Host, and a mutating request only with
// the custom header, a matching or absent Origin and a JSON content type.
func FuzzGuardHost(f *testing.F) {
	f.Add("PUT", "127.0.0.1:5555", "http://127.0.0.1:5555", "qsc", "application/json; charset=utf-8", 2)
	f.Add("POST", "LOCALHOST:5555", "", "qsc", "application/json", 0)
	f.Add("GET", "evil.com", "http://evil.com", "", "", 0)
	f.Add("DELETE", "127.0.0.1:5555", "null", "qsc", "application/json", 1)
	f.Add("PUT", "127.0.0.1:5555", "http://127.0.0.1:5555", "qsc", "text/plain", 1)
	f.Fuzz(func(t *testing.T, method, host, origin, xrw, ctype string, origins int) {
		if method == "" {
			method = http.MethodGet // what net/http does with an empty method
		}
		req, err := http.NewRequest(method, "http://x/api/config", strings.NewReader("{}"))
		if err != nil {
			return
		}
		req.Host = host
		n := origins % 4
		if n < 0 {
			n = -n
		}
		for i := 0; i < n; i++ {
			req.Header.Add("Origin", origin)
		}
		if xrw != "" {
			req.Header.Set("X-Requested-With", xrw)
		}
		if ctype != "" {
			req.Header.Set("Content-Type", ctype)
		}
		called := false
		h := GuardHost(fuzzPort, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		hostOK := host == "127.0.0.1:5555" || strings.EqualFold(host, "localhost:5555")
		if !hostOK {
			if called || w.Code != http.StatusForbidden {
				t.Fatalf("host %q let through (called=%v code=%d)", host, called, w.Code)
			}
			return
		}
		switch method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			if !called {
				t.Fatalf("safe %s refused: %d", method, w.Code)
			}
			return
		}
		if !called {
			return
		}
		if xrw != "qsc" || (n > 1) || (n == 1 && origin != "http://"+host) {
			t.Fatalf("mutating request let through: method=%q xrw=%q origins=%d origin=%q host=%q", method, xrw, n, origin, host)
		}
		if mt := strings.ToLower(strings.TrimSpace(strings.SplitN(ctype, ";", 2)[0])); mt != "application/json" {
			t.Fatalf("content type %q let through", ctype)
		}
	})
}

// FuzzParseSettings: an accepted document only holds known top-level keys and decodes into a Config.
func FuzzParseSettings(f *testing.F) {
	for _, s := range []string{`{"port":7878}`, `{}`, `{"port":7878} x`, `{"Port":1}`, `{"keys":{"k4":["A","S","K","L"]}}`, `[]`, `null`, `{"version":2}`} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, body []byte) {
		c, err := parseSettings(body)
		if err != nil {
			return
		}
		var raw map[string]json.RawMessage
		if json.Unmarshal(body, &raw) != nil || len(raw) == 0 {
			t.Fatalf("accepted %q", body)
		}
		for k := range raw {
			if !settingsKeys[k] {
				t.Fatalf("accepted unknown key %q", k)
			}
		}
		_ = config.Normalize(c)
	})
}

// FuzzPutConfig: PUT /api/config answers 200, 400, 413 or 415 and never 500; after a 200 the saved
// file reads back as the config the response reported.
func FuzzPutConfig(f *testing.F) {
	for _, s := range []string{`{"port":7878,"language":"fr"}`, `{"overlay":{"theme":"neutral-dark","accent":"ff00ff"}}`,
		`{"keys":{"enabled":true,"layout":"4k","k4":["A","S","K","L"]}}`, `{"port":0}`, `{"port":70000}`, `{"language":"xx"}`} {
		f.Add([]byte(s))
	}
	th := []themes.Theme{{ID: "neutral-light"}, {ID: "neutral-dark"}}
	f.Fuzz(func(t *testing.T, body []byte) {
		if c, err := parseSettings(body); err == nil && c.QuaverDir != "" {
			return // never stat arbitrary folders
		}
		cfgPath := filepath.Join(t.TempDir(), "config.json")
		d := Deps{ConfigPath: cfgPath, Port: fuzzPort, Effective: func() string { return "" },
			Themes: func() []themes.Theme { return th }}
		h := GuardHost(fuzzPort, New(d))
		req := httptest.NewRequest("PUT", "/api/config", strings.NewReader(string(body)))
		req.Host = "127.0.0.1:5555"
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Requested-With", "qsc")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		switch w.Code {
		case 200:
		case 400, 413, 415:
			return
		default:
			t.Fatalf("PUT %q: %d %s", body, w.Code, w.Body.String())
		}
		var resp struct {
			Config config.Config `json:"config"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatal(err)
		}
		saved, err := config.Load(cfgPath)
		if err != nil {
			t.Fatal(err)
		}
		a, _ := json.Marshal(saved)
		b, _ := json.Marshal(resp.Config)
		if string(a) != string(b) {
			t.Fatalf("saved %s, response %s", a, b)
		}
	})
}

// FuzzScoreID: an accepted path is exactly /api/scores/<positive decimal id>/timeline.
func FuzzScoreID(f *testing.F) {
	for _, s := range []string{"/api/scores/42/timeline", "/api/scores/0/timeline", "/api/scores/-1/timeline", "/api/scores/+1/timeline",
		"/api/scores/99999999999999999999/timeline", "/api/scores//timeline", "/api/scores/1/timeline/x", "/api/scores/007/timeline"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, path string) {
		id, ok := scoreID(path)
		if !ok {
			return
		}
		if id <= 0 {
			t.Fatalf("id %d accepted from %q", id, path)
		}
		digits := strings.TrimSuffix(strings.TrimPrefix(path, scoresPrefix), timelineSuffix)
		if path != scoresPrefix+digits+timelineSuffix || strings.TrimLeft(digits, "0") != strconv.FormatInt(id, 10) {
			t.Fatalf("scoreID(%q) = %d", path, id)
		}
	})
}
