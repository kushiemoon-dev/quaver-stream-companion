package web

import (
	"strings"
	"testing"
)

func TestDashboardDoesNotRepeatBadgesAsModChips(t *testing.T) {
	runNodeModel(t, "dashboard/pages/history-model.js", `
const assert = require('assert');
const m = require(process.argv[2]);
assert.deepStrictEqual(m.modsWithoutBadges(['Paused', 'Mirror', 'Autoplay'], { paused: true, autoplay: false }), ['Mirror', 'Autoplay']);
assert.deepStrictEqual(m.modsWithoutBadges(['Paused', 'Autoplay'], { paused: true, autoplay: true }), []);
assert.deepStrictEqual(m.modsWithoutBadges(['Paused'], {}), ['Paused']);
assert.deepStrictEqual(m.modsWithoutBadges(null, { paused: true }), []);
assert.deepStrictEqual(m.modsWithoutBadges(['__proto__', 'Mirror'], { paused: true }), ['__proto__', 'Mirror']);
`)
	for _, f := range []string{"dashboard/pages/history.js", "dashboard/pages/status.js"} {
		if !strings.Contains(readWeb(t, f), "modsWithoutBadges(") {
			t.Errorf("%s must filter the mods it shows next to the badges", f)
		}
	}
}
