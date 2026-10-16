package web

import (
	"regexp"
	"strings"
	"testing"
)

func TestCurveOverlayPage(t *testing.T) {
	src := readPage(t, "curve")
	for _, want := range []string{
		"'score.saved'", "qscModel.timelineView(", "qscAutohide(", "params.demo",
		"'/api/scores/' + ", "'/timeline'", "AbortController", "FETCH_TIMEOUT_MS", "credentials: 'same-origin'",
		"qscT('curve.title')", "qscT('curve.label')", "createElementNS",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("curve overlay is missing %q", want)
		}
	}
	if regexp.MustCompile(`innerHTML|outerHTML|insertAdjacentHTML`).MatchString(src) {
		t.Error("curve must not write markup")
	}
	css := readWeb(t, "overlays/overlay.css")
	for _, want := range []string{".curve .miss", ".curve text.final"} {
		if !strings.Contains(css, want) {
			t.Errorf("overlay.css is missing %q", want)
		}
	}
	for _, name := range []string{"now-playing", "result", "history", "state", "alert", "keys"} {
		if strings.Contains(readPage(t, name), "/timeline") {
			t.Errorf("%s must not fetch timelines", name)
		}
	}
}

func TestCurveCatalogsHaveEveryKey(t *testing.T) {
	src := readWeb(t, "overlays/i18n.js")
	for _, k := range []string{"curve.title", "curve.label", "curve.misses"} {
		if n := strings.Count(src, "'"+k+"'"); n != 3 {
			t.Errorf("overlay catalogs must define %s in en, fr and de (found %d)", k, n)
		}
	}
}

func TestCurveModel(t *testing.T) {
	runNodeModel(t, "overlays/model.js", curveModelScript)
}

const curveModelScript = `
const assert = require('assert');
const m = require(process.argv[2]);
const nan = NaN;

const pts = Array.from({ length: 100 }, (_, i) => 99 - i * 0.01);
const v = m.timelineView({ score_id: 1, total: 1000, points: pts, misses: [0, 500, 999, -1, 1000, 'x', 3.5, null], counts: [] }, 520, 180);
assert.strictEqual(v.points.length, 100);
assert.ok(v.points.every((p, i) => p.x >= v.left && p.x <= v.right && p.y >= v.yTop && p.y <= v.yBottom && (i === 0 || p.x > v.points[i - 1].x)));
assert.strictEqual(v.points[99].x, v.right);
assert.strictEqual(v.misses.length, 3);
assert.ok(v.misses.every((x) => x >= v.left && x <= v.right));
assert.ok(v.misses[0] < v.misses[1] && v.misses[1] < v.misses[2]);
assert.strictEqual(v.final, '98.01%');
assert.strictEqual(v.yMax, 100);
assert.ok(v.yMin <= 90 && v.yMin % 5 === 0);

const low = m.timelineView({ total: 20, points: [0, 92, 93, 94, 95, 95, 96, 97, 97, 98] }, 520, 180);
assert.strictEqual(low.yMin, 90, 'the first tenth of the points does not stretch the axis');
assert.ok(low.points.every((p) => p.y >= low.yTop && p.y <= low.yBottom));
const dive = m.timelineView({ total: 10, points: [100, 90, 60, 20, 10] }, 520, 180);
assert.strictEqual(dive.yMin, 10);

assert.strictEqual(m.timelineView({ total: 1, points: [100] }, 520, 180).points.length, 1);
assert.strictEqual(m.timelineView({ points: [95, 96] }, 520, 180).points.length, 2);

for (const bad of [null, undefined, 'x', 7, [], {}, { points: [] }, { points: 'x' }, { points: [nan, null, 'a'] }]) assert.strictEqual(m.timelineView(bad, 520, 180), null);

const big = m.timelineView({ total: 1e12, points: Array.from({ length: 5000 }, () => 1e30), misses: Array.from({ length: 5000 }, (_, i) => i) }, 520, 180);
assert.strictEqual(big.points.length, 200);
assert.ok(big.misses.length <= 500);
assert.strictEqual(big.final, '100.00%');
const neg = m.timelineView({ total: 3, points: [-50, nan, 20] }, 520, 180);
assert.strictEqual(neg.points.length, 2);
assert.ok(neg.points.every((p) => p.y <= neg.yBottom && p.y >= neg.yTop));
assert.ok(m.timelineView({ total: 5, points: [90], misses: { length: 3 } }, 520, 180).misses.length === 0);
`

func TestCurveIsDocumented(t *testing.T) {
	for _, doc := range []string{"README.md", "docs/OBS.md", "docs/OVERLAYS.md"} {
		text := readDoc(t, doc)
		for _, want := range []string{"/overlays/curve/", "520 x 180"} {
			if !strings.Contains(text, want) {
				t.Errorf("%s does not mention %q", doc, want)
			}
		}
	}
	guide := readDoc(t, "docs/OVERLAYS.md")
	for _, want := range []string{
		"GET /api/scores/{id}/timeline", "score_id", "misses", "counts", "judgment index", "98.25", "no timestamps", "404",
		"`curve`",
	} {
		if !strings.Contains(guide, want) {
			t.Errorf("docs/OVERLAYS.md does not mention %q", want)
		}
	}
	if !strings.Contains(readDoc(t, "README.md"), "GET /api/scores/{id}/timeline") {
		t.Error("README.md must list the timeline endpoint")
	}
}
