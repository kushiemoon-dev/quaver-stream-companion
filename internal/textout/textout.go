// Package textout keeps small text files about the current map and the last score, for OBS text sources.
package textout

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/bus"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/scores"
)

// maxBytes caps the text of one file.
const maxBytes = 256

// Names lists the files, which are also the names accepted by GET /api/text/{name}.
var Names = []string{"title.txt", "artist.txt", "difficulty.txt", "mapper.txt", "nowplaying.txt", "last_grade.txt", "last_accuracy.txt"}

// Dir returns the folder of the files next to the config file, or "" when there is no config path.
func Dir(configPath string) string {
	if configPath == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(configPath), "text")
}

// clean drops control, line separator and format characters (a byte order mark included) and cuts the text to maxBytes on a character boundary.
func clean(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.In(r, unicode.Cf, unicode.Zl, unicode.Zp) || r == utf8.RuneError {
			return -1
		}
		return r
	}, s)
	if len(s) > maxBytes {
		cut := maxBytes
		for !utf8.RuneStart(s[cut]) {
			cut--
		}
		s = s[:cut]
	}
	return s
}

// Texts returns the content of every file. Without a map every text is empty; without a score only the score texts are.
func Texts(m *scores.Map, r *scores.Result) map[string]string {
	out := make(map[string]string, len(Names))
	for _, n := range Names {
		out[n] = ""
	}
	if m == nil {
		return out
	}
	now := clean(m.Title)
	if a := clean(m.Artist); a != "" {
		now = a + " - " + now
	}
	if d := clean(m.DifficultyName); d != "" {
		now += " [" + d + "]"
	}
	out["title.txt"], out["artist.txt"], out["difficulty.txt"] = clean(m.Title), clean(m.Artist), clean(m.DifficultyName)
	out["mapper.txt"], out["nowplaying.txt"] = clean(m.Creator), clean(now)
	if r != nil {
		out["last_grade.txt"] = clean(r.Score.Grade)
		out["last_accuracy.txt"] = fmt.Sprintf("%.2f%%", r.Score.Accuracy)
	}
	return out
}

// Writer rewrites the files when the map or the last score changes.
type Writer struct {
	dir     string
	enabled func() bool

	mu      sync.Mutex
	m       *scores.Map
	r       *scores.Result
	written map[string]string
}

// NewWriter returns a writer for dir. enabled is asked at every event, so the setting applies without a restart.
func NewWriter(dir string, enabled func() bool) *Writer {
	return &Writer{dir: dir, enabled: enabled}
}

// Seed fills the map and score still unknown, as the state store does at startup.
func (w *Writer) Seed(m *scores.Map, r *scores.Result) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.m == nil {
		w.m = m
	}
	if w.r == nil {
		w.r = r
	}
	w.flush()
}

// Refresh writes the files for the current knowledge.
func (w *Writer) Refresh() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.flush()
}

// Handle folds a map.started or score.saved event in and updates the files. Other events are ignored.
func (w *Writer) Handle(e bus.Event) {
	w.mu.Lock()
	defer w.mu.Unlock()
	switch d := e.Data.(type) {
	case scores.Map:
		w.m = &d
	case scores.Result:
		w.r = &d
	default:
		return
	}
	w.flush()
}

// Run writes the files once, then follows ch until ctx ends.
func (w *Writer) Run(ctx context.Context, ch <-chan bus.Event) {
	w.Refresh()
	for {
		select {
		case <-ctx.Done():
			return
		case e, ok := <-ch:
			if !ok {
				return
			}
			w.Handle(e)
		}
	}
}

// flush needs w.mu. While the setting is off nothing is written and the memory of what was written is dropped, so the next write brings every file up to date.
func (w *Writer) flush() {
	if w.dir == "" || !w.enabled() {
		w.written = nil
		return
	}
	next := make(map[string]string, len(Names))
	for name, text := range Texts(w.m, w.r) {
		if old, ok := w.written[name]; ok && old == text {
			next[name] = text
			continue
		}
		if err := writeAtomic(w.dir, name, text); err != nil {
			slog.Warn("cannot write a text file", "name", name, "err", err)
			continue
		}
		next[name] = text
	}
	w.written = next
}

// writeAtomic replaces dir/name through a temporary file of the same folder. The rename is retried briefly because OBS or an antivirus may hold the file open on Windows.
func writeAtomic(dir, name, text string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+name+".*.tmp")
	if err != nil {
		return err
	}
	_, werr := tmp.WriteString(text)
	cerr := tmp.Close()
	if werr != nil || cerr != nil {
		os.Remove(tmp.Name())
		if werr != nil {
			return werr
		}
		return cerr
	}
	err = os.Rename(tmp.Name(), filepath.Join(dir, name))
	for i := 0; err != nil && i < 50; i++ {
		time.Sleep(10 * time.Millisecond)
		err = os.Rename(tmp.Name(), filepath.Join(dir, name))
	}
	if err != nil {
		os.Remove(tmp.Name())
	}
	return err
}
