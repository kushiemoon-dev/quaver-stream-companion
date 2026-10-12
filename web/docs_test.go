package web

import (
	"io/fs"
	"os"
	"path/filepath"
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
	for _, rel := range []string{"README.md", "docs/THEMES.md"} {
		if strings.ContainsAny(readDoc(t, rel), string([]rune{0x2014, 0x2013})) {
			t.Errorf("%s contains an em or en dash", rel)
		}
	}
}
