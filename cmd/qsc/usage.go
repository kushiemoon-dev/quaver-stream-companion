package main

import (
	"fmt"
	"io"
)

func usage(w io.Writer) {
	fmt.Fprint(w, `qsc: QuaverStreamCompanion, Quaver stream overlays served from your computer

Usage:
  qsc [command] [flags]

Commands:
  serve     start the server and the dashboard (the default when no command is given)
  doctor    check the install; with --report, print a support report to share in a bug report
  stop      ask the running qsc to quit
  version   print the version (also: qsc --version)
  help      show this text (also: qsc -h, qsc --help)

Flags (serve, doctor and stop):
  --port N          local port to listen on (default 7878)
  --config PATH     path to the config file (default: per-user config folder)
  --quaver-dir DIR  path to the Quaver folder (auto-detected when empty; serve and doctor)

Flags (serve only):
  --open            open the dashboard in the default browser
  -v, --verbose     write debug lines to the log

Flags (doctor only):
  --report          print the redacted support report

The log file qsc.log lives next to the config file.
`)
}
