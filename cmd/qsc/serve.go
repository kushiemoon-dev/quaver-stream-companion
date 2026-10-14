package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/api"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/app"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/bus"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/config"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/diagnose"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/gamestate"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/keys"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/logging"
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

// steamRoots lists the Steam folders to search; tests replace it.
var steamRoots = paths.DefaultSteamRoots

const logPollInterval = 200 * time.Millisecond

// seedMaxAge is how recent runtime.log must be to seed the game state at startup.
const seedMaxAge = 15 * time.Minute

func serve(ctx context.Context, args []string, out, errw io.Writer) (code int) {
	opts, code := parseFlags("qsc", true, args, errw)
	if code >= 0 {
		return code
	}
	out, errw = &syncWriter{w: out}, &syncWriter{w: errw}

	logPath := ""
	if opts.configPath != "" {
		_ = os.MkdirAll(filepath.Dir(opts.configPath), 0o755)
		logPath = filepath.Join(filepath.Dir(opts.configPath), "qsc.log")
	}
	logger, logFile := logging.Setup(logPath, opts.verbose, errw, diagnose.HomeRedactor(""))
	defer logFile.Close()
	previous := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(previous)
	var crashed atomic.Bool
	defer func() {
		if r := recover(); r != nil {
			logCrash(r, errw)
			code = 2
		}
	}()

	addr := fmt.Sprintf("127.0.0.1:%d", opts.port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		slog.Error("cannot listen", logging.FileOnly, "addr", addr, "err", err.Error())
		fmt.Fprintln(errw, "error: "+bindMessage(opts.port, err))
		return 1
	}
	slog.Info("qsc started", "version", version.Version, "port", opts.port)
	defer slog.Info("qsc stopped")

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	be := &backend{}
	var wg sync.WaitGroup
	defer be.close()
	defer wg.Wait()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	spawn := func(f func()) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			guard(f, func(r any) {
				logCrash(r, errw)
				crashed.Store(true)
				cancel()
			})
		}()
	}

	b := bus.New()
	st := state.New()
	events, unsub := b.Subscribe() // subscribed before the seed line is published
	spawn(func() {
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
	})

	laneKeys := keys.NewService(b, opts.keys, newSampler)
	spawn(func() { laneKeys.Run(ctx) })

	override := func() string {
		if opts.quaverDirFlag {
			return opts.quaverDir
		}
		if c, err := config.Load(opts.configPath); err == nil {
			return c.QuaverDir
		}
		return opts.quaverDir
	}
	reported := ""
	find := func() (string, error) {
		dir, checks := diagnose.Folder(ctx, override(), steamRoots())
		for _, c := range checks {
			if !c.OK {
				if c.Detail != reported {
					reported = c.Detail
					slog.Warn("Quaver folder is not usable", "detail", c.Detail)
					reportDegraded(errw, c.Detail)
				}
				return "", errors.New(c.Detail)
			}
		}
		return dir, nil
	}
	start := func(dir string) error {
		store, err := scores.Open(dir)
		if err != nil {
			slog.Warn("cannot open the score database", "err", err)
			return err
		}
		be.set(dir, store)
		a := app.New(b, be)
		be.mu.Lock()
		be.onReplaced = a.ForgetScores
		be.onTick = func() { a.CheckRestore(ctx) }
		be.mu.Unlock()
		lines := make(chan string, 64)
		logPath := paths.LogPath(dir)
		game := ""
		if seed, ok := gamestate.LastStateLine(logPath, seedMaxAge); ok {
			a.Seed(ctx, seed)
			if ln, ok := gamestate.ParseLine(seed); ok && ln.Kind == gamestate.KindState {
				game = ln.State
			}
		}
		st.Seed(a.SeedSnapshot(ctx, game))
		raw := make(chan string, 64)
		spawn(func() { gamestate.Tail(ctx, logPath, logPollInterval, raw) })
		spawn(func() { relay(ctx, raw, lines, laneKeys.Touch) })
		spawn(func() { a.Run(ctx, lines) })
		slog.Info("Quaver folder found", "dir", dir)
		fmt.Fprintf(out, "Quaver folder: %s\n", dir)
		return nil
	}
	// A usable folder is picked up before the first request; otherwise keep looking.
	started := false
	if dir, err := find(); err == nil && start(dir) == nil {
		started = true
	}
	spawn(func() {
		if !started {
			app.Watch(ctx, recheckInterval, find, start)
		}
		app.Supervise(ctx, recheckInterval, be.problem, be.reopen)
	})

	dashboard, err := fs.Sub(web.FS, "dashboard")
	if err != nil {
		fmt.Fprintln(errw, "error:", err)
		return 1
	}
	effective := func() string {
		if dir, _ := be.current(); dir != "" {
			return dir
		}
		return override()
	}
	apiHandler := api.New(api.Deps{
		Version:    version.Version,
		Started:    time.Now(),
		Snapshot:   st.Snapshot,
		History:    be.recent,
		Timeline:   be.timeline,
		ConfigPath: opts.configPath,
		Effective:  effective,
		Degraded:   func() bool { dir, _ := be.current(); return dir == "" },
		Detect: func() string {
			d, _ := paths.Find("", steamRoots())
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
		SteamRoots:       steamRoots(),
		Port:             opts.port,
		Diagnostics: func(c context.Context) string {
			return diagnose.Collect(c, diagnose.Source{Version: version.Version, QuaverDir: effective(), ConfigPath: opts.configPath,
				ThemesDir: themes.UserDir(opts.configPath), OverlaysDir: overlays.Dir(opts.configPath), SteamRoots: steamRoots(),
				Port: opts.port, PortOwnedByUs: true, KeysEnabled: laneKeys.Status().Enabled})
		},
		Quit: cancel,
	})
	srv := &http.Server{
		Handler: server.New(b, st.Snapshot, web.FS,
			server.WithAPI(apiHandler, opts.port), server.WithDashboard(dashboard), server.WithThemesDir(themes.UserDir(opts.configPath)), server.WithOverlaysDir(overlays.Dir(opts.configPath))),
		ReadHeaderTimeout: 5 * time.Second,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}
	go func() {
		<-ctx.Done()
		srv.Close()
	}()

	fmt.Fprintf(out, "Now playing overlay (OBS browser source): http://%s/overlays/now-playing/\n", addr)
	fmt.Fprintf(out, "Dashboard: http://%s/\n", addr)
	fmt.Fprintf(out, "Live state: http://%s/state\n", addr)
	fmt.Fprintln(out, "Press Ctrl+C to stop.")
	fmt.Fprintln(out, "Next: open the dashboard, OBS page, to add the overlays.")
	fmt.Fprintln(out, "Keep this window open while you stream, or run qsc stop to quit.")

	slog.Debug("verbose logging is on")
	afterStart()

	if opts.open {
		if err := openBrowser(fmt.Sprintf("http://%s/", addr)); err != nil {
			fmt.Fprintln(errw, "warning: cannot open the browser:", err)
		}
	}

	if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintln(errw, "error:", err)
		return 1
	}
	if crashed.Load() {
		return 2
	}
	return 0
}

func reportDegraded(w io.Writer, detail string) {
	fmt.Fprintf(w, "warning: Quaver is not ready yet (%s)\n", detail)
	for _, l := range diagnose.Advice() {
		fmt.Fprintln(w, " ", l)
	}
	fmt.Fprintf(w, "The dashboard works meanwhile. qsc checks again every %s.\n", recheckInterval)
}
