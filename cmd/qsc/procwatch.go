package main

import (
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/config"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/procwatch"
)

// processInterval is how often qsc looks for a Quaver process; tests replace it.
var processInterval = newHook(10 * time.Second)

// processLister gives the lister of running program names, false when unsupported; tests replace it.
var processLister = newHook(procwatch.NewLister)

// processTick is told, at each check, whether the setting was on; tests replace it to follow the checks.
var processTick = newHook(func(bool) {})

// watchEnabled reads the setting at each call, so a change applies without a restart. A config
// file that cannot be read turns the check off, with one debug line.
func watchEnabled(configPath string) func() bool {
	var warned atomic.Bool
	return func() bool {
		c, err := config.Load(configPath)
		if err != nil {
			if warned.CompareAndSwap(false, true) {
				slog.Debug("config file unreadable; the Quaver process check is off")
			}
			return false
		}
		warned.Store(false)
		return c.WatchQuaverProcess
	}
}

// observedEnabled is watchEnabled that also reports each answer to the processTick hook.
func observedEnabled(configPath string) func() bool {
	inner := watchEnabled(configPath)
	return func() bool {
		on := inner()
		processTick.get()(on)
		return on
	}
}
