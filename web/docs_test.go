package web

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func readDoc(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestDocsMentionThemesAndTokens(t *testing.T) {
	readme := readDoc(t, "README.md")
	files, err := fs.Glob(FS, "themes/*.css")
	if err != nil || len(files) == 0 {
		t.Fatalf("no themes found: %v", err)
	}
	for _, f := range files {
		id := strings.TrimSuffix(filepath.Base(f), ".css")
		if !strings.Contains(readme, id) {
			t.Errorf("README.md does not mention theme %q", id)
		}
	}
	guide := readDoc(t, "docs/THEMES.md")
	for _, tok := range append(requiredTokens, "scale") {
		if !strings.Contains(guide, "`--"+tok+"`") {
			t.Errorf("docs/THEMES.md does not mention --%s", tok)
		}
	}
}

func TestDocsHaveNoDashes(t *testing.T) {
	for _, rel := range []string{"README.md", "docs/THEMES.md", "docs/OBS.md"} {
		if strings.ContainsAny(readDoc(t, rel), string([]rune{0x2014, 0x2013})) {
			t.Errorf("%s contains an em or en dash", rel)
		}
	}
}

func TestREADMELinksGuidesOverlaysAndScreenshots(t *testing.T) {
	readme := readDoc(t, "README.md")
	guides, err := filepath.Glob(filepath.Join("..", "docs", "*.md"))
	if err != nil || len(guides) == 0 {
		t.Fatalf("no guides found: %v", err)
	}
	for _, g := range guides {
		if rel := "docs/" + filepath.Base(g); !strings.Contains(readme, "]("+rel) {
			t.Errorf("README.md does not link %s", rel)
		}
	}
	dirs, err := fs.ReadDir(FS, "overlays")
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range dirs {
		if d.IsDir() && !strings.Contains(readme, "/overlays/"+d.Name()+"/") {
			t.Errorf("README.md does not list overlay %q", d.Name())
		}
	}
	refs := regexp.MustCompile(`docs/screenshots/[A-Za-z0-9_-]+\.png`).FindAllString(readme, -1)
	if len(refs) < 6 {
		t.Errorf("README.md references %d screenshots, want at least 6", len(refs))
	}
	for _, r := range refs {
		if _, err := os.Stat(filepath.Join("..", r)); err != nil {
			t.Errorf("screenshot %s is missing: %v", r, err)
		}
	}
}

func TestAllDocsHaveNoDashes(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "docs", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range append(files, filepath.Join("..", "README.md")) {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if strings.ContainsAny(string(b), string([]rune{0x2014, 0x2013})) {
			t.Errorf("%s contains an em or en dash", f)
		}
	}
}

func TestInstallWarnsAboutTheRuntimeLogExcerpt(t *testing.T) {
	src := readDoc(t, "docs/INSTALL.md")
	for _, want := range []string{"player name", "map titles", "replay paths"} {
		if !strings.Contains(src, want) {
			t.Errorf("INSTALL.md must warn about %q in the runtime.log excerpt", want)
		}
	}
}
