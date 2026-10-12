package web

import (
	"io/fs"
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

func TestObsPageIsWiredBetweenOverlaysAndHistory(t *testing.T) {
	index, err := fs.ReadFile(FS, "dashboard/index.html")
	if err != nil {
		t.Fatal(err)
	}
	catalog := strings.Index(string(index), `src="pages/overlay-catalog.js"`)
	page := strings.Index(string(index), `src="pages/obs.js"`)
	if page < 0 || catalog < 0 || page < catalog {
		t.Error("index.html must load pages/obs.js after the overlay catalog")
	}
	src, err := fs.ReadFile(FS, "dashboard/pages/obs.js")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), "registerPage('obs'") {
		t.Error("obs page must register itself")
	}
	app, err := fs.ReadFile(FS, "dashboard/app.js")
	if err != nil {
		t.Fatal(err)
	}
	a := string(app)
	o, b, h := strings.Index(a, "id: 'overlays'"), strings.Index(a, "id: 'obs'"), strings.Index(a, "id: 'history'")
	if b < 0 || !(o < b && b < h) {
		t.Error("nav item obs must sit between overlays and history")
	}
}

func TestObsPageUsesCatalogAndStatus(t *testing.T) {
	src, err := fs.ReadFile(FS, "dashboard/pages/obs.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"qscOverlayCatalog", "cat.OVERLAYS", "/api/status", "AbortController", "aria-live"} {
		if !strings.Contains(string(src), want) {
			t.Errorf("obs.js must use %s", want)
		}
	}
}

func TestOverlaysCopySizeButtonHasAccessibleName(t *testing.T) {
	src, err := fs.ReadFile(FS, "dashboard/pages/overlays.js")
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`copySize:[^\n]*overlays\.copy_size_aria`).Match(src) {
		t.Error("the copy size button needs an aria-label from overlays.copy_size_aria")
	}
}

func TestGuideDocsMatchTheCatalog(t *testing.T) {
	guide := readDoc(t, "docs/OBS.md")
	readme := readDoc(t, "README.md")
	for _, size := range []string{"540 x 140", "520 x 230", "520 x 180", "280 x 64", "520 x 120"} {
		if !strings.Contains(guide, size) {
			t.Errorf("docs/OBS.md does not mention %s", size)
		}
		if !strings.Contains(readme, size) {
			t.Errorf("README.md does not mention %s", size)
		}
	}
	if !strings.Contains(readme, "docs/OBS.md") {
		t.Error("README.md must link docs/OBS.md")
	}
}

func TestObsSizesMatchPlan(t *testing.T) {
	runNodeModel(t, "dashboard/pages/overlay-catalog.js", `
const assert = require('assert');
const c = require(process.argv[2]);
const got = c.OVERLAYS.map((o) => o.id + ':' + c.sizeText(o)).join(',');
assert.strictEqual(got, 'now-playing:540 x 140,result:520 x 230,history:520 x 180,state:280 x 64,alert:520 x 120,keys:480 x 200');
`)
}

func TestObsPageHasACreateYourOwnCard(t *testing.T) {
	src, err := fs.ReadFile(FS, "dashboard/pages/obs.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"/api/overlays", "docs/OVERLAYS.md", "obs.custom.title", "obs.custom.body", "obs.custom.link", "obs.custom.folder", "obs.custom.folder_unavailable", "noopener", "pathNodes"} {
		if !strings.Contains(string(src), want) {
			t.Errorf("obs.js must use %s", want)
		}
	}
}

func TestObsCardShowsTheOverlaysFolder(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not on PATH")
	}
	out, err := exec.Command(node, mustAbs(t, "testdata/obs_card_harness.js"), mustAbs(t, "dashboard/pages/obs.js")).CombinedOutput()
	if err != nil {
		t.Fatalf("obs card: %v: %s", err, out)
	}
}
