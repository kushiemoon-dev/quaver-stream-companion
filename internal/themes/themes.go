// Package themes lists the overlay themes and reads their main colors.
package themes

import (
	"io/fs"
	"path"
	"regexp"
	"slices"
	"strings"
)

// Theme sources.
const (
	SourceBuiltin = "builtin"
	SourceCustom  = "custom"
)

// Colors are the three swatches of a theme, "#rgb" or "#rrggbb", or "" when unknown.
type Colors struct {
	BG     string `json:"bg"`
	FG     string `json:"fg"`
	Accent string `json:"accent"`
}

// Theme is one selectable overlay theme.
type Theme struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Colors Colors `json:"colors"`
}

var builtinOrder = []string{"neutral-light", "neutral-dark", "catppuccin-mocha", "catppuccin-latte", "tokyo-night", "dracula", "nord", "gruvbox", "rose-pine", "pixel", "neon-cosy"}

var (
	rootBlock = regexp.MustCompile(`:root\s*\{([^}]*)\}`)
	property  = regexp.MustCompile(`(?:^|[;{\s])--(bg|fg|accent)\s*:\s*([^;]*)`)
	hexColor  = regexp.MustCompile(`^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)
	comment   = regexp.MustCompile(`(?s)/\*.*?\*/`)
	themeID   = regexp.MustCompile(`^[a-z0-9-]+$`)
)

func validHex(s string) bool { return hexColor.MatchString(s) }

// ParseColors reads --bg, --fg and --accent from the first :root block of css; anything but a hex color gives "".
func ParseColors(css string) Colors {
	var c Colors
	block := rootBlock.FindStringSubmatch(comment.ReplaceAllString(css, " "))
	if block == nil {
		return c
	}
	for _, m := range property.FindAllStringSubmatch(block[1], -1) {
		v := strings.TrimSpace(m[2])
		if !validHex(v) {
			v = ""
		}
		switch m[1] {
		case "bg":
			c.BG = v
		case "fg":
			c.FG = v
		case "accent":
			c.Accent = v
		}
	}
	return c
}

// Builtin lists the themes/*.css files of fsys whose base name matches [a-z0-9-]+: the known themes in their display order, then any other alphabetically.
func Builtin(fsys fs.FS) []Theme {
	files, _ := fs.Glob(fsys, "themes/*.css")
	ids := make([]string, 0, len(files))
	for _, f := range files {
		if id := strings.TrimSuffix(path.Base(f), ".css"); themeID.MatchString(id) {
			ids = append(ids, id)
		}
	}
	rank := func(id string) int {
		if i := slices.Index(builtinOrder, id); i >= 0 {
			return i
		}
		return len(builtinOrder)
	}
	slices.SortFunc(ids, func(a, b string) int {
		if d := rank(a) - rank(b); d != 0 {
			return d
		}
		return strings.Compare(a, b)
	})
	out := make([]Theme, 0, len(ids))
	for _, id := range ids {
		var colors Colors
		if data, err := fs.ReadFile(fsys, "themes/"+id+".css"); err == nil {
			colors = ParseColors(string(data))
		}
		out = append(out, Theme{ID: id, Source: SourceBuiltin, Colors: colors})
	}
	return out
}

// IDs returns the theme ids in order.
func IDs(list []Theme) []string {
	ids := make([]string, len(list))
	for i, t := range list {
		ids[i] = t.ID
	}
	return ids
}
