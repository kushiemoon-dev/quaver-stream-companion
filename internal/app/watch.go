package app

import (
	"context"
	"log/slog"
	"time"
)

// Supervise calls problem every interval and, when it names one, reopen. A failed reopen is
// tried again at the next tick. It returns when ctx is done.
func Supervise(ctx context.Context, interval time.Duration, problem func() string, reopen func() error) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		reason := problem()
		if reason == "" {
			continue
		}
		if err := reopen(); err != nil {
			slog.Warn("cannot reopen quaver.db", "reason", reason, "err", err)
			continue
		}
		slog.Info("quaver.db reopened", "reason", reason)
	}
}

// Watch waits for the Quaver folder to become usable. It calls find now and then every
// interval until it returns a folder and start succeeds, then returns without checking again.
func Watch(ctx context.Context, interval time.Duration, find func() (string, error), start func(dir string) error) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if dir, err := find(); err == nil && start(dir) == nil {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
