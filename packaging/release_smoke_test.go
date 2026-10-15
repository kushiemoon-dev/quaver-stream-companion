package packaging

import (
	"regexp"
	"strings"
	"testing"
)

func TestReleaseSmokesTheArchivesBeforeTheDraft(t *testing.T) {
	wf := read(t, ".github/workflows/release.yml")
	lin, win, rel := jobBlock(t, wf, "smoke-linux"), jobBlock(t, wf, "smoke-windows"), jobBlock(t, wf, "release")
	for _, want := range []string{"os: ubuntu-24.04-arm", "tar -xzf dist/qsc_*_linux_", "scripts/smoke/drive.mjs", "APPIMAGE_EXTRACT_AND_RUN: '1'", "pattern: dist-*"} {
		if !strings.Contains(lin, want) {
			t.Errorf("smoke-linux does not contain %q", want)
		}
	}
	for _, want := range []string{"windows-latest", "Expand-Archive", "scripts/smoke/drive.mjs", "name: dist-windows-amd64"} {
		if !strings.Contains(win, want) {
			t.Errorf("smoke-windows does not contain %q", want)
		}
	}
	for name, block := range map[string]string{"smoke-linux": lin, "smoke-windows": win} {
		if strings.Contains(block, "permissions:") || strings.Contains(block, "secrets.") || strings.Contains(block, "github.token") {
			t.Errorf("%s must run with the read only default token and no secrets", name)
		}
	}
	needs := regexp.MustCompile(`(?m)^    needs:.*`).FindString(rel)
	for _, job := range []string{"smoke-linux", "smoke-windows"} {
		if !strings.Contains(needs, job) {
			t.Errorf("the release job must wait for %s", job)
		}
		if !strings.Contains(jobBlock(t, wf, job), "--expect-version") {
			t.Errorf("%s must check the version against the tag", job)
		}
	}
}
