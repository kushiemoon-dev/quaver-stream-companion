package packaging

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

const smokeWorkflow = ".github/workflows/smoke.yml"

func TestSmokeWorkflowTriggers(t *testing.T) {
	wf := read(t, smokeWorkflow)
	head := wf[:strings.Index(wf, "\njobs:\n")]
	paths := `    paths:
      - 'cmd/**'
      - 'internal/**'
      - 'web/**'
      - 'packaging/**'
      - 'scripts/**'
      - 'go.mod'
      - 'go.sum'
      - '.github/workflows/**'
`
	want := "on:\n  pull_request:\n" + paths + "  push:\n    branches: [main]\n" + paths + "  workflow_dispatch:\npermissions:\n  contents: read\n"
	if !strings.Contains(head, want) {
		t.Errorf("smoke.yml triggers or permissions changed; the head must contain:\n%s", want)
	}
	for _, bad := range []string{"pull_request_target", "schedule:", "secrets.", "contents: write", "id-token", "GITHUB_TOKEN"} {
		if strings.Contains(wf, bad) {
			t.Errorf("smoke.yml must not contain %q", bad)
		}
	}
}

func TestSmokeWorkflowMatrix(t *testing.T) {
	wf := read(t, smokeWorkflow)
	linux := smokeJob(t, wf, "linux")
	for _, want := range []string{
		"fail-fast: false",
		"os: [ubuntu-latest, ubuntu-24.04-arm]",
		"layout: [dot-steam, local-share, flatpak, snap, symlink, library]",
		"--layout", "--exe native", "--exe exe", "--quaver-dir", "--quick",
	} {
		if !strings.Contains(linux, want) {
			t.Errorf("linux job does not contain %q", want)
		}
	}
	if !strings.Contains(smokeJob(t, wf, "windows"), "runs-on: windows-latest") {
		t.Error("the windows job must run on windows-latest")
	}
	app := smokeJob(t, wf, "appimage")
	for _, want := range []string{"packaging/build-appimage.sh", "APPIMAGE_EXTRACT_AND_RUN: '1'", "runs-on: ubuntu-latest"} {
		if !strings.Contains(app, want) {
			t.Errorf("appimage job does not contain %q", want)
		}
	}
	if n := strings.Count(wf, "fail-fast: false"); n != 1 {
		t.Errorf("fail-fast: false appears %d times, want once", n)
	}
}

func TestSmokeNodeSupportsSQLite(t *testing.T) {
	release := read(t, ".github/workflows/release.yml")
	for rel, wf := range map[string]string{smokeWorkflow: read(t, smokeWorkflow), "release.yml smoke jobs": jobBlock(t, release, "smoke-linux") + jobBlock(t, release, "smoke-windows")} {
		n := strings.Count(wf, "actions/setup-node@")
		if n == 0 {
			t.Fatalf("%s has no setup-node step", rel)
		}
		if got := strings.Count(wf, "node:sqlite, which needs Node 22.13 or newer"); got < 1 {
			t.Errorf("%s must say why the Node version is pinned", rel)
		}
		for _, m := range regexp.MustCompile(`node-version: (\d+)`).FindAllStringSubmatch(wf, -1) {
			if v, _ := strconv.Atoi(m[1]); v < 22 {
				t.Errorf("%s: node-version %s has no node:sqlite", rel, m[1])
			}
		}
	}
}

func TestSmokeScriptsUseOnlyBuiltins(t *testing.T) {
	for _, rel := range []string{"scripts/smoke/fixture.mjs", "scripts/smoke/drive.mjs"} {
		src := read(t, rel)
		for _, m := range regexp.MustCompile(`(?m)^import .* from '([^']+)'`).FindAllStringSubmatch(src, -1) {
			if !strings.HasPrefix(m[1], "node:") && !strings.HasPrefix(m[1], "./") {
				t.Errorf("%s imports %q: only Node built-ins are allowed", rel, m[1])
			}
		}
	}
	fixture := read(t, "scripts/smoke/fixture.mjs")
	for _, want := range []string{"JudgementBreakdown", "HasScratchKey", "'Quaver.exe'", "'Quaver'", ".steam", "snap", "com.valvesoftware.Steam", "libraryfolders.vdf", "symlinkSync"} {
		if !strings.Contains(fixture, want) {
			t.Errorf("fixture.mjs does not contain %q", want)
		}
	}
	drive := read(t, "scripts/smoke/drive.mjs")
	for _, want := range []string{"7878, 5001, 5003", "'overlays'", "overlay-catalog.js", "--expect-version", "/events", "/state", "/api/status", "'doctor'", "'stop'"} {
		if !strings.Contains(drive, want) {
			t.Errorf("drive.mjs does not contain %q", want)
		}
	}
}

// smokeJob returns the text of one top-level job of smoke.yml.
func smokeJob(t *testing.T, wf, name string) string {
	t.Helper()
	jobs := wf[strings.Index(wf, "\njobs:\n"):]
	m := regexp.MustCompile(`(?ms)^  ` + name + `:\n(.*?)(?:^  [a-z-]+:\n|\z)`).FindStringSubmatch(jobs)
	if m == nil {
		t.Fatalf("job %q not found in smoke.yml", name)
	}
	return m[1]
}

// runBlocks returns the body of every run: step of a workflow.
func runBlocks(wf string) []string {
	lines := strings.Split(wf, "\n")
	var out []string
	for i, l := range lines {
		trim := strings.TrimLeft(l, " -")
		if !strings.HasPrefix(trim, "run:") {
			continue
		}
		indent := len(l) - len(strings.TrimLeft(l, " "))
		body := strings.TrimPrefix(trim, "run:")
		for _, next := range lines[i+1:] {
			if strings.TrimSpace(next) != "" && len(next)-len(strings.TrimLeft(next, " ")) <= indent {
				break
			}
			body += "\n" + next
		}
		out = append(out, body)
	}
	return out
}

func TestWorkflowHygiene(t *testing.T) {
	for _, rel := range []string{smokeWorkflow, ".github/workflows/release.yml"} {
		wf := read(t, rel)
		blocks := runBlocks(wf)
		if len(blocks) == 0 {
			t.Fatalf("%s: found no run steps", rel)
		}
		for _, b := range blocks {
			if strings.Contains(b, "${{") {
				t.Errorf("%s: an expression is used inside a run step (pass it through env):\n%s", rel, b)
			}
		}
		jobs := wf[strings.Index(wf, "\njobs:\n"):]
		names := regexp.MustCompile(`(?m)^  ([a-z][a-z-]*):\n`).FindAllStringSubmatch(jobs, -1)
		if len(names) == 0 {
			t.Fatalf("%s: no jobs", rel)
		}
		for _, n := range names {
			job := jobs[strings.Index(jobs, "\n  "+n[1]+":\n"):]
			if next := regexp.MustCompile(`\n  [a-z][a-z-]*:\n`).FindStringIndex(job[1:]); next != nil {
				job = job[:next[0]+1]
			}
			if !regexp.MustCompile(`(?m)^    timeout-minutes: \d+$`).MatchString(job) {
				t.Errorf("%s: job %s has no timeout-minutes", rel, n[1])
			}
		}
		checkouts := regexp.MustCompile(`(?m)uses: actions/checkout@\S+.*\n\s+with:\n\s+persist-credentials: false\n`).FindAllString(wf, -1)
		if all := strings.Count(wf, "actions/checkout@"); len(checkouts) != all {
			t.Errorf("%s: %d of %d checkouts set persist-credentials: false", rel, len(checkouts), all)
		}
	}
	if !regexp.MustCompile(`(?m)^concurrency:\n  group: smoke-\$\{\{ github\.ref \}\}\n  cancel-in-progress: true\n`).MatchString(read(t, smokeWorkflow)) {
		t.Error("smoke.yml needs a concurrency group that cancels superseded runs")
	}
	if strings.Contains(read(t, ".github/workflows/release.yml"), "concurrency:") {
		t.Error("release.yml must never cancel a running release")
	}
}
