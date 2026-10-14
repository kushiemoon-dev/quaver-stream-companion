package web

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var communityFiles = []string{
	"CONTRIBUTING.md", "CODE_OF_CONDUCT.md", "CHANGELOG.md", "SECURITY.md",
	".github/ISSUE_TEMPLATE/bug_report.yml", ".github/ISSUE_TEMPLATE/config.yml",
	".github/pull_request_template.md", "docs/README.md", "docs/RELEASING.md",
	"docs/releases/v0.1.0-rc1.md",
	"docs/releases/v0.1.0-rc2.md",
}

func TestCommunityFilesExist(t *testing.T) {
	for _, rel := range communityFiles {
		if strings.TrimSpace(readDoc(t, rel)) == "" {
			t.Errorf("%s is empty", rel)
		}
	}
}

func TestREADMELinksTheCommunityFiles(t *testing.T) {
	readme := readDoc(t, "README.md")
	for _, rel := range []string{"CONTRIBUTING.md", "CODE_OF_CONDUCT.md", "CHANGELOG.md", "SECURITY.md", "THIRD_PARTY_NOTICES.md", "docs/RELEASING.md", "docs/README.md"} {
		if !strings.Contains(readme, "]("+rel) {
			t.Errorf("README.md does not link %s", rel)
		}
	}
}

func TestDocsIndexLinksEveryGuide(t *testing.T) {
	index := readDoc(t, "docs/README.md")
	guides, err := filepath.Glob(filepath.Join("..", "docs", "*.md"))
	if err != nil || len(guides) == 0 {
		t.Fatalf("no guides found: %v", err)
	}
	for _, g := range guides {
		name := filepath.Base(g)
		if name != "README.md" && !strings.Contains(index, "]("+name) {
			t.Errorf("docs/README.md does not link %s", name)
		}
	}
}

func TestChangelogHasTheCurrentVersion(t *testing.T) {
	log := readDoc(t, "CHANGELOG.md")
	if !strings.Contains(log, "Keep a Changelog") || !strings.Contains(log, "## [0.1.0-rc1]") {
		t.Error("CHANGELOG.md needs the Keep a Changelog header and a 0.1.0-rc1 entry")
	}
	if !strings.Contains(readDoc(t, "docs/releases/v0.1.0-rc1.md"), "v0.1.0-rc1") {
		t.Error("docs/releases/v0.1.0-rc1.md must name its tag")
	}
}

func TestReleasingListsTheManualRepositorySettings(t *testing.T) {
	doc := readDoc(t, "docs/RELEASING.md")
	for _, want := range []string{"Repository settings to check", "private vulnerability reporting", "topics", "branch protection", "test (ubuntu-latest)", "test (windows-latest)", "test -s", "git push origin main", "homepage", "docs/releases/"} {
		if !strings.Contains(doc, want) {
			t.Errorf("docs/RELEASING.md does not mention %q", want)
		}
	}
}

func TestContributingCoversTheBasics(t *testing.T) {
	doc := readDoc(t, "CONTRIBUTING.md")
	for _, want := range []string{"gofmt -l .", "go vet ./...", "go test ./...", "string(rune(92))", "one line", "docs/THEMES.md"} {
		if !strings.Contains(doc, want) {
			t.Errorf("CONTRIBUTING.md does not mention %q", want)
		}
	}
	if !strings.Contains(readDoc(t, "CODE_OF_CONDUCT.md"), "Contributor Covenant") {
		t.Error("CODE_OF_CONDUCT.md must be the Contributor Covenant")
	}
}

func TestIssueTemplatesAreStructuredYAML(t *testing.T) {
	for _, rel := range []string{".github/ISSUE_TEMPLATE/bug_report.yml", ".github/ISSUE_TEMPLATE/config.yml"} {
		src := readDoc(t, rel)
		if strings.Contains(src, "\t") {
			t.Errorf("%s contains a tab", rel)
		}
		if !regexp.MustCompile(`(?m)^[a-z_]+:`).MatchString(src) {
			t.Errorf("%s has no top level key", rel)
		}
	}
	bug := readDoc(t, ".github/ISSUE_TEMPLATE/bug_report.yml")
	for _, want := range []string{"name:", "description:", "body:", "type: textarea", "qsc doctor --report", "Operating system", "Quaver install"} {
		if !strings.Contains(bug, want) {
			t.Errorf("bug_report.yml does not contain %q", want)
		}
	}
	checkIssueForm(t, "bug_report.yml", bug)
	if !strings.Contains(readDoc(t, ".github/ISSUE_TEMPLATE/config.yml"), "blank_issues_enabled:") {
		t.Error("config.yml needs blank_issues_enabled")
	}
}

// checkIssueForm checks the shape GitHub requires of an issue form with a line based reader:
// the form keys sit at column 0, body items start with "  - type:" and their fields are indented.
func checkIssueForm(t *testing.T, name, src string) {
	t.Helper()
	top := map[string]bool{}
	type item struct {
		kind, id    string
		label       bool
		badRequired string
	}
	var items []item
	section := ""
	for _, line := range strings.Split(src, "\n") {
		trim := strings.TrimSpace(line)
		switch {
		case line != "" && line[0] != ' ' && line[0] != '#':
			if k, _, ok := strings.Cut(line, ":"); ok {
				top[k] = true
			}
		case strings.HasPrefix(line, "  - type: "):
			items = append(items, item{kind: strings.TrimPrefix(line, "  - type: ")})
			section = ""
		case len(items) == 0:
		case strings.HasPrefix(line, "    id: "):
			items[len(items)-1].id = strings.TrimPrefix(line, "    id: ")
		case line == "    attributes:" || line == "    validations:":
			section = trim
		case section == "attributes:" && strings.HasPrefix(line, "      label: "):
			items[len(items)-1].label = true
		case section == "validations:" && strings.HasPrefix(line, "      required: "):
			if v := strings.TrimPrefix(line, "      required: "); v != "true" && v != "false" {
				items[len(items)-1].badRequired = v
			}
		}
	}
	for _, k := range []string{"name", "description", "body"} {
		if !top[k] {
			t.Errorf("%s lacks the top level key %q", name, k)
		}
	}
	if len(items) == 0 {
		t.Fatalf("%s has no body items", name)
	}
	ids := map[string]bool{}
	for _, it := range items {
		if it.kind == "" {
			t.Errorf("%s has a body item without a type", name)
		}
		if it.badRequired != "" {
			t.Errorf("%s: validations.required must be true or false, got %q", name, it.badRequired)
		}
		if it.kind != "input" && it.kind != "textarea" && it.kind != "dropdown" {
			continue
		}
		if it.id == "" || ids[it.id] {
			t.Errorf("%s: %s needs a unique id, got %q", name, it.kind, it.id)
		}
		ids[it.id] = true
		if !it.label {
			t.Errorf("%s: %s %q needs attributes.label", name, it.kind, it.id)
		}
	}
}

func TestNoDashesInAnyPublicMarkdownOrYAML(t *testing.T) {
	skip := map[string]bool{".git": true, "node_modules": true}
	dashes := string([]rune{0x2014, 0x2013})
	root := ".."
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if skip[d.Name()] || ignoredByGit(root, rel) {
				return filepath.SkipDir
			}
			return nil
		}
		if ext := filepath.Ext(path); ext != ".md" && ext != ".yml" && ext != ".yaml" {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.ContainsAny(string(b), dashes) {
			t.Errorf("%s contains an em or en dash", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func ignoredByGit(root, rel string) bool {
	cmd := exec.Command("git", "check-ignore", "-q", rel)
	cmd.Dir = root
	return cmd.Run() == nil
}
