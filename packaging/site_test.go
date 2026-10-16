package packaging

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var toolWords = []string{"a" + "i", "cl" + "aude", "anth" + "ropic", "chat" + "gpt", "open" + "ai", "copi" + "lot", "l" + "lm", "assis" + "tant", "super" + "powers"}

const repoURL = "https://github.com/kushiemoon-dev/quaver-stream-companion"

// fontHosts are the only other sites the website may load from: its two fonts come from Google Fonts.
var fontHosts = []string{"https://fonts.googleapis.com", "https://fonts.gstatic.com"}

var overlayNames = []string{"now-playing", "result", "history", "state", "alert", "keys", "curve", "session"}

func TestPagesWorkflow(t *testing.T) {
	wf := read(t, ".github/workflows/pages.yml")
	for _, want := range []string{
		"workflow_dispatch:",
		"- 'site/**'",
		"- 'web/overlays/**'",
		"- 'web/themes/**'",
		"- 'scripts/build-site.mjs'",
		"permissions:\n  contents: read\n  pages: write\n  id-token: write\n",
		"environment:\n      name: github-pages",
		"actions/configure-pages@",
		"run: node scripts/build-site.mjs",
		"actions/upload-pages-artifact@",
		"path: .site-out",
		"actions/deploy-pages@",
	} {
		if !strings.Contains(wf, want) {
			t.Errorf("pages.yml lacks %q", want)
		}
	}
	if strings.Contains(wf, "write-all") || strings.Contains(wf, "contents: write") {
		t.Error("pages.yml asks for more permissions than it needs")
	}
	uses := regexp.MustCompile(`(?m)^\s*- uses:\s*(\S+)(.*)$`).FindAllStringSubmatch(wf, -1)
	if len(uses) != 4 {
		t.Fatalf("pages.yml has %d actions, want 4", len(uses))
	}
	for _, m := range uses {
		if !regexp.MustCompile(`@[0-9a-f]{40}$`).MatchString(m[1]) {
			t.Errorf("%q is not pinned to a full commit SHA", m[1])
		}
		if !regexp.MustCompile(`^\s+# v\d+$`).MatchString(m[2]) {
			t.Errorf("%q needs a trailing major tag comment", m[1])
		}
	}
}

func siteFiles(t *testing.T) map[string]string {
	t.Helper()
	root := filepath.Join("..", "site")
	files := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(p[len(root)+1:])] = string(b)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func allowedURL(u string) bool {
	for _, ok := range append([]string{repoURL, "https://quavergame.com"}, fontHosts...) {
		if strings.HasPrefix(u, ok) {
			return true
		}
	}
	return false
}

func TestSiteLoadsOnlyItsFontsFromOtherSites(t *testing.T) {
	files := siteFiles(t)
	html, css := files["index.html"], files["style.css"]
	if html == "" || css == "" {
		t.Fatal("site/index.html and site/style.css are required")
	}
	dashes := string([]rune{0x2014, 0x2013, 0x2018, 0x2019, 0x201c, 0x201d})
	words := regexp.MustCompile(`(?i)\b(` + strings.Join(toolWords, "|") + `)\b`)
	for name, body := range files {
		if strings.ContainsAny(body, dashes) {
			t.Errorf("%s contains a dash or curly quote", name)
		}
		if m := words.FindString(body); m != "" {
			t.Errorf("%s mentions %q", name, m)
		}
		for _, u := range regexp.MustCompile(`https?://[^\s"'<>)]+`).FindAllString(body, -1) {
			if !allowedURL(u) {
				t.Errorf("%s links to %s", name, u)
			}
		}
	}
	for _, m := range regexp.MustCompile(`<(script|link|iframe|img)\b[^>]*>`).FindAllString(html, -1) {
		src := regexp.MustCompile(`\s(?:src|href)="([^"]*)"`).FindStringSubmatch(m)
		if strings.HasPrefix(m, "<script") && src == nil {
			t.Errorf("index.html has an inline script: %s", m)
		}
		if src != nil && strings.Contains(src[1], "//") && !(strings.HasPrefix(m, "<link") && allowedURL(src[1]) && src[1] != repoURL) {
			t.Errorf("index.html loads a remote resource: %s", m)
		}
	}
	if regexp.MustCompile(`(?i)@import|url\(\s*["']?(https?:)?//`).MatchString(css) {
		t.Error("style.css loads a remote resource")
	}
	fonts := regexp.MustCompile(`<link rel="stylesheet" href="(https://fonts\.googleapis\.com/[^"]+)">`).FindStringSubmatch(html)
	if fonts == nil || !strings.Contains(fonts[1], "display=swap") || strings.Count(fonts[1], "family=") != 2 {
		t.Error("index.html must load two font families from Google Fonts with display=swap")
	}
	for name, body := range files {
		if filepath.Ext(name) != ".js" {
			continue
		}
		for _, bad := range []string{"fetch(", "XMLHttpRequest", "WebSocket", "EventSource", "sendBeacon", "innerHTML", "outerHTML", "insertAdjacentHTML", "document.cookie"} {
			if strings.Contains(body, bad) {
				t.Errorf("%s uses %s", name, bad)
			}
		}
	}
}

func TestSiteContent(t *testing.T) {
	files := siteFiles(t)
	html := files["index.html"]
	for _, want := range []string{
		`<html lang="en">`, `name="viewport"`, `<meta name="color-scheme" content="dark">`, "<title>", `rel="stylesheet" href="style.css"`,
		"qsc --open", repoURL + "/releases/latest", "v0.2.0", "not affiliated with Quaver",
		"quaver.db", "read-only", "127.0.0.1", "Linux", "not yet with a real Quaver install", repoURL + "/blob/main/LICENSE",
		repoURL + "/blob/main/docs/OBS.md", repoURL + "/blob/main/docs/OVERLAYS.md", repoURL + "/blob/main/docs/THEMES.md",
		"loads its two fonts from Google Fonts", "The app itself makes no network request",
		`id="lanes"`, `id="theme"`, `id="lang"`, `id="play"`, `aria-live="polite"`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("index.html lacks %q", want)
		}
	}
	if !strings.Contains(files["style.css"], "color-scheme: dark") {
		t.Error("style.css does not declare its dark scheme")
	}
	if !strings.Contains(files["lanes.js"], "prefers-reduced-motion") || !strings.Contains(files["lanes.js"], "visibilitychange") {
		t.Error("lanes.js must respect reduced motion and pause in a hidden tab")
	}
	if !strings.Contains(files["demo.js"], "'feed', 'parent'") || !strings.Contains(files["demo.js"], "location.origin") {
		t.Error("demo.js must feed the overlays through feed=parent and post to its own origin only")
	}
	frames := regexp.MustCompile(`<iframe [^>]*>`).FindAllString(html, -1)
	seen := map[string]bool{}
	for _, f := range frames {
		for _, attr := range []string{"title", "width", "height"} {
			if !regexp.MustCompile(`\s` + attr + `="[^"]+"`).MatchString(f) {
				t.Errorf("%s lacks %s", f, attr)
			}
		}
		m := regexp.MustCompile(`src="/overlays/([a-z-]+)/\?demo=1`).FindStringSubmatch(f)
		if m == nil {
			t.Errorf("%s must start on the demo data of a built-in overlay", f)
			continue
		}
		if _, err := os.Stat(filepath.Join("..", "web", "overlays", m[1], "index.html")); err != nil {
			t.Errorf("overlay %s does not exist", m[1])
		}
		seen[m[1]] = true
	}
	for _, name := range overlayNames {
		if !seen[name] {
			t.Errorf("the demo lacks the %s overlay", name)
		}
	}
}

func TestSiteIsSmall(t *testing.T) {
	total := 0
	for _, body := range siteFiles(t) {
		total += len(body)
	}
	if total > 100*1024 {
		t.Errorf("site/ weighs %d bytes, limit is 102400", total)
	}
}

func TestBuildSite(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not on PATH")
	}
	out := t.TempDir()
	cmd := exec.Command(node, "scripts/build-site.mjs", out)
	cmd.Dir = ".."
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build-site failed: %v\n%s", err, b)
	}
	for _, want := range []string{"index.html", "demo.js", "CNAME", "overlays/common.js", "overlays/session/index.html", "themes/pixel.css", "themes/fonts/VT323-Regular.ttf"} {
		if _, err := os.Stat(filepath.Join(out, want)); err != nil {
			t.Errorf("the built site lacks %s", want)
		}
	}
}
