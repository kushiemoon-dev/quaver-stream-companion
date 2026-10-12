package main

import (
	"context"
	"fmt"
	"io"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/diagnose"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/paths"
)

func doctor(args []string, out, errw io.Writer) int {
	opts, code := parseFlags("qsc doctor", false, args, errw)
	if code >= 0 {
		return code
	}

	names := map[string]string{
		"quaver_dir": "Quaver folder",
		"log":        "Logs/runtime.log",
		"database":   "quaver.db (read-only)",
		"port":       fmt.Sprintf("port %d", opts.port),
	}
	checks := diagnose.Run(context.Background(), diagnose.Options{
		QuaverDir:  opts.quaverDir,
		SteamRoots: paths.DefaultSteamRoots(),
		Port:       opts.port,
	})

	healthy := true
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
	if !healthy {
		return 1
	}
	return 0
}
