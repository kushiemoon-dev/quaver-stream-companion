package web

import (
	"strings"
	"testing"
)

func TestSessionViewModel(t *testing.T) {
	runNodeModel(t, "overlays/model.js", `
const assert = require('assert');
const m = require(process.argv[2]);
const nb = String.fromCharCode(160);
const t0 = Date.parse('2026-01-01T00:00:00Z');
assert.strictEqual(m.formatElapsed(0), '0:00');
assert.strictEqual(m.formatElapsed(59999), '0:59');
assert.strictEqual(m.formatElapsed(61000), '1:01');
assert.strictEqual(m.formatElapsed(3600000), '1:00:00');
assert.strictEqual(m.formatElapsed(3723000), '1:02:03');
for (const bad of [-5000, NaN, null, undefined, 'x', {}]) assert.strictEqual(m.formatElapsed(bad), '0:00');
assert.strictEqual(m.formatElapsed(1e15), '99:59:59');
const v = m.sessionView({ started_at: '2026-01-01T00:00:00Z', plays: 5, passes: 4, fails: 1, avg_accuracy: 90.126, best_accuracy: 99.5, best_rating: 12.345, full_combos: 1, streak: 2, best_streak: 3 }, t0 + 125000, 'fr');
assert.deepStrictEqual(v, { elapsed: '2:05', plays: '5', outcome: '4 / 1', average: '90,13' + nb + '%', best: '99,50' + nb + '%', streak: '2', bestStreak: '3', fullCombos: '1', rating: '12,35' });
const none = m.sessionView({ plays: 2, passes: 0, fails: 2, avg_accuracy: 0 }, t0, 'en');
assert.strictEqual(none.average, '-');
assert.strictEqual(none.best, '-');
assert.strictEqual(none.rating, '-');
assert.strictEqual(none.elapsed, '0:00');
for (const bad of [null, undefined, 'x', 5, []]) assert.strictEqual(m.sessionView(bad, t0, 'en').plays, '0');
assert.strictEqual(m.sessionView({ passes: 1, best_rating: 1e30 }, t0, 'en').rating, '9999.00');
assert.strictEqual(m.sessionView({ passes: 1, best_rating: 'x' }, t0, 'en').rating, '-');
assert.strictEqual(m.sessionView({ plays: 7.9 }, t0, 'en').plays, '7');
`)
}

func TestSessionIsDocumented(t *testing.T) {
	for _, doc := range []string{"README.md", "docs/OBS.md", "docs/OVERLAYS.md"} {
		text := readDoc(t, doc)
		for _, want := range []string{"/overlays/session/", "480 x 170"} {
			if !strings.Contains(text, want) {
				t.Errorf("%s does not mention %q", doc, want)
			}
		}
	}
	for _, want := range []string{"session.updated", "POST /api/session/reset", "best_streak"} {
		if !strings.Contains(readDoc(t, "docs/OVERLAYS.md"), want) {
			t.Errorf("docs/OVERLAYS.md does not mention %q", want)
		}
	}
	if !strings.Contains(readDoc(t, "README.md"), "POST /api/session/reset") {
		t.Error("README.md must list the reset endpoint")
	}
}

func TestStatusSessionCardIsWiredBeforeTheStatusPage(t *testing.T) {
	index := readWeb(t, "dashboard/index.html")
	card, page := strings.Index(index, `src="pages/status-session.js"`), strings.Index(index, `src="pages/status.js"`)
	if card < 0 || card > page {
		t.Error("status-session.js must be loaded before status.js")
	}
	src := readWeb(t, "dashboard/pages/status-session.js")
	for _, want := range []string{"/api/session/reset", "aria-live", "AbortController", "aria-disabled", "status.session_reset"} {
		if !strings.Contains(src, want) {
			t.Errorf("status-session.js is missing %q", want)
		}
	}
	for _, bad := range []string{"innerHTML", "outerHTML", "insertAdjacentHTML"} {
		if strings.Contains(src, bad) {
			t.Errorf("status-session.js uses %s", bad)
		}
	}
}
