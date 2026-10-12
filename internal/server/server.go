package server

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"time"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/api"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/bus"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/state"
)

const keepAliveEvery = 15 * time.Second

type settings struct {
	api       http.Handler
	port      int
	dashboard fs.FS
}

// Option customizes New.
type Option func(*settings)

// WithAPI mounts h on /api/ and checks the Host header of every request on
// the whole server against the given port.
func WithAPI(h http.Handler, port int) Option {
	return func(s *settings) { s.api, s.port = h, port }
}

// WithDashboard serves the files of dashboard on /.
func WithDashboard(dashboard fs.FS) Option {
	return func(s *settings) { s.dashboard = dashboard }
}

// New returns the HTTP handler. The overlay routes are read-only. CORS is open
// on them because the data is not sensitive and OBS pages may load from other origins.
func New(b *bus.Bus, snap func() state.Snapshot, assets fs.FS, opts ...Option) http.Handler {
	var cfg settings
	for _, o := range opts {
		o(&cfg)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /state", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(snap())
	})
	mux.HandleFunc("GET /events", events(b, snap))
	files := http.FileServerFS(assets)
	mux.Handle("GET /overlays/", files)
	mux.Handle("GET /themes/", files)
	if cfg.dashboard != nil {
		mux.Handle("/", withCSP(readOnly(http.FileServerFS(cfg.dashboard))))
	}
	if cfg.api == nil {
		return mux
	}
	mux.Handle("/api/", cfg.api)
	return api.GuardHost(cfg.port, mux)
}

func events(b *bus.Bus, snap func() state.Snapshot) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fl, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		h := w.Header()
		h.Set("Content-Type", "text/event-stream")
		h.Set("Cache-Control", "no-cache")
		h.Set("Access-Control-Allow-Origin", "*")

		// Subscribe before sending the snapshot so no event is lost in between.
		ch, unsub := b.Subscribe()
		defer unsub()

		writeEvent(w, bus.Event{V: bus.SchemaVersion, Type: bus.TypeSnapshot, Time: time.Now().UTC(), Data: snap()})
		fl.Flush()

		keepAlive := time.NewTicker(keepAliveEvery)
		defer keepAlive.Stop()
		for {
			select {
			case <-r.Context().Done():
				return
			case e, ok := <-ch:
				if !ok {
					return
				}
				writeEvent(w, e)
				fl.Flush()
			case <-keepAlive.C:
				fmt.Fprint(w, ": ping\n\n")
				fl.Flush()
			}
		}
	}
}

func writeEvent(w io.Writer, e bus.Event) {
	raw, err := json.Marshal(e)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", e.Type, raw)
}

// dashboardCSP restricts the dashboard to its own files. The overlays are not covered because OBS pages use inline scripts.
const dashboardCSP = "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; frame-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'"

func withCSP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", dashboardCSP)
		next.ServeHTTP(w, r)
	})
}

func readOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		next.ServeHTTP(w, r)
	})
}
