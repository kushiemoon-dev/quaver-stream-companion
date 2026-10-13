package app

import (
	"context"
	"time"
)

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
