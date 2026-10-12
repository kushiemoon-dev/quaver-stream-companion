package web

import (
	"io/fs"
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

var builtinThemes = []string{
	"neutral-dark", "catppuccin-mocha", "catppuccin-latte", "tokyo-night",
	"dracula", "nord", "gruvbox", "rose-pine", "pixel", "neon-cosy",
}

var requiredTokens = []string{"bg", "fg", "muted", "accent", "border", "radius", "shadow", "font"}

func themeToken(css, name string) string {
	m := regexp.MustCompile(`--` + name + `\s*:\s*([^;]+);`).FindStringSubmatch(css)
	if m == nil {
		return ""
	}
	return strings.TrimSpace(m[1])
}

func parseHex(s string) (r, g, b float64, ok bool) {
	if !regexp.MustCompile(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`).MatchString(s) {
		return 0, 0, 0, false
	}
	h := s[1:]
	if len(h) == 3 {
		h = string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
	}
	v, err := strconv.ParseUint(h, 16, 32)
	if err != nil {
		return 0, 0, 0, false
	}
	return float64(v >> 16 & 255), float64(v >> 8 & 255), float64(v & 255), true
}

func luminance(hex string) (float64, bool) {
	r, g, b, ok := parseHex(hex)
	if !ok {
		return 0, false
	}
	lin := func(c float64) float64 {
		c /= 255
		if c <= 0.03928 {
			return c / 12.92
		}
		return math.Pow((c+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(r) + 0.7152*lin(g) + 0.0722*lin(b), true
}

func contrast(fg, bg string) (float64, bool) {
	a, ok1 := luminance(fg)
	b, ok2 := luminance(bg)
	if !ok1 || !ok2 {
		return 0, false
	}
	if a < b {
		a, b = b, a
	}
	return (a + 0.05) / (b + 0.05), true
}

func readTheme(t *testing.T, name string) string {
	t.Helper()
	data, err := fs.ReadFile(FS, "themes/"+name+".css")
	if err != nil {
		t.Fatalf("theme %s: %v", name, err)
	}
	return string(data)
}

func TestBuiltinThemesDefineAllTokens(t *testing.T) {
	for _, name := range builtinThemes {
		css := readTheme(t, name)
		for _, tok := range requiredTokens {
			if themeToken(css, tok) == "" {
				t.Errorf("%s: missing --%s", name, tok)
			}
		}
	}
}

func TestBuiltinThemesContrast(t *testing.T) {
	for _, name := range append([]string{"neutral-light"}, builtinThemes...) {
		css := readTheme(t, name)
		bg := themeToken(css, "bg")
		for _, c := range []struct {
			tok string
			min float64
		}{{"fg", 4.5}, {"muted", 3.0}, {"accent", 3.0}} {
			ratio, ok := contrast(themeToken(css, c.tok), bg)
			if !ok {
				t.Errorf("%s: --%s or --bg is not a 3 or 6 digit hex color", name, c.tok)
				continue
			}
			if ratio < c.min {
				t.Errorf("%s: --%s on --bg has contrast %.2f, want at least %.1f", name, c.tok, ratio, c.min)
			}
		}
	}
}

func TestBuiltinThemesUseNoExternalResources(t *testing.T) {
	urlRe := regexp.MustCompile(`url\(\s*["']?([^"')]*)`)
	for _, name := range builtinThemes {
		css := readTheme(t, name)
		if strings.Contains(css, "http") || strings.Contains(css, "@import") {
			t.Errorf("%s: external resource", name)
		}
		for _, m := range urlRe.FindAllStringSubmatch(css, -1) {
			if !strings.HasPrefix(m[1], "fonts/") {
				t.Errorf("%s: url(%s) must point inside fonts/", name, m[1])
			}
		}
	}
}

func TestPixelThemeEmbedsItsFont(t *testing.T) {
	css := readTheme(t, "pixel")
	for _, p := range []string{"themes/fonts/VT323-Regular.ttf", "themes/fonts/OFL.txt"} {
		if _, err := fs.Stat(FS, p); err != nil {
			t.Errorf("%s: %v", p, err)
		}
	}
	if !strings.Contains(css, `url("fonts/VT323-Regular.ttf")`) || !strings.Contains(css, "@font-face") {
		t.Error("pixel.css must load fonts/VT323-Regular.ttf through a relative @font-face")
	}
	if themeToken(css, "scale") != "1.3" {
		t.Error("pixel.css must set --scale: 1.3")
	}
}

func TestOnlyCSSFilesAreThemes(t *testing.T) {
	files, err := fs.Glob(FS, "themes/*.css")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1+len(builtinThemes) {
		t.Errorf("got %d themes %v, want %d", len(files), files, 1+len(builtinThemes))
	}
}

func TestOverlayCSSHonoursScale(t *testing.T) {
	data, err := fs.ReadFile(FS, "overlays/overlay.css")
	if err != nil {
		t.Fatal(err)
	}
	css := string(data)
	for _, want := range []string{"calc(20px * var(--scale, 1))", "calc(14px * var(--scale, 1))"} {
		if !strings.Contains(css, want) {
			t.Errorf("overlay.css is missing %q", want)
		}
	}
}
