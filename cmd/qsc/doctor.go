package main

import (
	"context"
	"fmt"
	"io"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/diagnose"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/overlays"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/themes"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/version"
)

func doctor(args []string, out, errw io.Writer) int {
	opts, problem, code := parseOptions("qsc doctor", false, args, errw)
	if code >= 0 {
		return code
	}

	if opts.report {
		dir := opts.quaverDir
		fmt.Fprint(out, diagnose.Collect(context.Background(), diagnose.Source{Version: version.Version, QuaverDir: dir, ConfigPath: opts.configPath,
			ThemesDir: themes.UserDir(opts.configPath), OverlaysDir: overlays.Dir(opts.configPath), SteamRoots: steamRoots(),
			Port: opts.port, KeysEnabled: opts.keys.Enabled}))
		return 0
	}

	names := map[string]string{
		"quaver_dir": "Quaver folder",
		"log":        "Logs/runtime.log",
		"database":   "quaver.db (read-only)",
		"port":       fmt.Sprintf("port %d", opts.port),
		"keys":       "Lane keys",
	}
	checks := diagnose.Run(context.Background(), diagnose.Options{
		QuaverDir:   opts.quaverDir,
		SteamRoots:  steamRoots(),
		Port:        opts.port,
		KeysEnabled: opts.keys.Enabled,
	})

	healthy := problem == nil
	if problem != nil {
		fmt.Fprintf(out, "[FAIL] Config file: %s\n", problem.headline)
		if problem.fix != "" {
			fmt.Fprintln(out, " ", problem.fix)
		}
	}
	for i, c := range checks {
		// Without a Quaver folder the dependent checks add only noise.
		if i > 0 && i < 3 && !checks[0].OK {
			continue
		}
		tag := "[ OK ]"
		if !c.OK {
			healthy = false
			tag = "[FAIL]"
		}
		fmt.Fprintf(out, "%s %s: %s\n", tag, names[c.ID], c.Detail)
	}
	if !checks[0].OK {
		for _, l := range diagnose.Advice() {
			fmt.Fprintln(out, " ", l)
		}
	}
	if !opts.keys.Enabled {
		fmt.Fprintf(out, "[ OK ] %s: off (opt-in)\n", names["keys"])
	}
	if !healthy {
		return 1
	}
	return 0
}
