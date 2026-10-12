package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/config"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/themes"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/version"
	"github.com/kushiemoon-dev/quaver-stream-companion/web"
)

type options struct {
	quaverDir  string
	port       int
	configPath string
	open       bool

	portFlag, quaverDirFlag bool
}

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, out, errw io.Writer) int {
	if len(args) > 0 {
		switch args[0] {
		case "serve":
			return serve(ctx, args[1:], out, errw)
		case "doctor":
			return doctor(args[1:], out, errw)
		case "version", "--version":
			fmt.Fprintln(out, version.String())
			return 0
		}
	}
	return serve(ctx, args, out, errw)
}

// parseFlags returns the options and -1, or a zero value and the exit code to use.
// Explicit flags win over the config file, which wins over auto-detection.
func parseFlags(name string, serving bool, args []string, errw io.Writer) (options, int) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(errw)
	var o options
	fs.StringVar(&o.quaverDir, "quaver-dir", "", "path to the Quaver folder (auto-detected when empty)")
	fs.IntVar(&o.port, "port", config.DefaultPort, "local port to listen on")
	fs.StringVar(&o.configPath, "config", "", "path to the config file (default: per-user config folder)")
	if serving {
		fs.BoolVar(&o.open, "open", false, "open the dashboard in the default browser")
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return options{}, 0
		}
		return options{}, 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(errw, "error: unexpected argument %q\n", fs.Arg(0))
		return options{}, 2
	}
	if o.port < 1 || o.port > 65535 {
		fmt.Fprintf(errw, "error: invalid port %d (use 1-65535)\n", o.port)
		return options{}, 2
	}
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	o.quaverDirFlag, o.portFlag = set["quaver-dir"], set["port"]

	if o.configPath == "" {
		p, err := config.DefaultPath()
		if err != nil {
			fmt.Fprintf(errw, "warning: cannot locate the config folder (%v); using the default settings\n", err)
			return o, -1
		}
		o.configPath = p
	}
	cfg, err := config.Load(o.configPath)
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		fmt.Fprintf(errw, "error: cannot read config %s: %v\n", o.configPath, pathErr.Err)
		return options{}, 1
	}
	if err == nil {
		// The folder is checked later by paths.Find, so a stale saved folder cannot hide a good flag or block doctor.
		checked := cfg
		checked.QuaverDir = ""
		ids := themes.IDs(availableThemes(o.configPath))
		// A missing default theme only falls back to neutral-light; its syntax is still checked.
		if t := cfg.Overlay.Theme; themes.UserID.MatchString(t) && !slices.Contains(ids, t) {
			fmt.Fprintf(errw, "warning: default theme %q not found, using neutral-light\n", t)
			ids = append(ids, t)
		}
		if err = checked.Validate(ids); err != nil {
			err = fmt.Errorf("%s: %w", o.configPath, err)
		}
	}
	if err != nil {
		fmt.Fprintf(errw, "error: invalid config %v\n", err)
		return options{}, 1
	}
	if !set["quaver-dir"] {
		o.quaverDir = cfg.QuaverDir
	}
	if !set["port"] {
		o.port = cfg.Port
	}
	return o, -1
}

var builtinThemes = themes.Builtin(web.FS)

// availableThemes lists the overlay themes a config file may select: the built-in ones and the custom ones next to the config file.
func availableThemes(configPath string) []themes.Theme {
	return themes.Merge(builtinThemes, themes.UserDir(configPath))
}
