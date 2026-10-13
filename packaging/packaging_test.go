package packaging

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func read(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestReleaseWorkflow(t *testing.T) {
	wf := read(t, ".github/workflows/release.yml")
	for _, want := range []string{
		"tags:", "'v*'", "go vet ./...", "go test ./... -count=1",
		"windows", "linux", "amd64", "arm64",
		"packaging/build-archive.sh", "packaging/build-appimage.sh",
		"checksums.txt", "sha256sum", "gh release create", "--draft",
		"actions/checkout@", "actions/setup-go@",
		"actions/upload-artifact@", "actions/download-artifact@",
	} {
		if !strings.Contains(wf, want) {
			t.Errorf("release.yml does not contain %q", want)
		}
	}
	allowed := regexp.MustCompile(`^actions/(checkout|setup-go|setup-node|upload-artifact|download-artifact)@[0-9a-f]{40}$`)
	for _, m := range regexp.MustCompile(`(?m)uses:\s*(\S+)`).FindAllStringSubmatch(wf, -1) {
		if !allowed.MatchString(m[1]) {
			t.Errorf("action %q is not allowed", m[1])
		}
	}
}

func TestBuildScripts(t *testing.T) {
	archive := read(t, "packaging/build-archive.sh")
	for _, want := range []string{"CGO_ENABLED=0", "-trimpath", "-s -w -X github.com/kushiemoon-dev/quaver-stream-companion/internal/version.Version=", "LICENSE", "README.md", "docs/"} {
		if !strings.Contains(archive, want) {
			t.Errorf("build-archive.sh does not contain %q", want)
		}
	}
	app := read(t, "packaging/build-appimage.sh")
	if !regexp.MustCompile(`APPIMAGETOOL_SHA256="[0-9a-f]{64}"`).MatchString(app) {
		t.Error("build-appimage.sh does not pin appimagetool by sha256")
	}
	for _, want := range []string{"sha256sum -c", "APPIMAGETOOL_VERSION=", "packaging/appimage/AppRun", "web/dashboard/favicon.svg"} {
		if !strings.Contains(app, want) {
			t.Errorf("build-appimage.sh does not contain %q", want)
		}
	}
}

func TestPackagingFiles(t *testing.T) {
	if got := read(t, "packaging/appimage/AppRun"); !strings.Contains(got, "serve --open") {
		t.Error("AppRun must run qsc serve --open")
	}
	desktop := read(t, "packaging/appimage/qsc.desktop")
	for _, want := range []string{"Type=Application", "Exec=qsc", "Icon=qsc", "Terminal=false"} {
		if !strings.Contains(desktop, want) {
			t.Errorf("qsc.desktop does not contain %q", want)
		}
	}
	unit := read(t, "packaging/qsc.service")
	for _, want := range []string{"[Service]", "ExecStart=", " serve", "WantedBy=default.target"} {
		if !strings.Contains(unit, want) {
			t.Errorf("qsc.service does not contain %q", want)
		}
	}
	if !strings.Contains(unit, "Restart=on-failure") || strings.Contains(unit, "Restart=always") {
		t.Error("qsc.service must restart only on failure so a clean quit stays stopped")
	}
	if strings.Contains(unit, "--open") {
		t.Error("qsc.service must not open a browser")
	}
}

func TestNoDashes(t *testing.T) {
	for _, rel := range []string{
		".github/workflows/release.yml", "docs/INSTALL.md", "README.md",
		"packaging/build-archive.sh", "packaging/build-appimage.sh", "packaging/qsc.service",
		"packaging/appimage/AppRun", "packaging/appimage/qsc.desktop",
	} {
		if strings.ContainsAny(read(t, rel), string([]rune{0x2014, 0x2013})) {
			t.Errorf("%s contains an em or en dash", rel)
		}
	}
}

func TestInstallDocCoversTopics(t *testing.T) {
	doc := read(t, "docs/INSTALL.md")
	for _, want := range []string{"checksums.txt", "sha256sum", "shell:startup", "qsc.service", "systemctl --user", "AppImage", "not been run", "Uninstall", "config.json"} {
		if !strings.Contains(doc, want) {
			t.Errorf("INSTALL.md does not mention %q", want)
		}
	}
}

// jobBlock returns the text of one top-level job of release.yml.
func jobBlock(t *testing.T, wf, name string) string {
	t.Helper()
	jobs := wf[strings.Index(wf, "\njobs:\n"):]
	re := regexp.MustCompile(`(?ms)^  ` + name + `:\n(.*?)(?:^  [a-z]+:\n|\z)`)
	m := re.FindStringSubmatch(jobs)
	if m == nil {
		t.Fatalf("job %q not found in release.yml", name)
	}
	return m[1]
}

func TestReleaseWorkflowStructure(t *testing.T) {
	wf := read(t, ".github/workflows/release.yml")

	if !regexp.MustCompile(`(?m)^on:\n  push:\n    tags:\n      - 'v\*'\n`).MatchString(wf) {
		t.Error("the only trigger must be a push of tags v*")
	}
	if !regexp.MustCompile(`(?m)^permissions:\n  contents: read\n`).MatchString(wf) {
		t.Error("top level permissions must be contents: read")
	}
	if n := strings.Count(wf, "contents: write"); n != 1 {
		t.Errorf("contents: write appears %d times, want once", n)
	}
	if !strings.Contains(jobBlock(t, wf, "release"), "contents: write") {
		t.Error("contents: write must be inside the release job")
	}

	for job, needs := range map[string]string{
		"build":    `needs: test`,
		"appimage": `needs: test`,
		"release":  `needs: [build, appimage]`,
	} {
		if !strings.Contains(jobBlock(t, wf, job), needs) {
			t.Errorf("job %s must have %q", job, needs)
		}
	}

	gate := jobBlock(t, wf, "test")
	for _, want := range []string{"actions/setup-node@", "go vet ./...", "go test ./... -count=1"} {
		if !strings.Contains(gate, want) {
			t.Errorf("test job does not contain %q", want)
		}
	}

	rel := jobBlock(t, wf, "release")
	for _, want := range []string{"gh release create", "--draft", "--verify-tag", "sha256sum"} {
		if !strings.Contains(rel, want) {
			t.Errorf("release job does not contain %q", want)
		}
	}
}

func TestWorkflowsArePinned(t *testing.T) {
	for _, rel := range []string{".github/workflows/ci.yml", ".github/workflows/release.yml"} {
		wf := read(t, rel)
		uses := regexp.MustCompile(`(?m)^\s*- uses:\s*(\S+)(.*)$`).FindAllStringSubmatch(wf, -1)
		if len(uses) == 0 {
			t.Fatalf("%s has no actions", rel)
		}
		for _, m := range uses {
			if !regexp.MustCompile(`@[0-9a-f]{40}$`).MatchString(m[1]) {
				t.Errorf("%s: %q is not pinned to a full commit SHA", rel, m[1])
			}
			if !regexp.MustCompile(`^\s+# v\d+$`).MatchString(m[2]) {
				t.Errorf("%s: %q needs a trailing major tag comment", rel, m[1])
			}
		}
	}
	if !regexp.MustCompile(`(?m)^permissions:\n  contents: read\n`).MatchString(read(t, ".github/workflows/ci.yml")) {
		t.Error("ci.yml must declare top level permissions: contents: read")
	}
	rel := read(t, ".github/workflows/release.yml")
	if !strings.Contains(rel, "--prerelease") || !strings.Contains(rel, "--draft") {
		t.Error("release.yml must keep --draft and pass --prerelease for tags with a hyphen")
	}
}

func TestDependabotAndToolchain(t *testing.T) {
	dep := read(t, ".github/dependabot.yml")
	for _, eco := range []string{"gomod", "github-actions"} {
		if !strings.Contains(dep, "package-ecosystem: "+eco) {
			t.Errorf("dependabot.yml lacks %s", eco)
		}
	}
	if strings.Count(dep, "interval: weekly") != 2 {
		t.Error("dependabot.yml must update both ecosystems weekly")
	}
	if !regexp.MustCompile(`(?m)^toolchain go1\.26\.\d+$`).MatchString(read(t, "go.mod")) {
		t.Error("go.mod must pin a go1.26.x toolchain line")
	}
}

func TestThirdPartyNoticesCoverLinkedModules(t *testing.T) {
	gobin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go is not on PATH: cannot compute the linked modules")
	}
	notices := read(t, "THIRD_PARTY_NOTICES.md")
	for _, target := range [][2]string{{"windows", "amd64"}, {"linux", "amd64"}, {"linux", "arm64"}} {
		cmd := exec.Command(gobin, "list", "-deps", "-f", "{{with .Module}}{{if not .Main}}{{.Path}} {{.Version}}{{end}}{{end}}", "./cmd/qsc")
		cmd.Dir = ".."
		cmd.Env = append(os.Environ(), "GOOS="+target[0], "GOARCH="+target[1], "CGO_ENABLED=0")
		out, err := cmd.Output()
		if err != nil {
			t.Skipf("go list failed (offline module cache?): %v", err)
		}
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if line == "" {
				continue
			}
			if !strings.Contains(notices, "## "+line) {
				t.Errorf("%s/%s: THIRD_PARTY_NOTICES.md lacks a section for %q", target[0], target[1], line)
			}
		}
	}
	for _, want := range []string{"VT323", "SIL Open Font License", "Catppuccin", "## Go standard library", "The Go Authors. All rights reserved."} {
		if !strings.Contains(notices, want) {
			t.Errorf("THIRD_PARTY_NOTICES.md does not mention %q", want)
		}
	}
}

func TestNoticesShipInEveryPackage(t *testing.T) {
	for _, rel := range []string{"packaging/build-archive.sh", "packaging/build-appimage.sh"} {
		if !strings.Contains(read(t, rel), "THIRD_PARTY_NOTICES.md") {
			t.Errorf("%s does not copy THIRD_PARTY_NOTICES.md", rel)
		}
	}
	if !regexp.MustCompile(`LICENSE[^\n]*"\$appdir`).MatchString(read(t, "packaging/build-appimage.sh")) {
		t.Error("build-appimage.sh must place LICENSE in the AppDir")
	}
}

func TestSecurityPolicy(t *testing.T) {
	doc := read(t, "SECURITY.md")
	for _, want := range []string{"Supported versions", "Security tab", "Report a vulnerability", "In scope", "Out of scope", "keys"} {
		if !strings.Contains(doc, want) {
			t.Errorf("SECURITY.md does not mention %q", want)
		}
	}
	for _, rel := range []string{"SECURITY.md", "THIRD_PARTY_NOTICES.md", ".github/workflows/ci.yml", ".github/dependabot.yml"} {
		if strings.ContainsAny(read(t, rel), string([]rune{0x2014, 0x2013})) {
			t.Errorf("%s contains an em or en dash", rel)
		}
	}
}

func TestThemesDocExplainsTheCSP(t *testing.T) {
	doc := read(t, "docs/THEMES.md")
	for _, want := range []string{"A remote `@import` in a custom theme is blocked", "falls back to the `neutral-light` theme", "only that font or image is lost", "`data:` URI font"} {
		if !strings.Contains(doc, want) {
			t.Errorf("THEMES.md does not say %q", want)
		}
	}
}

func TestWorkflowsUseTheToolchainOfGoMod(t *testing.T) {
	want := regexp.MustCompile(`(?m)^toolchain go(1\.26\.\d+)$`).FindStringSubmatch(read(t, "go.mod"))
	if want == nil {
		t.Fatal("go.mod has no go1.26.x toolchain line")
	}
	for _, rel := range []string{".github/workflows/ci.yml", ".github/workflows/release.yml"} {
		wf := read(t, rel)
		n := strings.Count(wf, "actions/setup-go@")
		if n == 0 || strings.Count(wf, "go-version: '"+want[1]+"'") != n || strings.Count(wf, "check-latest: false") != n {
			t.Errorf("%s: every setup-go step must set go-version '%s' and check-latest: false", rel, want[1])
		}
		if strings.Contains(wf, "go-version-file") {
			t.Errorf("%s: go-version-file ignores the toolchain line", rel)
		}
	}
}

func TestOverlaysDocMentionsTheClientLimit(t *testing.T) {
	if !strings.Contains(read(t, "docs/OVERLAYS.md"), "at most 64 event streams") {
		t.Error("OVERLAYS.md must document the 64 client limit")
	}
}
