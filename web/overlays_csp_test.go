package web

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"
)

func TestBuiltInOverlaysNeedNoInlineHandlerOrRemoteResource(t *testing.T) {
	handler := regexp.MustCompile(`(?i)\son[a-z]+\s*=`)
	ref := regexp.MustCompile(`(?i)\b(?:src|href|action)\s*=\s*["']([^"']*)["']`)
	cssURL := regexp.MustCompile(`(?i)url\(\s*["']?([^"')]*)`)
	local := func(u string) bool {
		return (strings.HasPrefix(u, "/") && !strings.HasPrefix(u, "//")) || strings.HasPrefix(u, "data:") || strings.HasPrefix(u, "#") || !strings.Contains(u, ":") && !strings.HasPrefix(u, "//")
	}
	for _, name := range overlayPages {
		src := readWeb(t, "overlays/"+name+"/index.html")
		if handler.MatchString(src) {
			t.Errorf("%s uses an inline event handler attribute", name)
		}
		if strings.Contains(strings.ToLower(src), "javascript:") {
			t.Errorf("%s uses a javascript: URL", name)
		}
		for _, m := range ref.FindAllStringSubmatch(src, -1) {
			if !local(m[1]) {
				t.Errorf("%s loads %q, which the CSP would block", name, m[1])
			}
		}
		for _, m := range cssURL.FindAllStringSubmatch(src, -1) {
			if !local(m[1]) {
				t.Errorf("%s uses css url(%q), which the CSP would block", name, m[1])
			}
		}
	}
	for _, p := range []string{"overlays/overlay.css", "themes/pixel.css", "themes/neon-cosy.css"} {
		if _, err := fs.Stat(FS, p); err != nil {
			continue
		}
		for _, m := range cssURL.FindAllStringSubmatch(readWeb(t, p), -1) {
			if !local(m[1]) {
				t.Errorf("%s uses css url(%q), which the CSP would block", p, m[1])
			}
		}
	}
}
