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
	"sync"
	"syscall"
	"time"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/api"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/app"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/bus"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/config"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/diagnose"
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

// steamRoots lists the Steam folders to search; tests replace it.
var steamRoots = paths.DefaultSteamRoots

const logPollInterval = 200 * time.Millisecond

// seedMaxAge is how recent runtime.log must be to seed the game state at startup.
const seedMaxAge = 15 * time.Minute

func serve(ctx context.Context, args []string, out, errw io.Writer) int {
	opts, code := parseFlags("qsc", true, args, errw)
	if code >= 0 {
		return code
	}
	out, errw = &syncWriter{w: out}, &syncWriter{w: errw}

	addr := fmt.Sprintf("127.0.0.1:%d", opts.port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Fprintf(errw, "error: cannot listen on %s (port already in use?): %v\n", addr, err)
		return 1
	}

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	be := &backend{}
	var wg sync.WaitGroup
	defer be.close()
	defer wg.Wait()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	b := bus.New()
	st := state.New()
	events, unsub := b.Subscribe() // subscribed before the seed line is published
	wg.Add(1)
	go func() {
		defer wg.Done()
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
	wg.Add(1)
	go func() {
		defer wg.Done()
		laneKeys.Run(ctx)
	}()

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
			return err
		}
		be.set(dir, store)
		a := app.New(b, store)
		lines := make(chan string, 64)
		logPath := paths.LogPath(dir)
		if seed, ok := gamestate.LastStateLine(logPath, seedMaxAge); ok {
			a.Seed(ctx, seed)
		}
		wg.Add(2)
		go func() { defer wg.Done(); gamestate.Tail(ctx, logPath, logPollInterval, lines) }()
		go func() { defer wg.Done(); a.Run(ctx, lines) }()
		fmt.Fprintf(out, "Quaver folder: %s\n", dir)
		return nil
	}
	// A usable folder is picked up before the first request; otherwise keep looking.
	if dir, err := find(); err != nil || start(dir) != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			app.Watch(ctx, recheckInterval, find, start)
		}()
	}

	dashboard, err := fs.Sub(web.FS, "dashboard")
	if err != nil {
		fmt.Fprintln(errw, "error:", err)
		return 1
	}
	apiHandler := api.New(api.Deps{
		Version:    version.Version,
		Started:    time.Now(),
		Snapshot:   st.Snapshot,
		History:    be.recent,
		Timeline:   be.timeline,
		ConfigPath: opts.configPath,
		Effective: func() string {
			if dir, _ := be.current(); dir != "" {
				return dir
			}
			return override()
		},
		Degraded: func() bool { dir, _ := be.current(); return dir == "" },
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

func reportDegraded(w io.Writer, detail string) {
	fmt.Fprintf(w, "warning: Quaver is not ready yet (%s)\n", detail)
	for _, l := range diagnose.Advice() {
		fmt.Fprintln(w, " ", l)
	}
	fmt.Fprintf(w, "The dashboard works meanwhile. qsc checks again every %s.\n", recheckInterval)
}
