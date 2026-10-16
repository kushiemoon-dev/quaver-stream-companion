package packaging

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var toolWords = []string{"a" + "i", "cl" + "aude", "anth" + "ropic", "chat" + "gpt", "open" + "ai", "copi" + "lot", "l" + "lm", "assis" + "tant", "super" + "powers"}

const repoURL = "https://github.com/kushiemoon-dev/quaver-stream-companion"

func TestPagesWorkflow(t *testing.T) {
	wf := read(t, ".github/workflows/pages.yml")
	for _, want := range []string{
		"workflow_dispatch:",
		"- 'site/**'",
		"permissions:\n  contents: read\n  pages: write\n  id-token: write\n",
		"environment:\n      name: github-pages",
		"actions/configure-pages@",
		"actions/upload-pages-artifact@",
		"actions/deploy-pages@",
		"path: site",
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
		if ext := filepath.Ext(p); ext == ".html" || ext == ".css" {
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			files[filepath.ToSlash(p[len(root)+1:])] = string(b)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestSiteIsSelfContained(t *testing.T) {
	files := siteFiles(t)
	html, css := files["index.html"], files["style.css"]
	if html == "" || css == "" {
		t.Fatal("site/index.html and site/style.css are required")
	}
	dashes := string([]rune{0x2014, 0x2013, 0x2018, 0x2019, 0x201c, 0x201d})
	words := regexp.MustCompile(`(?i)(` + strings.Join(toolWords, "|") + `)`)
	embedded := regexp.MustCompile(`(?i)(src|href)="(https?:)?//[^"]*"|@import|url\(\s*["']?(https?:)?//`)
	for name, body := range files {
		if strings.ContainsAny(body, dashes) {
			t.Errorf("%s contains a dash or curly quote", name)
		}
		if m := words.FindString(body); m != "" {
			t.Errorf("%s mentions %q", name, m)
		}
		for _, u := range regexp.MustCompile(`https?://[^\s"'<>)]+`).FindAllString(body, -1) {
			if !strings.HasPrefix(u, repoURL) && !strings.HasPrefix(u, "https://quavergame.com") {
				t.Errorf("%s links to %s", name, u)
			}
		}
		if strings.Contains(body, "<script") {
			t.Errorf("%s has a script", name)
		}
	}
	for _, m := range embedded.FindAllString(html, -1) {
		if !strings.HasPrefix(m, "href=") {
			t.Errorf("index.html loads a remote resource: %s", m)
		}
	}
	if embedded.MatchString(css) {
		t.Error("style.css loads a remote resource")
	}
}

func TestSiteContent(t *testing.T) {
	files := siteFiles(t)
	html := files["index.html"]
	for _, want := range []string{
		`<html lang="en">`, `name="viewport"`, "<title>", `rel="stylesheet" href="style.css"`,
		"qsc --open", repoURL + "/releases/latest", "not affiliated with Quaver",
		"quaver.db", "read-only", "127.0.0.1", "Linux", repoURL + "/blob/main/LICENSE",
		repoURL + "/blob/main/docs/OBS.md", repoURL + "/blob/main/docs/OVERLAYS.md", repoURL + "/blob/main/docs/THEMES.md",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("index.html lacks %q", want)
		}
	}
	if !strings.Contains(files["style.css"], "prefers-color-scheme: dark") {
		t.Error("style.css has no dark scheme")
	}
	imgs := regexp.MustCompile(`<img [^>]*>`).FindAllString(html, -1)
	if len(imgs) < 4 {
		t.Errorf("gallery has %d screenshots, want at least 4", len(imgs))
	}
	for _, img := range imgs {
		for _, attr := range []string{"alt", "width", "height"} {
			if !regexp.MustCompile(`\s` + attr + `="[^"]+"`).MatchString(img) {
				t.Errorf("%s lacks %s", img, attr)
			}
		}
		src := regexp.MustCompile(`src="([^"]+)"`).FindStringSubmatch(img)
		if src == nil {
			t.Errorf("%s lacks src", img)
			continue
		}
		if _, err := os.Stat(filepath.Join("..", "site", src[1])); err != nil {
			t.Errorf("image %s is missing", src[1])
		}
	}
}

func TestSiteImagesAreSmall(t *testing.T) {
	var total int64
	err := filepath.WalkDir(filepath.Join("..", "site", "img"), func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if filepath.Ext(p) != ".png" {
			t.Errorf("%s is not a png", p)
		}
		info, err := d.Info()
		total += info.Size()
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if total > 600*1024 {
		t.Errorf("screenshots weigh %d bytes, limit is 614400", total)
	}
}
