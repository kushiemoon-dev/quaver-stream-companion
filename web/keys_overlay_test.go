package web

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/keys"
)

func TestKeysOverlayPage(t *testing.T) {
	src := readWeb(t, "overlays/keys/index.html")
	for _, want := range []string{
		"qscConnect(", "{ keys: true }", "'keys.frame'", "snapshot", "qscModel.keysView(", "qscModel.demoKeys(", "qscModel.keysUnknown(", "showHint('unknown_mode')", "qscModel.keysHint(",
		"/api/status", "params.demo", "params.counts", "params.peak", `class="card fill hidden"`,
		"qscT('keys.kps')", "qscT('keys.peak')", "qscT('keys.' + hint)",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("keys overlay is missing %q", want)
		}
	}
	if !strings.Contains(readWeb(t, "overlays/overlay.css"), ".keys-lane") {
		t.Error("overlay.css must style the lanes")
	}
}

func TestKeysOverlayParamsAndOptIn(t *testing.T) {
	common := readWeb(t, "overlays/common.js")
	for _, want := range []string{
		"counts: params.get('counts') !== '0'", "peak: params.get('peak') !== '0'",
		"'/events?keys=1'", "new EventSource(",
	} {
		if !strings.Contains(common, want) {
			t.Errorf("common.js is missing %q", want)
		}
	}
	for _, name := range []string{"now-playing", "result", "history", "state", "alert", "curve"} {
		if strings.Contains(readWeb(t, "overlays/"+name+"/index.html"), "keys: true") {
			t.Errorf("%s must not ask for key frames", name)
		}
	}
}

func TestKeysCatalogsHaveEveryKey(t *testing.T) {
	src := readWeb(t, "overlays/i18n.js")
	for _, k := range []string{"keys.title", "keys.kps", "keys.peak", "keys.off", "keys.unsupported", "keys.lane", "keys.scratch", "keys.unknown_mode", "history.best"} {
		if n := strings.Count(src, "'"+k+"'"); n != 3 {
			t.Errorf("overlay catalogs must define %s in en, fr and de (found %d)", k, n)
		}
	}
}

func TestKeysModel(t *testing.T) {
	runNodeModel(t, "overlays/model.js", keysModelScript)
}

const keysModelScript = `
const assert = require('assert');
const m = require(process.argv[2]);

assert.deepStrictEqual(m.keysView({ layout: '4k', lanes: 4, down: [true, false, 1, 'x'], presses: [1, 2, 3, 4], kps: 7, peak: 9 }),
  { lanes: 4, scratch: false, down: [true, false, false, false], presses: [1, 2, 3, 4], kps: 7, peak: 9 });
const seven = m.keysView({ lanes: 7, down: [true], presses: [] });
assert.strictEqual(seven.lanes, 7);
assert.deepStrictEqual(seven.down, [true, false, false, false, false, false, false]);
assert.deepStrictEqual(seven.presses, [0, 0, 0, 0, 0, 0, 0]);
assert.strictEqual(seven.kps, 0);
for (const bad of [null, undefined, 'x', 7, {}, { lanes: 12 }, { lanes: '4' }, { lanes: 0 }]) assert.strictEqual(m.keysView(bad), null, JSON.stringify(bad));
const wild = m.keysView({ lanes: 4, down: [], presses: [1e12, -5, NaN, 2.9], kps: -1, peak: 'x' });
assert.deepStrictEqual(wild.presses, [999999, 0, 0, 2]);
assert.strictEqual(wild.kps, 0);
assert.strictEqual(wild.peak, 0);
assert.strictEqual(m.keysView({ lanes: 4, down: new Array(100).fill(true) }).down.length, 4);

assert.strictEqual(m.keysHint(null), '');
assert.strictEqual(m.keysHint('x'), '');
assert.strictEqual(m.keysHint({ enabled: false, supported: true }), 'off');
assert.strictEqual(m.keysHint({ enabled: true, supported: false }), 'unsupported');
assert.strictEqual(m.keysHint({ enabled: false, supported: false }), 'unsupported');
assert.strictEqual(m.keysHint({ enabled: true, supported: true }), '');

// the best label sits at the right edge and never on the first data point or another label
const plays = [{ accuracy: 99.5 }, { accuracy: 90 }, { accuracy: 92 }, { accuracy: 91 }];
for (const list of [plays, plays.slice().reverse(), [{ accuracy: 95 }], [{ accuracy: 95 }, { accuracy: 95 }, { accuracy: 95 }]]) {
  const c = m.curve(list, 480, 200);
  const b = m.bestLine(c);
  assert.strictEqual(b.label.anchor, 'end');
  assert.strictEqual(b.label.x, c.right);
  assert.ok(b.label.y >= c.yTop - 4 && b.label.y <= c.yBottom + 12, String(b.label.y));
  const first = c.points[0];
  const nearFirst = Math.abs(first.x - b.label.x) < 80 && Math.abs(first.y - b.label.y) < 12 && c.points.length > 1;
  assert.strictEqual(nearFirst, false);
}
const c = m.curve(plays.slice().reverse(), 480, 200);
const ll = m.lastLabel(c);
const bl = m.bestLine(c);
assert.ok(!(Math.abs(ll.y - bl.label.y) < 12 && ll.anchor === 'end'), 'best label must not sit on the latest accuracy label');
assert.strictEqual(m.lastLabel(m.curve([], 480, 200)), null);
`

func TestSettingsKeysModel(t *testing.T) {
	runNodeModel(t, "dashboard/pages/settings-model.js", settingsKeysScript)
}

const settingsKeysScript = `
const assert = require('assert');
const m = require(process.argv[2]);

assert.strictEqual(m.KEY_NAMES.length, 64);
assert.strictEqual(new Set(m.KEY_NAMES).size, 64);
assert.deepStrictEqual(m.LAYOUTS, ['auto', '1k', '2k', '3k', '4k', '5k', '6k', '7k', '8k', '9k', '10k']);

const defaults = m.keysFromConfig(undefined);
assert.strictEqual(defaults.enabled, false);
assert.strictEqual(defaults.layout, 'auto');
assert.strictEqual(defaults.scratch, 'SHIFT_L');
assert.deepStrictEqual(defaults.k4, ['A', 'S', 'K', 'L']);
assert.deepStrictEqual(defaults.k7, ['A', 'S', 'D', 'SPACE', 'J', 'K', 'L']);
m.LISTS.forEach(function (name) { assert.strictEqual(defaults[name].length, m.LANES[name], name); });
const loaded = m.keysFromConfig({ enabled: true, layout: '7k', k4: ['Q', 'W', 'E', 'R'], k7: ['A', 'B', 'C', 'D', 'E', 'F', 'G'] });
assert.deepStrictEqual(loaded.k4, ['Q', 'W', 'E', 'R']);
assert.strictEqual(loaded.enabled, true);
const broken = m.keysFromConfig({ enabled: 'yes', layout: 'huge', k4: ['Q', 'nope', 'E'], k7: 'x' });
assert.deepStrictEqual(broken, defaults);
assert.notStrictEqual(m.keysFromConfig(undefined).k4, m.keysFromConfig(undefined).k4);

const current = { quaver_dir: 'C:\\Q', port: 7878, language: '', overlay: { theme: 'dark' }, keys: { enabled: false, layout: 'auto', k4: ['A', 'S', 'K', 'L'], k7: ['A', 'S', 'D', 'SPACE', 'J', 'K', 'L'] } };
const frozen = JSON.stringify(current);
const body = m.buildKeysBody(current, Object.assign({}, defaults, { enabled: true, layout: '4k', k4: ['D', 'F', 'J', 'K'] }));
assert.strictEqual(JSON.stringify(current), frozen);
const noScratch = function (o) { const c = Object.assign({}, o); delete c.scratch; return c; };
assert.deepStrictEqual(body.keys, noScratch(Object.assign({}, defaults, { enabled: true, layout: '4k', k4: ['D', 'F', 'J', 'K'] })));
assert.strictEqual(body.port, 7878);
assert.deepStrictEqual(body.overlay, { theme: 'dark' });
assert.notStrictEqual(body.overlay, current.overlay);
assert.notStrictEqual(body.keys.k4, current.keys.k4);
assert.deepStrictEqual(m.buildKeysBody({ port: 1 }, defaults).keys, noScratch(defaults));

const main = m.buildConfigBody(current, { quaver_dir: 'X', port: 9, language: 'en' }, []);
assert.deepStrictEqual(main.keys, current.keys);

assert.strictEqual(m.keysFieldFor('keys.layout'), 'layout');
assert.strictEqual(m.keysFieldFor('keys.k4'), 'k4');
assert.strictEqual(m.keysFieldFor('keys.k7'), 'k7');
assert.strictEqual(m.keysFieldFor('keys.scratch'), 'scratch');
m.LISTS.forEach(function (name) { assert.strictEqual(m.keysFieldFor('keys.' + name), name); });
for (const other of ['keys.k0', 'keys.k11', 'keys.k04', 'keys.K4']) assert.strictEqual(m.keysFieldFor(other), '', other);
for (const other of ['keys', 'keys.enabled', 'port', '', undefined, '__proto__', 'keys.constructor']) assert.strictEqual(m.keysFieldFor(other), '', String(other));
assert.strictEqual(m.fieldFor('keys.k4'), '');
`

func TestSettingsKeyWhitelistMatchesTheBackend(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not on PATH")
	}
	out, err := exec.Command(node, "-e", "console.log(JSON.stringify(require('./dashboard/pages/settings-model.js').KEY_NAMES))").CombinedOutput()
	if err != nil {
		t.Fatalf("node: %v: %s", err, out)
	}
	var names []string
	if err := json.Unmarshal(out, &names); err != nil {
		t.Fatalf("node printed %s: %v", out, err)
	}
	if len(names) != 64 {
		t.Fatalf("expected 64 key names, got %d: %v", len(names), names)
	}
	for _, n := range names {
		if !keys.Valid(keys.Key(n)) {
			t.Errorf("%q is offered by the dashboard but the backend rejects it", n)
		}
	}
	for _, n := range []string{"A", "Z", "0", "9", "SPACE", "LEFT", "SHIFT_L", "CTRL_R", "ALT_L", ";", "'", "F1", "F12"} {
		found := false
		for _, have := range names {
			found = found || have == n
		}
		if !found {
			t.Errorf("%q is missing from the dashboard list", n)
		}
	}
}

func TestSettingsPageLoadsTheKeysCard(t *testing.T) {
	index := readWeb(t, "dashboard/index.html")
	card := strings.Index(index, `src="pages/settings-keys.js"`)
	page := strings.Index(index, `src="pages/settings.js"`)
	if card < 0 || card > page {
		t.Error("settings-keys.js must be loaded before settings.js")
	}
	src := readWeb(t, "dashboard/pages/settings-keys.js")
	for _, want := range []string{"/api/config", "/api/status", "keysFieldFor", "buildKeysBody", "aria-invalid", "role: 'alert'", "restart_required", "settings.keys.disclosure", "m.LISTS", "scratchSelect", "KEY_NAMES", "'select'"} {
		if !strings.Contains(src, want) {
			t.Errorf("settings-keys.js is missing %q", want)
		}
	}
	if !strings.Contains(readWeb(t, "dashboard/pages/settings.js"), "qscSettingsKeys") {
		t.Error("settings.js must build the keys card")
	}
}

func TestDocsStatePrivacyOfLaneKeys(t *testing.T) {
	statements := []string{
		"off by default", "only the lane keys you choose are read", "only while a map is being played",
		"nothing is stored or logged", "regardless of the focused window", "Windows only for now",
		"shorter than the polling interval",
	}
	for _, doc := range []string{"README.md", "docs/OVERLAYS.md", "docs/OBS.md"} {
		low := strings.ToLower(readDoc(t, doc))
		for _, want := range statements {
			if !strings.Contains(low, strings.ToLower(want)) {
				t.Errorf("%s must state %q", doc, want)
			}
		}
	}
	guide := readDoc(t, "docs/OVERLAYS.md")
	for _, want := range []string{"/overlays/keys/", "?keys=1", "keys.frame", "480 x 200", "560 x 200", "`counts`", "`peak`", "serves seven"} {
		if !strings.Contains(guide, want) {
			t.Errorf("docs/OVERLAYS.md must mention %q", want)
		}
	}
	if !strings.Contains(readDoc(t, "docs/OBS.md"), "/overlays/keys/") || !strings.Contains(readDoc(t, "README.md"), "/overlays/keys/") {
		t.Error("README.md and docs/OBS.md must list the keys overlay")
	}
}

func TestSettingsKeysModelEveryLayout(t *testing.T) {
	runNodeModel(t, "dashboard/pages/settings-model.js", settingsEveryLayoutScript)
}

const settingsEveryLayoutScript = `
const assert = require('assert');
const m = require(process.argv[2]);

assert.deepStrictEqual(m.LISTS, ['k1', 'k2', 'k3', 'k4', 'k5', 'k6', 'k7', 'k8', 'k9', 'k10']);
const defaults = m.keysFromConfig(undefined);
assert.deepStrictEqual(defaults.k1, ['SPACE']);
assert.deepStrictEqual(defaults.k10, ['A', 'S', 'D', 'F', 'V', 'B', 'H', 'J', 'K', 'L']);
for (const name of m.LISTS) {
  assert.strictEqual(new Set(defaults[name]).size, m.LANES[name], name);
  for (const bad of [[], defaults[name].slice(1), defaults[name].concat(['Z']), defaults[name].map(function () { return 'nope'; }), 'x', null]) {
    assert.deepStrictEqual(m.keysFromConfig({ [name]: bad })[name], defaults[name], name);
  }
  const edited = defaults[name].slice();
  edited[0] = 'F1';
  assert.strictEqual(m.keysFromConfig({ [name]: edited })[name][0], 'F1');
  assert.notStrictEqual(m.keysFromConfig({ [name]: edited })[name], edited);
}
assert.strictEqual(m.keysFromConfig({ scratch: 'ENTER' }).scratch, 'SHIFT_L');
assert.strictEqual(m.keysFromConfig({ scratch: 'CTRL_R' }).scratch, 'CTRL_R');
for (const l of m.LAYOUTS) assert.strictEqual(m.keysFromConfig({ layout: l }).layout, l);
for (const l of ['0k', '11k', '4K', '']) assert.strictEqual(m.keysFromConfig({ layout: l }).layout, 'auto', l);

const current = { port: 1, keys: { future: 7 } };
const body = m.buildKeysBody(current, Object.assign({}, defaults, { scratch: 'ALT_L' }));
assert.strictEqual(body.keys.future, 7);
assert.strictEqual(body.keys.scratch, 'ALT_L');
for (const name of m.LISTS) assert.deepStrictEqual(body.keys[name], defaults[name]);
// the default scratch key is never sent unless the config already holds one
assert.strictEqual('scratch' in m.buildKeysBody({ port: 1, keys: {} }, defaults).keys, false);
assert.strictEqual('scratch' in m.buildKeysBody({ port: 1 }, defaults).keys, false);
assert.strictEqual(m.buildKeysBody({ port: 1, keys: { scratch: 'SHIFT_L' } }, defaults).keys.scratch, 'SHIFT_L');
assert.strictEqual(m.buildKeysBody({ port: 1, keys: {} }, Object.assign({}, defaults, { scratch: 'CTRL_L' })).keys.scratch, 'CTRL_L');
`

func TestKeysModelEveryLaneCount(t *testing.T) {
	runNodeModel(t, "overlays/model.js", keysEveryLaneScript)
}

const keysEveryLaneScript = `
const assert = require('assert');
const m = require(process.argv[2]);

for (let n = 1; n <= 11; n++) {
  const down = Array.from({ length: n }, function (_, i) { return i % 2 === 0; });
  const v = m.keysView({ layout: 'x', lanes: n, down: down, presses: down.map(function (_, i) { return i; }), kps: 3, peak: 5 });
  assert.strictEqual(v.lanes, n);
  assert.strictEqual(v.scratch, false);
  assert.deepStrictEqual(v.down, down);
  assert.strictEqual(v.presses.length, n);
}
const s = m.keysView({ lanes: 8, scratch: true, down: [], presses: [] });
assert.strictEqual(s.scratch, true);
assert.strictEqual(m.keysView({ lanes: 8, scratch: 'yes' }).scratch, false);
assert.strictEqual(m.keysView({ lanes: 1, scratch: true }).scratch, false);
for (const bad of [0, -1, 12, 100, 4.5, NaN, Infinity, '7', null, [], {}]) assert.strictEqual(m.keysView({ lanes: bad }), null, String(bad));

assert.strictEqual(m.keysUnknown({ layout: 'unknown', lanes: 0 }), true);
for (const other of [null, undefined, {}, { lanes: 0 }, { layout: 'unknown' }, { layout: '4k', lanes: 4 }, { layout: 'unknown', lanes: 4 }]) assert.strictEqual(m.keysUnknown(other), false, JSON.stringify(other));

const params = function (q) { const p = new URLSearchParams(q); return function (k) { return p.get(k); }; };
const lanesOf = function (q) { const f = m.demoKeys(params(q)); return f ? [f.lanes, f.scratch] : null; };
assert.deepStrictEqual(lanesOf(''), [4, false]);
assert.deepStrictEqual(lanesOf('lanes=7'), [7, false]);
assert.deepStrictEqual(lanesOf('lanes=10'), [10, false]);
assert.deepStrictEqual(lanesOf('layout=9k'), [9, false]);
assert.deepStrictEqual(lanesOf('layout=10K'), [10, false]);
assert.deepStrictEqual(lanesOf('layout=7k&scratch=1'), [8, true]);
assert.deepStrictEqual(lanesOf('lanes=10&scratch=1'), [11, true]);
assert.deepStrictEqual(lanesOf('lanes=1'), [1, false]);
assert.deepStrictEqual(lanesOf('layout=5k&lanes=8'), [5, false]);
for (const bad of ['lanes=0', 'lanes=11', 'lanes=-2', 'lanes=x', 'lanes=3.5', 'layout=11k', 'layout=0k', 'layout=auto', 'layout=7', 'layout=1e1k', 'lanes=99999999999999999999']) {
  assert.deepStrictEqual(lanesOf(bad), [4, false], bad);
}
for (let n = 1; n <= 10; n++) {
  const f = m.demoKeys(params('lanes=' + n + '&scratch=1'));
  assert.ok(m.keysView(f), String(n));
  assert.strictEqual(f.down.length, f.lanes);
  assert.strictEqual(f.presses.length, f.lanes);
}
`
