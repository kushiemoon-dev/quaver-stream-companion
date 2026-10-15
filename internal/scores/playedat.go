package scores

import (
	"log/slog"
	"strings"
	"sync"
	"time"
)

const maxPlayedAtLen = 64

var playedAtLayouts = []string{
	"1/2/2006 15:04",
	"1/2/2006 15:04:05",
	"1/2/2006 3:04 PM",
	"1/2/2006 3:04:05 PM",
	"2006-01-02 15:04",
	"2006-01-02 15:04:05",
	"2006-01-02T15:04",
	"2006-01-02T15:04:05",
}

var playedAtDebug sync.Once

// parsePlayedAt turns the local time text of a score row into RFC 3339 using loc. Text it cannot
// read is returned unchanged.
func parsePlayedAt(raw string, loc *time.Location) string {
	// Quaver writes dates with the invariant culture, so the month comes first.
	text := strings.TrimSpace(raw)
	if len(raw) <= maxPlayedAtLen && text != "" {
		for _, layout := range playedAtLayouts {
			if t, err := time.ParseInLocation(layout, text, loc); err == nil {
				return t.Format(time.RFC3339)
			}
		}
	}
	if raw != "" {
		playedAtDebug.Do(func() { slog.Debug("score date in an unknown layout; keeping the raw text") })
	}
	return raw
}
