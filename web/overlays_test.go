package web

import (
	"encoding/json"
	"io/fs"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var overlayPages = []string{"now-playing", "result", "history", "state", "alert", "keys", "curve"}

func mustAbs(t *testing.T, p string) string {
	t.Helper()
	abs, err := filepath.Abs(p)
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

func TestOverlayPagesLoadSharedScripts(t *testing.T) {
	for _, name := range overlayPages {
		data, err := fs.ReadFile(FS, "overlays/"+name+"/index.html")
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		src := string(data)
		if !regexp.MustCompile(`<title>[^<]+</title>`).MatchString(src) {
			t.Errorf("%s needs a title", name)
		}
		if strings.Contains(src, "aria-live") || strings.Contains(src, `role="status"`) {
			t.Errorf("%s must not use live regions", name)
		}
		last := -1
		for _, s := range []string{"/overlays/i18n.js", "/overlays/model.js", "/overlays/common.js"} {
			i := strings.Index(src, `src="`+s+`"`)
			if i < 0 || i < last {
				t.Errorf("%s must load %s after the previous shared script", name, s)
			}
			last = i
		}
		if !strings.Contains(src, "/overlays/overlay.css") {
			t.Errorf("%s must load overlay.css", name)
		}
	}
}

func TestOverlaySourcesAreSafe(t *testing.T) {
	ext := regexp.MustCompile(`(?i)https?://|//[a-z0-9.-]+.[a-z]{2,}`)
	err := fs.WalkDir(FS, "overlays", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, rerr := fs.ReadFile(FS, p)
		if rerr != nil {
			return rerr
		}
		src := string(data)
		for _, bad := range []string{"innerHTML", "outerHTML", "insertAdjacentHTML"} {
			if strings.Contains(src, bad) {
				t.Errorf("%s uses %s", p, bad)
			}
		}
		if ext.MatchString(src) {
			t.Errorf("%s references an external URL", p)
		}
		for _, r := range []rune{0x2014, 0x2013} {
			if strings.ContainsRune(src, r) {
				t.Errorf("%s contains a dash character %U", p, r)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestOverlayDemoBranchesNeverReachTheNetwork(t *testing.T) {
	for _, name := range overlayPages {
		data, err := fs.ReadFile(FS, "overlays/"+name+"/index.html")
		if err != nil {
			t.Fatal(err)
		}
		src := string(data)
		loc := regexp.MustCompile(`if \((?:params.demo|isDemo)\) {`).FindStringIndex(src)
		if loc == nil {
			t.Errorf("%s must branch on the demo flag", name)
			continue
		}
		open := loc[0]
		split := strings.Index(src[open:], "} else {")
		if split < 0 {
			t.Errorf("%s needs an else branch", name)
			continue
		}
		demo, live := src[open:open+split], src[open+split:]
		for _, bad := range []string{"qscConnect", "EventSource", "fetch("} {
			if strings.Contains(demo, bad) {
				t.Errorf("%s demo branch uses %s", name, bad)
			}
		}
		if !strings.Contains(live, "qscConnect") {
			t.Errorf("%s live branch must connect through qscConnect", name)
		}
	}
}

func TestOverlayHistoryFetchesBoundedList(t *testing.T) {
	data, err := fs.ReadFile(FS, "overlays/history/index.html")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	for _, want := range []string{"/api/history?limit=", "AbortController", "score.saved"} {
		if !strings.Contains(src, want) {
			t.Errorf("history overlay is missing %q", want)
		}
	}
}

func TestOverlayCSSDefinesSharedComponents(t *testing.T) {
	data, err := fs.ReadFile(FS, "overlays/overlay.css")
	if err != nil {
		t.Fatal(err)
	}
	css := string(data)
	for _, want := range []string{".chip", ".pill", ".grade", ".gold", ".green", ".blue", ".amber", ".red", ".muted", ".j-marv", ".j-perf", ".j-great", ".j-good", ".j-okay", ".j-miss", "prefers-reduced-motion", "overflow: hidden"} {
		if !strings.Contains(css, want) {
			t.Errorf("overlay.css is missing %q", want)
		}
	}
}

func TestOverlayHiddenBeatsFill(t *testing.T) {
	data, err := fs.ReadFile(FS, "overlays/overlay.css")
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`\.card\.fill\.hidden\s*\{\s*display:\s*none;`).Match(data) {
		t.Fatal("overlay.css needs .card.fill.hidden { display: none; } so an empty card stays hidden")
	}
}

func TestOverlayReconnectsDoNotReshowOrRestartTimers(t *testing.T) {
	result, _ := fs.ReadFile(FS, "overlays/result/index.html")
	if !strings.Contains(string(result), "d.last_score && gate.fresh(d.last_score)") {
		t.Error("result must render snapshot.last_score only when its id was not rendered yet")
	}
	np, _ := fs.ReadFile(FS, "overlays/now-playing/index.html")
	src := string(np)
	if strings.Contains(strings.SplitN(src[strings.Index(src, "function render()"):], "\n  }\n", 2)[0], "autohide.touch") {
		t.Error("now-playing render must not restart the autohide timer")
	}
	for _, want := range []string{"firstSnapshot", "'map.started': function (d) { map = d; fresh(); }", "'score.saved'"} {
		if !strings.Contains(src, want) {
			t.Errorf("now-playing is missing %q", want)
		}
	}
	history, _ := fs.ReadFile(FS, "overlays/history/index.html")
	if !strings.Contains(string(history), "es.onopen") {
		t.Error("history must retry a failed first load when the stream connects")
	}
	if strings.Contains(string(history), "HEIGHT - 10") {
		t.Error("history must use the model's axis positions")
	}
}

func overlayCatalogs(t *testing.T) map[string]map[string]string {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not on PATH")
	}
	out, err := exec.Command(node, "-e", `process.stdout.write(JSON.stringify(require(process.argv[1])))`, mustAbs(t, "overlays/i18n.js")).CombinedOutput()
	if err != nil {
		t.Fatalf("i18n.js must export its catalogs in Node: %v: %s", err, out)
	}
	var cats map[string]map[string]string
	if err := json.Unmarshal(out, &cats); err != nil {
		t.Fatal(err)
	}
	return cats
}

func TestOverlayCatalogs(t *testing.T) {
	cats := overlayCatalogs(t)
	en := cats["en"]
	if len(en) == 0 {
		t.Fatal("no english catalog")
	}
	for _, lang := range []string{"fr", "de"} {
		c := cats[lang]
		if len(c) != len(en) {
			t.Errorf("%s has %d keys, en has %d", lang, len(c), len(en))
		}
		for k := range en {
			if _, ok := c[k]; !ok {
				t.Errorf("%s is missing %q", lang, k)
			}
		}
	}
	for lang, c := range cats {
		for k, v := range c {
			if strings.TrimSpace(v) == "" {
				t.Errorf("%s: empty value for %q", lang, k)
			}
		}
	}
}

func TestOverlayKeysUsedExist(t *testing.T) {
	en := overlayCatalogs(t)["en"]
	re := regexp.MustCompile(`qscT\('([a-z0-9_.]+)'\)`)
	for _, name := range overlayPages {
		data, _ := fs.ReadFile(FS, "overlays/"+name+"/index.html")
		for _, m := range re.FindAllStringSubmatch(string(data), -1) {
			if _, ok := en[m[1]]; !ok {
				t.Errorf("%s uses unknown key %q", name, m[1])
			}
		}
	}
	for _, key := range []string{"result.marv", "result.perf", "result.great", "result.good", "result.okay", "result.miss", "badge.pb", "badge.fc", "badge.failed", "badge.paused", "badge.autoplay", "state.initialization", "state.menu", "state.select", "state.loading", "state.gameplay", "state.results", "state.closed", "state.unknown"} {
		if _, ok := en[key]; !ok {
			t.Errorf("missing %q", key)
		}
	}
}

func TestOverlayModel(t *testing.T) {
	runNodeModel(t, "overlays/model.js", overlayModelScript)
}

const overlayModelScript = `
const assert = require('assert');
const m = require(process.argv[2]);
const nan = NaN;

assert.strictEqual(m.gradeClass('X'), 'gold');
assert.strictEqual(m.gradeClass('ss'), 'gold');
assert.strictEqual(m.gradeClass('S'), 'gold');
assert.strictEqual(m.gradeClass('A'), 'green');
assert.strictEqual(m.gradeClass('B'), 'blue');
assert.strictEqual(m.gradeClass('C'), 'amber');
assert.strictEqual(m.gradeClass('D'), 'amber');
assert.strictEqual(m.gradeClass('F'), 'red');
for (const bad of ['Z', '', null, undefined, 5, {}, '__proto__', 'constructor']) assert.strictEqual(m.gradeClass(bad), 'muted');
assert.strictEqual(m.gradeText(null), '-');
assert.strictEqual(m.gradeText(''), '-');
assert.strictEqual(m.gradeText('SS'), 'SS');
assert.ok(m.gradeText('x'.repeat(500)).length <= 3);

assert.strictEqual(m.formatAccuracy(98.7654), '98.77%');
assert.strictEqual(m.formatAccuracy(nan), '0.00%');
assert.strictEqual(m.formatAccuracy(null), '0.00%');
assert.strictEqual(m.formatAccuracy(Infinity), '0.00%');
assert.strictEqual(m.formatAccuracy(1e30), '100.00%');
assert.strictEqual(m.formatAccuracy(-5), '0.00%');

assert.strictEqual(m.formatRate(1), '');
assert.strictEqual(m.formatRate(1.15), '1.15x');
assert.strictEqual(m.formatRate(0.9), '0.9x');
for (const bad of [0, -1, nan, null, undefined, 'x', Infinity]) assert.strictEqual(m.formatRate(bad), '');

assert.strictEqual(m.formatStars(18.456), '18.46');
assert.strictEqual(m.formatStars(nan), '');
assert.strictEqual(m.formatStars(Infinity), '');
assert.strictEqual(m.formatStars(-1), '');
assert.strictEqual(m.formatStars(1e30), '');
assert.strictEqual(m.formatStars(0), '0.00');
assert.strictEqual(m.formatRate(1e30), '');
assert.strictEqual(m.formatRate(11), '');
assert.strictEqual(m.formatRate(10), '10x');

const mods = m.modList(Array.from({ length: 10000 }, (_, i) => 'M' + i));
assert.ok(mods.shown.length <= 6);
assert.strictEqual(mods.shown.length + mods.extra, 10000);
assert.deepStrictEqual(m.modList(null), { shown: [], extra: 0 });
assert.deepStrictEqual(m.modList('x'), { shown: [], extra: 0 });
assert.ok(m.modList(['y'.repeat(1000)]).shown[0].length <= 12);
assert.deepStrictEqual(m.modList([null, 3, '<b>']).shown, ['null', '3', '<b>']);

const j = m.judgments({ marv: 5, perf: 4.9, great: -1, good: nan, okay: '7', miss: 1e30 });
assert.deepStrictEqual(j.map(x => x.key), ['marv', 'perf', 'great', 'good', 'okay', 'miss']);
assert.deepStrictEqual(j.map(x => x.value), [5, 4, 0, 0, 7, 999999]);
assert.strictEqual(m.judgments(null).length, 6);
assert.ok(m.judgments(undefined).every(x => x.value === 0));

assert.strictEqual(m.parseCount(undefined), 20);
assert.strictEqual(m.parseCount('abc'), 20);
assert.strictEqual(m.parseCount('1'), 5);
assert.strictEqual(m.parseCount('4'), 5);
assert.strictEqual(m.parseCount('5'), 5);
assert.strictEqual(m.parseCount('50'), 50);
assert.strictEqual(m.parseCount('51'), 50);
assert.strictEqual(m.parseCount('1e9'), 50);
assert.strictEqual(m.parseCount('12.7'), 12);
assert.strictEqual(m.parseCount('-3'), 5);

assert.strictEqual(m.parseScale(undefined), 1);
assert.strictEqual(m.parseScale('1.5'), 1.5);
assert.strictEqual(m.parseScale('0'), 1);
assert.strictEqual(m.parseScale('99'), 3);
assert.strictEqual(m.parseScale('0.1'), 0.5);
assert.strictEqual(m.parseScale('nope'), 1);

assert.strictEqual(m.parseAutohide(undefined), 0);
assert.strictEqual(m.parseAutohide('8'), 8);
assert.strictEqual(m.parseAutohide('-4'), 0);
assert.strictEqual(m.parseAutohide('x'), 0);
assert.strictEqual(m.parseAutohide('999999'), 3600);

assert.strictEqual(m.parseLang('fr', 'en-US'), 'fr');
assert.strictEqual(m.parseLang('xx', 'de-DE'), 'de');
assert.strictEqual(m.parseLang(null, 'fr-CA'), 'fr');
assert.strictEqual(m.parseLang(null, 'ja'), 'en');
assert.strictEqual(m.parseLang(undefined, undefined), 'en');

assert.deepStrictEqual(m.parseHide(undefined), []);
assert.deepStrictEqual(m.parseHide('closed,unknown,menu'), ['closed', 'unknown', 'menu']);
assert.deepStrictEqual(m.parseHide(' Menu , bogus,,menu'), ['menu']);
assert.strictEqual(m.isHiddenState('menu', ['menu']), true);
assert.strictEqual(m.isHiddenState('gameplay', ['menu']), false);
assert.strictEqual(m.isHiddenState('weird', ['unknown']), true);
assert.strictEqual(m.isHiddenState('weird', []), false);

for (const s of ['initialization', 'menu', 'select', 'loading', 'gameplay', 'results', 'closed', 'unknown']) assert.strictEqual(m.stateKey(s), 'state.' + s);
for (const bad of ['', null, undefined, 'x', '__proto__', 5]) assert.strictEqual(m.stateKey(bad), 'state.unknown');
assert.strictEqual(m.stateTone('gameplay'), 'green');
assert.strictEqual(m.stateTone('closed'), 'muted');
assert.strictEqual(m.stateTone('bogus'), 'muted');
assert.strictEqual(m.stateTone('loading'), 'amber');

const res = m.resultView({
  score: { grade: 'S', accuracy: 97.256, max_combo: 812, rate: 1.15, mods: ['Mirror'], judgments: { marv: 1, perf: 2, great: 3, good: 4, okay: 5, miss: 6 } },
  map: { title: 'A <b> & "q" é世', artist: 'Z', difficulty_name: 'Hard', difficulty: 18.5 },
  is_pb: true, full_combo: false, failed: false, paused: true, autoplay: false,
});
assert.strictEqual(res.grade, 'S');
assert.strictEqual(res.gradeClass, 'gold');
assert.strictEqual(res.accuracy, '97.26%');
assert.strictEqual(res.combo, 812);
assert.strictEqual(res.title, 'A <b> & "q" é世');
assert.strictEqual(res.stars, '18.50');
assert.strictEqual(m.resultView({ map: { difficulty: nan } }).stars, '');
assert.strictEqual(res.rate, '1.15x');
assert.deepStrictEqual(res.mods.shown, ['Mirror']);
assert.deepStrictEqual(res.badges, ['pb', 'paused']);
assert.strictEqual(res.judgments.length, 6);
assert.strictEqual(m.resultView(null), null);
assert.strictEqual(m.resultView('x'), null);
assert.strictEqual(m.resultView(7), null);
const empty = m.resultView({});
assert.strictEqual(empty.grade, '-');
assert.strictEqual(empty.gradeClass, 'muted');
assert.strictEqual(empty.accuracy, '0.00%');
assert.deepStrictEqual(empty.badges, []);
assert.ok(m.resultView({ map: { title: 'x'.repeat(100000) } }).title.length <= 200);
assert.deepStrictEqual(m.resultView({ failed: true, full_combo: true, autoplay: true, is_pb: true, paused: true }).badges, ['pb', 'fc', 'failed', 'paused', 'autoplay']);

const plays = [
  { id: 3, accuracy: 99.1, grade: 'S' },
  { id: 2, accuracy: 88.2, grade: 'A' },
  { id: 1, accuracy: 97.5, grade: 'SS' },
];
const c = m.curve(plays, 200, 100);
assert.strictEqual(c.points.length, 3);
assert.deepStrictEqual(c.points.map(p => p.accuracy), [97.5, 88.2, 99.1]);
assert.ok(c.points[0].x < c.points[1].x && c.points[1].x < c.points[2].x);
assert.strictEqual(c.points[2].last, true);
assert.strictEqual(c.points[0].last, false);
assert.strictEqual(c.yMin, 75);
assert.strictEqual(c.yMax, 100);
assert.ok(c.yTop > 0 && c.yBottom < 100 && c.yTop < c.yBottom);
assert.ok(c.points.every(p => p.y >= c.yTop && p.y <= c.yBottom));
assert.ok(c.points[2].y < c.points[1].y);
assert.ok(Math.abs(c.avg - (99.1 + 88.2 + 97.5) / 3) < 1e-9);
assert.ok(c.avgY > c.points[2].y && c.avgY < c.points[1].y);
assert.strictEqual(m.curve([{ accuracy: 51.5 }, { accuracy: 99 }], 200, 100).yMin, 50);
assert.strictEqual(m.curve([{ accuracy: 50 }], 200, 100).yMin, 50);
assert.strictEqual(m.curve([{ accuracy: 0 }], 200, 100).yMin, 0);
assert.strictEqual(m.curve([{ accuracy: 80 }], 200, 100).yMin, 75);
assert.strictEqual(m.curve([], 200, 100), null);
assert.strictEqual(m.curve(null, 200, 100), null);
assert.strictEqual(m.curve([{ accuracy: nan }, null, 'x', { accuracy: 'a' }], 200, 100), null);
const one = m.curve([{ accuracy: 90, grade: 'B' }], 200, 100);
assert.strictEqual(one.points.length, 1);
assert.ok(isFinite(one.points[0].x) && isFinite(one.points[0].y));
const wild = m.curve([{ accuracy: 1e30 }, { accuracy: -50 }, { accuracy: 90 }, { accuracy: nan }], 200, 100);
assert.strictEqual(wild.points.length, 3);
assert.ok(wild.points.every(p => isFinite(p.x) && isFinite(p.y) && p.y >= 0 && p.y <= 100));
assert.strictEqual(m.curve(Array.from({ length: 5000 }, () => ({ accuracy: 90 })), 200, 100).points.length, 50);

assert.strictEqual(m.parseAutohide(null, 8), 8);
assert.strictEqual(m.parseAutohide(undefined, 8), 8);
assert.strictEqual(m.parseAutohide('', 8), 8);
assert.strictEqual(m.parseAutohide('3', 8), 3);
assert.strictEqual(m.parseAutohide('0', 8), 0);
assert.strictEqual(m.parseAutohide(null), 0);

const av = m.alertView({ score: { accuracy: 98.765 }, map: { title: 'Song <b>' }, is_pb: true, full_combo: true });
assert.deepStrictEqual(av.kinds, ['pb', 'fc']);
assert.strictEqual(av.title, 'Song <b>');
assert.strictEqual(av.accuracy, '98.77%');
assert.deepStrictEqual(m.alertView({ is_pb: true, map: {}, score: {} }).kinds, ['pb']);
assert.deepStrictEqual(m.alertView({ full_combo: true, map: {}, score: {} }).kinds, ['fc']);
for (const flag of ['failed', 'paused', 'autoplay']) assert.strictEqual(m.alertView({ is_pb: true, full_combo: true, [flag]: true }), null, flag);
assert.strictEqual(m.alertView({ is_pb: false, full_combo: false }), null);
assert.strictEqual(m.alertView({ is_pb: 'true' }), null);
for (const bad of [null, undefined, 'x', 7]) assert.strictEqual(m.alertView(bad), null);
assert.ok(m.alertView({ is_pb: true, map: { title: 'x'.repeat(100000) } }).title.length <= 200);

const bc = m.curve(plays, 200, 100);
const bl = m.bestLine(bc);
assert.strictEqual(bl.text, '99.10%');
assert.strictEqual(bl.x1, bc.left);
assert.strictEqual(bl.x2, bc.right);
assert.strictEqual(bl.y, bc.points[2].y);
assert.strictEqual(m.bestLine(null), null);
assert.strictEqual(m.bestLine(m.curve([{ accuracy: 90 }], 200, 100)).text, '90.00%');

function fakeClock() {
  let t = 0, id = 0; const timers = new Map();
  return {
    setTimeout(fn, ms) { id++; timers.set(id, { fn, at: t + ms }); return id; },
    clearTimeout(i) { timers.delete(i); },
    advance(ms) {
      t += ms;
      for (const [i, tm] of [...timers]) if (tm.at <= t) { timers.delete(i); tm.fn(); }
    },
    pending() { return timers.size; },
  };
}
{
  const clock = fakeClock(); let hidden = 0;
  const a = m.createAutohide(5, clock, () => { hidden++; });
  a.touch(); clock.advance(4999); assert.strictEqual(hidden, 0);
  a.touch(); clock.advance(4999); assert.strictEqual(hidden, 0);
  clock.advance(1); assert.strictEqual(hidden, 1);
  assert.strictEqual(clock.pending(), 0);
  a.touch(); clock.advance(5000); assert.strictEqual(hidden, 2);
  a.touch(); a.stop(); clock.advance(10000); assert.strictEqual(hidden, 2);
}
{
  const clock = fakeClock(); let hidden = 0;
  const off = m.createAutohide(0, clock, () => { hidden++; });
  off.touch(); assert.strictEqual(clock.pending(), 0); clock.advance(1e6); assert.strictEqual(hidden, 0);
}
`

func TestEventStreamReconnects(t *testing.T) {
	runNodeModel(t, "overlays/model.js", `
const assert = require('assert');
const m = require(process.argv[2]);
const made = [];
function fake(url) {
  const es = { url, readyState: 1, closed: false, handlers: {}, addEventListener(n, f) { this.handlers[n] = f; }, close() { this.closed = true; this.readyState = 2; } };
  made.push(es);
  return es;
}
const timers = { pending: [], setTimeout(fn, ms) { this.pending.push({ fn, ms }); return this.pending.length; }, clearTimeout() { this.pending = []; } };
const got = [];
const h = m.createConnection(fake, '/events?keys=1', { 'score.saved': (d) => got.push(d) }, timers);
assert.strictEqual(made.length, 1);
assert.strictEqual(made[0].url, '/events?keys=1');
made[0].handlers['score.saved']({ data: JSON.stringify({ data: { id: 1 } }) });
assert.deepStrictEqual(got, [{ id: 1 }]);
let opened = 0; h.onopen = () => opened++;
made[0].onopen(); assert.strictEqual(opened, 1);
made[0].readyState = 0; made[0].onerror();
assert.strictEqual(timers.pending.length, 0, 'a stream the browser retries itself is left alone');
for (const want of [5000, 10000, 20000, 30000, 30000]) {
  const cur = made[made.length - 1];
  cur.readyState = 2; cur.onerror(); cur.onerror();
  assert.strictEqual(timers.pending.length, 1, 'one timer only');
  assert.strictEqual(timers.pending[0].ms, want);
  const t = timers.pending.shift(); timers.pending = []; t.fn();
}
assert.strictEqual(made.length, 6);
assert.ok(made.slice(0, 5).every((e) => e.closed), 'old streams are closed');
made[5].onopen(); assert.strictEqual(opened, 2, 'onopen is forwarded after a reconnect');
made[5].readyState = 2; made[5].onerror();
assert.strictEqual(timers.pending[0].ms, 5000, 'delay starts over after a successful open');
h.close();
assert.strictEqual(timers.pending.length, 0);
assert.ok(made[5].closed);
`)
}

func TestCommonJSConnectsThroughTheReconnectingModel(t *testing.T) {
	src := readWeb(t, "overlays/common.js")
	for _, want := range []string{"model.createConnection(", "'/events?keys=1'", "'/events'"} {
		if !strings.Contains(src, want) {
			t.Errorf("common.js lacks %q", want)
		}
	}
}
