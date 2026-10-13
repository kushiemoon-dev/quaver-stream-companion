package packaging

import (
	"os"
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
		"actions/checkout@v4", "actions/setup-go@v5",
		"actions/upload-artifact@v4", "actions/download-artifact@v4",
	} {
		if !strings.Contains(wf, want) {
			t.Errorf("release.yml does not contain %q", want)
		}
	}
	allowed := regexp.MustCompile(`^actions/(checkout@v4|setup-go@v5|setup-node@v4|upload-artifact@v4|download-artifact@v4)$`)
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
	for _, want := range []string{"actions/setup-node@v4", "go vet ./...", "go test ./... -count=1"} {
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
