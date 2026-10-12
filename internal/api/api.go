// Package api implements the local JSON API used by the dashboard.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/config"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/diagnose"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/keys"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/overlays"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/scores"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/state"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/themes"
)

const (
	defaultHistory = 50
	maxHistory     = 200
)

// Deps holds what the API needs from the running server.
type Deps struct {
	Version    string
	Started    time.Time
	Snapshot   func() state.Snapshot
	History    func(ctx context.Context, limit int) ([]scores.Result, error)
	ConfigPath string
	// Effective returns the Quaver folder actually in use.
	Effective func() (quaverDir string)
	// Detect returns the Quaver folder auto-detection finds now, or "".
	Detect func() string
	// PortFlag and QuaverDirFlag tell that the setting came from a command-line flag.
	PortFlag, QuaverDirFlag bool
	// Themes lists the selectable overlay themes in display order; it is called per request.
	Themes func() []themes.Theme
	// ThemesDir is the custom themes folder shown to the dashboard.
	ThemesDir string
	// OverlaysDir is the custom overlays folder; IsBuiltinOverlay tells which ids custom overlays may not use.
	OverlaysDir      string
	IsBuiltinOverlay func(id string) bool
	// Keys reports the lane key sampler and KeysConfig holds the settings it started with.
	Keys       func() keys.Status
	KeysConfig keys.Config
	SteamRoots []string
	Port       int
}

type errBody struct {
	Error string `json:"error"`
	Field string `json:"field,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg, field string) {
	writeJSON(w, status, errBody{Error: msg, Field: field})
}

type api struct{ d Deps }

// New returns the handler for every /api/ route. Wrap it with GuardHost.
func New(d Deps) http.Handler {
	a := api{d}
	routes := map[string]map[string]http.HandlerFunc{
		"/api/status":   {http.MethodGet: a.status},
		"/api/config":   {http.MethodGet: a.getConfig, http.MethodPut: a.putConfig},
		"/api/history":  {http.MethodGet: a.history},
		"/api/themes":   {http.MethodGet: a.themes},
		"/api/overlays": {http.MethodGet: a.overlays},
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods, ok := routes[r.URL.Path]
		if !ok {
			writeError(w, http.StatusNotFound, "not found", "")
			return
		}
		h, ok := methods[r.Method]
		if !ok {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed", "")
			return
		}
		h(w, r)
	})
}

func (a api) status(w http.ResponseWriter, r *http.Request) {
	snap := a.d.Snapshot()
	ks := a.keysStatus()
	checks := diagnose.Run(r.Context(), diagnose.Options{
		QuaverDir:     a.d.Effective(),
		SteamRoots:    a.d.SteamRoots,
		Port:          a.d.Port,
		PortOwnedByUs: true,
		KeysEnabled:   ks.Enabled,
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"version":     a.d.Version,
		"game":        snap.Game,
		"map":         snap.Map,
		"last_score":  snap.LastScore,
		"checks":      checks,
		"keys":        ks,
		"config_path": a.d.ConfigPath,
		"uptime_s":    int64(time.Since(a.d.Started).Seconds()),
	})
}

func (a api) restartRequired(c config.Config) bool {
	if nk, ok := c.Keys.Normalized(), a.d.KeysConfig.Normalized(); (nk.Enabled || ok.Enabled) && !reflect.DeepEqual(nk, ok) {
		return true
	}
	if !a.d.PortFlag && c.Port != a.d.Port {
		return true
	}
	if a.d.QuaverDirFlag {
		return false
	}
	after := c.QuaverDir
	if after == "" && a.d.Detect != nil {
		after = a.d.Detect()
	}
	return !sameFolder(after, a.d.Effective())
}

func sameFolder(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func (a api) overridden() []string {
	out := []string{}
	if a.d.PortFlag {
		out = append(out, "port")
	}
	if a.d.QuaverDirFlag {
		out = append(out, "quaver_dir")
	}
	return out
}

func (a api) getConfig(w http.ResponseWriter, r *http.Request) {
	c, err := config.Load(a.d.ConfigPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not read the config file", "")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"config":           c,
		"effective":        map[string]any{"quaver_dir": a.d.Effective(), "port": a.d.Port},
		"restart_required": a.restartRequired(c),
		"overridden":       a.overridden(),
		"themes_dir":       a.d.ThemesDir,
	})
}

func (a api) putConfig(w http.ResponseWriter, r *http.Request) {
	var c config.Config
	dec := json.NewDecoder(r.Body)
	err := dec.Decode(&c)
	if err == nil {
		if _, e := dec.Token(); e != io.EOF {
			err = errors.New("unexpected data after the JSON object")
			if e != nil {
				err = e
			}
		}
	}
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeError(w, http.StatusRequestEntityTooLarge, "request body too large", "")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error(), "")
		return
	}
	c = config.Normalize(c)
	ids := themes.IDs(a.themeList())
	// Keep accepting the saved theme while its file is missing, so other settings still save.
	if saved, e := config.Load(a.d.ConfigPath); e == nil && saved.Overlay.Theme == c.Overlay.Theme && themes.UserID.MatchString(c.Overlay.Theme) {
		ids = append(ids, c.Overlay.Theme)
	}
	if err := c.Validate(ids); err != nil {
		var fe *config.FieldError
		if errors.As(err, &fe) {
			writeError(w, http.StatusBadRequest, fe.Msg, fe.Field)
			return
		}
		writeError(w, http.StatusBadRequest, err.Error(), "")
		return
	}
	if err := config.Save(a.d.ConfigPath, c); err != nil {
		writeError(w, http.StatusInternalServerError, "could not save the config file", "")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"config": c, "restart_required": a.restartRequired(c), "overridden": a.overridden()})
}

type historyItem struct {
	ID             int64            `json:"id"`
	PlayedAt       string           `json:"played_at"`
	Title          string           `json:"title"`
	Artist         string           `json:"artist"`
	DifficultyName string           `json:"difficulty_name"`
	Difficulty     float64          `json:"difficulty"`
	Keys           int              `json:"keys"`
	Rate           float64          `json:"rate"`
	Mods           []string         `json:"mods"`
	Grade          string           `json:"grade"`
	Accuracy       float64          `json:"accuracy"`
	MaxCombo       int              `json:"max_combo"`
	Judgments      scores.Judgments `json:"judgments"`
	Failed         bool             `json:"failed"`
	Paused         bool             `json:"paused"`
	IsPB           bool             `json:"is_pb"`
	FullCombo      bool             `json:"full_combo"`
	Autoplay       bool             `json:"autoplay"`
}

func (a api) history(w http.ResponseWriter, r *http.Request) {
	limit := defaultHistory
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > maxHistory {
			writeError(w, http.StatusBadRequest, "limit must be between 1 and 200", "limit")
			return
		}
		limit = n
	}
	if a.d.History == nil {
		writeError(w, http.StatusInternalServerError, "play history is unavailable", "")
		return
	}
	results, err := a.d.History(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not read the play history", "")
		return
	}
	items := make([]historyItem, 0, len(results))
	for _, res := range results {
		mods := res.Score.Mods
		if mods == nil {
			mods = []string{}
		}
		items = append(items, historyItem{
			ID: res.Score.ID, PlayedAt: res.Score.PlayedAt,
			Title: res.Map.Title, Artist: res.Map.Artist,
			DifficultyName: res.Map.DifficultyName, Difficulty: res.Map.Difficulty, Keys: res.Map.Keys,
			Rate: res.Score.Rate, Mods: mods, Grade: res.Score.Grade, Accuracy: res.Score.Accuracy,
			MaxCombo: res.Score.MaxCombo, Judgments: res.Score.Judgments,
			Failed: res.Failed, Paused: res.Paused, IsPB: res.IsPB, FullCombo: res.FullCombo, Autoplay: res.Autoplay,
		})
	}
	writeJSON(w, http.StatusOK, items)
}

func (a api) themes(w http.ResponseWriter, r *http.Request) {
	list := a.themeList()
	if list == nil {
		list = []themes.Theme{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (a api) themeList() []themes.Theme {
	if a.d.Themes == nil {
		return nil
	}
	return a.d.Themes()
}

func (a api) overlays(w http.ResponseWriter, r *http.Request) {
	builtin := a.d.IsBuiltinOverlay
	if builtin == nil {
		builtin = func(string) bool { return false }
	}
	writeJSON(w, http.StatusOK, map[string]any{"dir": a.d.OverlaysDir, "custom": overlays.List(a.d.OverlaysDir, builtin)})
}

func (a api) keysStatus() keys.Status {
	if a.d.Keys == nil {
		return keys.Status{Layout: "auto"}
	}
	return a.d.Keys()
}
