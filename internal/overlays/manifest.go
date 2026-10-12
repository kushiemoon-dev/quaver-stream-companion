package overlays

import (
	"bytes"
	"encoding/json"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
)

const (
	maxManifest = 8 << 10
	maxName     = 40
	maxDesc     = 160
	manifest    = "overlay.json"

	defaultWidth, defaultHeight = 500, 200
	minWidth, maxWidth          = 100, 3840
	minHeight, maxHeight        = 40, 2160
)

// Info describes one custom overlay for the dashboard.
type Info struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
}

type rawManifest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Width       *float64 `json:"width"`
	Height      *float64 `json:"height"`
}

// List returns the custom overlays of dir sorted by id, at most MaxListed. Folders with a built-in id,
// without a readable index.html or with too many files are skipped. A missing or invalid manifest gives defaults.
func List(dir string, builtin func(id string) bool) []Info {
	out := []Info{}
	if dir == "" {
		return out
	}
	f, err := os.Open(dir)
	if err != nil {
		return out
	}
	entries, _ := f.ReadDir(maxScanEntries)
	f.Close()
	var ids []string
	for _, e := range entries {
		if e.Type() == fs.ModeDir && ID.MatchString(e.Name()) && !builtin(e.Name()) {
			ids = append(ids, e.Name())
		}
	}
	slices.Sort(ids)
	for _, id := range ids {
		if len(out) == MaxListed {
			break
		}
		root := filepath.Join(dir, id)
		if tooMany(root) {
			continue
		}
		if _, err := readIn(root, indexFile, MaxFile); err != nil {
			continue
		}
		out = append(out, describe(root, id))
	}
	return out
}

func describe(root, id string) Info {
	info := Info{ID: id, Name: id, Width: defaultWidth, Height: defaultHeight}
	f, err := readIn(root, manifest, maxManifest)
	if err != nil {
		return info
	}
	dec := json.NewDecoder(bytes.NewReader(f.Data))
	var m rawManifest
	if dec.Decode(&m) != nil {
		return info
	}
	if _, err := dec.Token(); err != io.EOF {
		return info
	}
	if name := clean(m.Name, maxName); name != "" {
		info.Name = name
	}
	info.Description = clean(m.Description, maxDesc)
	if m.Width != nil {
		info.Width = clamp(*m.Width, minWidth, maxWidth)
	}
	if m.Height != nil {
		info.Height = clamp(*m.Height, minHeight, maxHeight)
	}
	return info
}

func clamp(v float64, lo, hi int) int {
	return int(math.Min(math.Max(math.Round(v), float64(lo)), float64(hi)))
}

// clean drops control characters, trims and caps the text to max runes.
func clean(s string, max int) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == unicode.ReplacementChar {
			return -1
		}
		return r
	}, s)
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > max {
		s = strings.TrimSpace(string(r[:max]))
	}
	return s
}
