package web

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/bus"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/overlays"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/scores"
)

var guideExamples = []string{
	"docs/examples/mini-card/index.html",
	"docs/examples/mini-card/overlay.json",
	"docs/examples/theme-card/index.html",
	"docs/examples/theme-card/overlay.json",
	"docs/examples/clients/watch.mjs",
	"docs/examples/clients/watch.py",
}

func jsonTags(t reflect.Type, out map[string]bool) {
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if tag := strings.Split(f.Tag.Get("json"), ",")[0]; tag != "" && tag != "-" {
			out[tag] = true
		}
		if f.Type.Kind() == reflect.Struct {
			jsonTags(f.Type, out)
		}
	}
}

func TestOverlayGuideCoversTheContract(t *testing.T) {
	guide := readDoc(t, "docs/OVERLAYS.md")
	tags := map[string]bool{}
	for _, v := range []any{scores.Result{}, bus.Event{}, bus.GameState{}} {
		jsonTags(reflect.TypeOf(v), tags)
	}
	for _, k := range []string{"game", "map", "last_score", "id", "played_at", "keys", "limit", "checks", "uptime_s"} {
		tags[k] = true
	}
	for tag := range tags {
		if !strings.Contains(guide, "`"+tag+"`") {
			t.Errorf("docs/OVERLAYS.md does not document the field %q", tag)
		}
	}
	for _, want := range []string{
		bus.TypeSnapshot, bus.TypeGameState, bus.TypeMapStarted, bus.TypeScoreSaved,
		"GET /state", "GET /events", "/api/history", "/api/status", "/api/overlays",
		"`theme`", "`accent`", "`lang`", "`autohide`", "`scale`", "`demo`", "`n`", "`hide`",
		"initialization", "menu", "select", "loading", "gameplay", "results", "closed", "unknown",
		"overlay.json", "Access-Control-Allow-Origin", "never put secrets", "sandbox", "THEMES.md",
		"qscConnect", "textContent", "Troubleshooting", "curl", "Stream Deck", "Twitch", "WebSocket",
		"2 MiB", "200 files", "100 overlays", "8 KiB",
		"%APPDATA%", "~/.config/quaver-stream-companion/overlays/", "~/Library/Application Support/",
	} {
		if !strings.Contains(guide, want) {
			t.Errorf("docs/OVERLAYS.md does not mention %q", want)
		}
	}
	if overlays.MaxFile != 2<<20 || overlays.MaxFiles != 200 || overlays.MaxListed != 100 || overlays.MaxDepth != 3 {
		t.Error("the limits in docs/OVERLAYS.md must follow internal/overlays")
	}
	for _, id := range []string{"now-playing", "result", "history", "state", "alert", "keys", "curve"} {
		if !strings.Contains(guide, "/overlays/"+id+"/") {
			t.Errorf("docs/OVERLAYS.md does not mention the %s overlay link", id)
		}
	}
}

func TestOverlayGuideEmbedsTheExamples(t *testing.T) {
	guide := readDoc(t, "docs/OVERLAYS.md")
	for _, rel := range guideExamples {
		if !strings.Contains(guide, strings.TrimSpace(readDoc(t, rel))) {
			t.Errorf("docs/OVERLAYS.md must contain %s verbatim", rel)
		}
	}
}

func TestGuideFilesHaveNoDashes(t *testing.T) {
	for _, rel := range append([]string{"docs/OVERLAYS.md"}, guideExamples...) {
		if strings.ContainsAny(readDoc(t, rel), string([]rune{0x2014, 0x2013})) {
			t.Errorf("%s contains an em or en dash", rel)
		}
	}
}

func TestReadmeLinksTheOverlayGuide(t *testing.T) {
	readme := readDoc(t, "README.md")
	if !strings.Contains(readme, "docs/OVERLAYS.md") {
		t.Error("README.md must link docs/OVERLAYS.md")
	}
	if !strings.Contains(readme, "minimized") || !strings.Contains(readme, "--quaver-dir") {
		t.Error("README.md must explain the minimized startup shortcut and --quaver-dir")
	}
	if strings.Contains(readme, "hidden") {
		t.Error("README.md must not claim qsc runs hidden")
	}
}

func TestExampleManifestsAreValidCustomOverlays(t *testing.T) {
	dir := t.TempDir()
	for _, rel := range guideExamples[:4] {
		dst := filepath.Join(dir, strings.TrimPrefix(rel, "docs/examples/"))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dst, []byte(readDoc(t, rel)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	list := overlays.List(dir, func(id string) bool { return overlays.IsBuiltin(FS, id) })
	if len(list) != 2 || list[0].Name != "Mini card" || list[1].Name != "Theme card" || list[0].Description == "" {
		t.Fatalf("list = %+v", list)
	}
}

func TestGuideExamplesRun(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not on PATH")
	}
	if out, err := exec.Command(node, "--check", mustAbs(t, "../docs/examples/clients/watch.mjs")).CombinedOutput(); err != nil {
		t.Fatalf("watch.mjs: %v: %s", err, out)
	}
	out, err := exec.Command(node, mustAbs(t, "testdata/example_harness.js"), mustAbs(t, ".."), mustAbs(t, ".")).CombinedOutput()
	if err != nil {
		t.Fatalf("examples: %v: %s", err, out)
	}
}
