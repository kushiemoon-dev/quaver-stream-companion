package diagnose

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/config"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/logging"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/paths"
)

const (
	reportLogLines = 50
	reportLineMax  = 500
	reportListMax  = 100
)

// Source tells Collect what to put in a support report.
type Source struct {
	Version, QuaverDir, ConfigPath string
	ThemesDir, OverlaysDir         string
	SteamRoots                     []string
	Port                           int
	PortOwnedByUs, KeysEnabled     bool
	// Home is the folder replaced by "~" in the report; it defaults to the user home folder.
	Home string
}

// Collect gathers the checks, the settings, the folder listings and the end of the log files into a redacted text report.
func Collect(ctx context.Context, src Source) string {
	home := src.Home
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	ci := caseInsensitiveHome(home)
	names := userNames(home)
	redact := func(s string) string {
		return cutRunes(RedactHome(s, home, ci, names...), reportLineMax)
	}
	var b strings.Builder
	line := func(format string, a ...any) { fmt.Fprintf(&b, format+"\n", a...) }

	line("QuaverStreamCompanion support report")
	line("Review this report before you share it: it lists folder and file names from your computer.")
	line("The Quaver runtime.log excerpt can contain your Quaver player name, map titles and replay paths: remove those lines before you share it.")
	line("")
	line("Version: %s", src.Version)
	line("OS: %s/%s", runtime.GOOS, runtime.GOARCH)
	line("Go: %s", runtime.Version())
	line("Config file: %s", redact(src.ConfigPath))
	line("")
	line("Settings (key bindings left out):")
	line("%s", settingsText(src.ConfigPath, redact))
	line("")
	line("Checks:")
	for _, c := range Run(ctx, Options{QuaverDir: src.QuaverDir, SteamRoots: src.SteamRoots, Port: src.Port, PortOwnedByUs: src.PortOwnedByUs, KeysEnabled: src.KeysEnabled}) {
		tag := "[ OK ]"
		if !c.OK {
			tag = "[FAIL]"
		}
		line("%s %s: %s", tag, c.ID, redact(c.Detail))
	}
	line("")
	line("Custom themes: %s", listNames(src.ThemesDir))
	line("Custom overlays: %s", listNames(src.OverlaysDir))
	line("")
	section(&b, "qsc.log", logging.Tail(filepath.Join(filepath.Dir(src.ConfigPath), "qsc.log"), reportLogLines), redact)
	runtimeLog := ""
	if dir, err := paths.Find(src.QuaverDir, src.SteamRoots); err == nil {
		runtimeLog = paths.LogPath(dir)
	}
	section(&b, "Quaver Logs/runtime.log", logging.Tail(runtimeLog, reportLogLines), redact)
	return RedactHome(b.String(), home, ci, names...)
}

func section(b *strings.Builder, title string, lines []string, redact func(string) string) {
	fmt.Fprintf(b, "--- %s (last %d lines) ---\n", title, reportLogLines)
	if len(lines) == 0 {
		fmt.Fprintf(b, "(no %s)\n", filepath.Base(title))
		return
	}
	for _, l := range lines {
		b.WriteString(redact(l) + "\n")
	}
}

func settingsText(path string, redact func(string) string) string {
	cfg, err := config.Load(path)
	if err != nil {
		return "config problem: " + redact(err.Error())
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return "config problem: " + err.Error()
	}
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil {
		return "config problem: unreadable"
	}
	if dir, ok := m["quaver_dir"].(string); ok {
		m["quaver_dir"] = redact(dir)
	}
	if k, ok := m["keys"].(map[string]any); ok {
		delete(k, "k4")
		delete(k, "k7")
	}
	out, _ := json.MarshalIndent(m, "", "  ")
	return string(out)
}

func listNames(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) == 0 {
		return "(none)"
	}
	list := make([]string, 0, len(entries))
	for _, e := range entries {
		list = append(list, e.Name())
	}
	sort.Strings(list)
	if len(list) > reportListMax {
		list = append(list[:reportListMax], "...")
	}
	return strings.Join(list, ", ")
}
