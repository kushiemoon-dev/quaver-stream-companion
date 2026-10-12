package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/api"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/app"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/bus"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/gamestate"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/keys"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/overlays"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/paths"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/scores"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/server"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/state"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/themes"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/version"
	"github.com/kushiemoon-dev/quaver-stream-companion/web"
)

// newSampler creates the lane key sampler; tests replace it so they never read the keyboard.
var newSampler = keys.NewSampler

const logPollInterval = 200 * time.Millisecond

// seedMaxAge is how recent runtime.log must be to seed the game state at startup.
const seedMaxAge = 15 * time.Minute

func serve(ctx context.Context, args []string, out, errw io.Writer) int {
	opts, code := parseFlags("qsc", true, args, errw)
	if code >= 0 {
		return code
	}

	dir, err := paths.Find(opts.quaverDir, paths.DefaultSteamRoots())
	if err != nil {
		fmt.Fprintln(errw, "error:", err)
		return 1
	}
	store, err := scores.Open(dir)
	if err != nil {
		fmt.Fprintln(errw, "error:", err)
		return 1
	}
	defer store.Close()

	addr := fmt.Sprintf("127.0.0.1:%d", opts.port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Fprintf(errw, "error: cannot listen on %s (port already in use?): %v\n", addr, err)
		return 1
	}

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	b := bus.New()
	st := state.New()
	events, unsub := b.Subscribe() // subscribed before the seed line is published
	go func() {
		defer unsub()
		for {
			select {
			case <-ctx.Done():
				return
			case e, ok := <-events:
				if !ok {
					return
				}
				st.Apply(e)
			}
		}
	}()

	laneKeys := keys.NewService(b, opts.keys, newSampler)
	go laneKeys.Run(ctx)
	a := app.New(b, store)
	lines := make(chan string, 64)
	logPath := filepath.Join(dir, "Logs", "runtime.log")
	if seed, ok := gamestate.LastStateLine(logPath, seedMaxAge); ok {
		a.Seed(ctx, seed)
	}
	go gamestate.Tail(ctx, logPath, logPollInterval, lines)
	go a.Run(ctx, lines)

	dashboard, err := fs.Sub(web.FS, "dashboard")
	if err != nil {
		fmt.Fprintln(errw, "error:", err)
		return 1
	}
	apiHandler := api.New(api.Deps{
		Version:    version.Version,
		Started:    time.Now(),
		Snapshot:   st.Snapshot,
		History:    store.Recent,
		ConfigPath: opts.configPath,
		Effective:  func() string { return dir },
		Detect: func() string {
			d, _ := paths.Find("", paths.DefaultSteamRoots())
			return d
		},
		PortFlag:         opts.portFlag,
		QuaverDirFlag:    opts.quaverDirFlag,
		Themes:           func() []themes.Theme { return availableThemes(opts.configPath) },
		ThemesDir:        themes.UserDir(opts.configPath),
		OverlaysDir:      overlays.Dir(opts.configPath),
		IsBuiltinOverlay: func(id string) bool { return overlays.IsBuiltin(web.FS, id) },
		Keys:             laneKeys.Status,
		KeysConfig:       opts.keys,
		SteamRoots:       paths.DefaultSteamRoots(),
		Port:             opts.port,
	})
	srv := &http.Server{
		Handler: server.New(b, st.Snapshot, web.FS,
			server.WithAPI(apiHandler, opts.port), server.WithDashboard(dashboard), server.WithThemesDir(themes.UserDir(opts.configPath)), server.WithOverlaysDir(overlays.Dir(opts.configPath))),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		<-ctx.Done()
		srv.Close()
	}()

	fmt.Fprintf(out, "Quaver folder: %s\n", dir)
	fmt.Fprintf(out, "Now playing overlay (OBS browser source): http://%s/overlays/now-playing/\n", addr)
	fmt.Fprintf(out, "Dashboard: http://%s/\n", addr)
	fmt.Fprintf(out, "Live state: http://%s/state\n", addr)
	fmt.Fprintln(out, "Press Ctrl+C to stop.")

	if opts.open {
		if err := openBrowser(fmt.Sprintf("http://%s/", addr)); err != nil {
			fmt.Fprintln(errw, "warning: cannot open the browser:", err)
		}
	}

	if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintln(errw, "error:", err)
		return 1
	}
	return 0
}
