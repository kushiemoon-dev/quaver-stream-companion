package web

import (
	"regexp"
	"strings"
	"testing"
)

func TestNowPlayingViewGuardsMissingFields(t *testing.T) {
	runNodeModel(t, "overlays/model.js", `
const assert = require('assert');
const m = require(process.argv[2]);
const dot = ' ' + String.fromCharCode(183) + ' ';
const star = String.fromCharCode(9733);

assert.deepStrictEqual(m.nowPlayingView({ title: 'T', artist: 'A', difficulty_name: 'Hard', difficulty: 18.5, keys: 4 }), { title: 'T', artist: 'A', line: 'Hard' + dot + '18.50' + star + dot + '4K' });
assert.strictEqual(m.nowPlayingView({ title: 'T', artist: 'A', difficulty_name: 'Hard', difficulty: null, keys: 7 }).line, 'Hard' + dot + '7K');
assert.strictEqual(m.nowPlayingView({ title: 'T', artist: 'A', difficulty_name: 'Hard', keys: 7 }).line, 'Hard' + dot + '7K');
assert.strictEqual(m.nowPlayingView({ difficulty: 'abc' }).line, '');
assert.strictEqual(m.nowPlayingView({ title: 'T' }).artist, '');
assert.deepStrictEqual(m.nowPlayingView(null), { title: '', artist: '', line: '' });
assert.strictEqual(m.nowPlayingView({ title: 'x'.repeat(500) }).title.length, 200);
`)
}

func TestNowPlayingPageHasNoUnguardedFormatting(t *testing.T) {
	src := readWeb(t, "overlays/now-playing/index.html")
	if strings.Contains(src, "toFixed") {
		t.Error("now-playing must format stars through the guarded model")
	}
	if !strings.Contains(src, "qscModel.nowPlayingView(") {
		t.Error("now-playing must render through qscModel.nowPlayingView")
	}
}

func TestOverlayCSSBoundsLongText(t *testing.T) {
	css := readWeb(t, "overlays/overlay.css")
	rule := func(sel string) string {
		m := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(sel) + ` \{([^}]*)\}`).FindStringSubmatch(css)
		if m == nil {
			t.Fatalf("overlay.css has no %s rule", sel)
		}
		return m[1]
	}
	card := rule(".card")
	for _, want := range []string{"box-sizing: border-box", "max-width: min(520px, 100%)"} {
		if !strings.Contains(card, want) {
			t.Errorf(".card must contain %q", want)
		}
	}
	ellipsis := []string{"overflow: hidden", "text-overflow: ellipsis", "white-space: nowrap"}
	for _, sel := range []string{".sub", ".big", ".title"} {
		body := rule(sel)
		for _, want := range ellipsis {
			if !strings.Contains(body, want) {
				t.Errorf("%s must contain %q", sel, want)
			}
		}
	}
}

func TestOverlayPagesDeclareAnEmptyIcon(t *testing.T) {
	for _, name := range overlayPages {
		src := readWeb(t, "overlays/"+name+"/index.html")
		if !strings.Contains(src, `<link rel="icon" href="data:,">`) {
			t.Errorf("%s must declare an empty icon so the browser does not request /favicon.ico", name)
		}
	}
}

func cssRule(t *testing.T, css, sel string) string {
	t.Helper()
	m := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(sel) + ` \{([^}]*)\}`).FindStringSubmatch(css)
	if m == nil {
		t.Fatalf("style.css has no %s rule", sel)
	}
	return m[1]
}

func TestObsPageFitsSmallScreensAndKeepsHeadingOrder(t *testing.T) {
	js := readWeb(t, "dashboard/pages/obs.js")
	if strings.Contains(js, "h('h3', null, t(titleKey))") || strings.Contains(js, "h('h4'") {
		t.Error("obs steps must be h2 and troubleshooting entries h3, no h4 after an h2")
	}
	for _, want := range []string{"h('h2', null, t(titleKey))", "h('h3', null, t(k[0]))", "tabindex: '0'", "obs.table_label"} {
		if !strings.Contains(js, want) {
			t.Errorf("obs.js is missing %q", want)
		}
	}
	css := readWeb(t, "dashboard/style.css")
	step := cssRule(t, css, ".obs-step")
	for _, want := range []string{"min-width: 0", "overflow-wrap: anywhere"} {
		if !strings.Contains(step, want) {
			t.Errorf(".obs-step must contain %q", want)
		}
	}
	if !strings.Contains(cssRule(t, css, "a.btn"), "display: inline-flex") {
		t.Error("a.btn must be inline-flex so its min-height applies")
	}
	for _, sel := range []string{".theme-input", ".check input"} {
		body := cssRule(t, css, sel)
		if !strings.Contains(body, "width: 24px") || !strings.Contains(body, "height: 24px") {
			t.Errorf("%s needs a 24px hit area", sel)
		}
	}
}

const apiHarness = `
const fs = require('fs');
const vm = require('vm');
const assert = require('assert');
function load(file, extra) {
  const win = Object.assign({ qscI18n: { t: (k, p) => k + (p ? JSON.stringify(p) : '') } }, extra);
  const ctx = vm.createContext(Object.assign({ window: win, AbortController, setTimeout, clearTimeout, setInterval, clearInterval, Date }, win.ctx));
  ctx.window.document = ctx.document;
  vm.runInContext(fs.readFileSync(file, 'utf8'), ctx);
  return win;
}
`

func TestAPIMapsFailuresToTranslatedMessages(t *testing.T) {
	runNodeModel(t, "dashboard/api.js", apiHarness+`
(async () => {
  const reports = [];
  let reply;
  const fetchStub = async () => { if (reply instanceof Error) throw reply; return reply; };
  const res = (status, body) => ({ ok: status < 400, status, json: async () => body });
  const win = { qscI18n: { t: (k, p) => k + (p ? JSON.stringify(p) : '') }, qscOffline: { report: (ok) => reports.push(ok) } };
  const ctx = vm.createContext({ window: win, fetch: fetchStub, AbortController, setTimeout, clearTimeout, Date });
  vm.runInContext(fs.readFileSync(process.argv[2], 'utf8'), ctx);
  const run = (r) => { reply = r; return win.qscApi.get('/api/x'); };

  await assert.rejects(run(new TypeError('Failed to fetch')), (e) => e.network === true && e.message === 'error.network');
  assert.deepStrictEqual(reports, [false]);

  await assert.rejects(run(Object.assign(new Error('aborted'), { name: 'AbortError' })), (e) => e.name === 'AbortError');
  assert.deepStrictEqual(reports, [false], 'an abort is the caller own timeout, not a report');

  await assert.rejects(run(res(400, { error: 'must be between 1 and 65535', field: 'port' })), (e) => e.message === 'error.port_range' && e.field === 'port' && e.status === 400);
  await assert.rejects(run(res(400, { error: 'must list exactly 4 keys', field: 'keys.k4' })), (e) => e.message === 'error.keys_count{"n":"4"}');
  await assert.rejects(run(res(503, { error: 'Quaver is writing the database, try again' })), (e) => e.message === 'error.busy');
  await assert.rejects(run(res(500, { error: 'open C:/x: access denied' })), (e) => e.message === 'error.server{"status":500}');
  await assert.rejects(run(res(502, null)), (e) => e.message === 'error.server{"status":502}');
  assert.deepStrictEqual(reports, [false, true, true, true, true, true], 'any HTTP answer means the companion is reachable');
  assert.deepStrictEqual(await run(res(200, { ok: 1 })), { ok: 1 });
  console.log('ok');
})().catch((e) => { console.error(e); process.exit(1); });
`)
}

func TestOfflineStateReportsAndClears(t *testing.T) {
	runNodeModel(t, "dashboard/offline.js", apiHarness+`
const banner = { textContent: '' };
const win = load(process.argv[2], { ctx: { document: { getElementById: (id) => (id === 'offline' ? banner : null), hidden: false } } });
const seen = [];
const off = win.qscOffline.onChange((v) => seen.push(v));
assert.strictEqual(win.qscOffline.isOffline(), false);
win.qscOffline.report(false);
assert.strictEqual(banner.textContent, 'offline.banner');
assert.strictEqual(win.qscOffline.isOffline(), true);
win.qscOffline.report(false);
win.qscOffline.report(true);
assert.strictEqual(banner.textContent, '', 'the banner clears on reconnect');
assert.deepStrictEqual(seen, [true, false], 'listeners hear changes only');
off();
win.qscOffline.report(false);
assert.deepStrictEqual(seen, [true, false], 'an unsubscribed listener hears nothing');
`)
}

func TestDashboardChromeHasSkipLinkBannerAndRedirect(t *testing.T) {
	index := readWeb(t, "dashboard/index.html")
	body := index[strings.Index(index, "<body>"):]
	skip := strings.Index(body, `id="skip"`)
	if skip < 0 || skip > strings.Index(body, `<aside`) {
		t.Error("the skip link must be the first thing in the body")
	}
	for _, want := range []string{`href="#main"`, `id="offline"`, `role="status"`, `src="offline.js"`} {
		if !strings.Contains(body, want) {
			t.Errorf("index.html is missing %q", want)
		}
	}
	if strings.Index(index, `src="api.js"`) > strings.Index(index, `src="offline.js"`) || strings.Index(index, `src="offline.js"`) > strings.Index(index, `src="app.js"`) {
		t.Error("offline.js must load after api.js and before app.js")
	}
	app := readWeb(t, "dashboard/app.js")
	for _, want := range []string{"history.replaceState", "skip.link", "qscOffline"} {
		if !strings.Contains(app, want) {
			t.Errorf("app.js is missing %q", want)
		}
	}
	css := readWeb(t, "dashboard/style.css")
	for _, want := range []string{".skip-link:focus", ".skip-link", ".banner-offline", ".is-stale"} {
		if !strings.Contains(css, want) {
			t.Errorf("style.css is missing %q", want)
		}
	}
	status := readWeb(t, "dashboard/pages/status.js")
	for _, want := range []string{"qscOffline.onChange", "status.stale", "check.detail.other"} {
		if !strings.Contains(status, want) {
			t.Errorf("status.js is missing %q", want)
		}
	}
}

func TestErrorAndOfflineStringsExist(t *testing.T) {
	for _, lang := range []string{"en", "fr", "de"} {
		c := loadCatalog(t, lang)
		for _, k := range []string{"error.network", "error.server", "error.busy", "error.port_range", "error.dir_unc", "error.dir_not_quaver", "error.language", "error.theme", "error.accent", "error.keys_count", "error.keys_entry", "error.no_timeline", "error.too_large", "offline.banner", "status.stale", "skip.link", "check.detail.busy", "check.detail.other"} {
			if c[k] == "" {
				t.Errorf("%s.json is missing %q", lang, k)
			}
		}
	}
}

func TestOverlayNumbersFollowTheLanguage(t *testing.T) {
	runNodeModel(t, "overlays/model.js", `
const assert = require('assert');
const m = require(process.argv[2]);
const nb = String.fromCharCode(160);
const star = String.fromCharCode(9733);
assert.strictEqual(m.formatAccuracy(97.4231), '97.42%');
assert.strictEqual(m.formatAccuracy(97.4231, 'en'), '97.42%');
assert.strictEqual(m.formatAccuracy(97.4231, 'fr'), '97,42' + nb + '%');
assert.strictEqual(m.formatAccuracy(97.4231, 'de'), '97,42' + nb + '%');
assert.strictEqual(m.formatAccuracy(100, 'fr'), '100,00' + nb + '%');
assert.strictEqual(m.formatAccuracy('x', 'fr'), '0,00' + nb + '%');
assert.strictEqual(m.formatAccuracy(97.4231, 'xx'), '97.42%');
assert.strictEqual(m.formatStars(18.5, 'fr'), '18,50');
assert.strictEqual(m.formatStars(18.5, 'de'), '18,50');
assert.strictEqual(m.formatStars(18.5, 'en'), '18.50');
assert.strictEqual(m.formatStars(null, 'fr'), '');
assert.strictEqual(m.formatRate(1.15, 'fr'), '1,15x');
assert.strictEqual(m.formatRate(1.15), '1.15x');
const result = { score: { accuracy: 97.4231, rate: 1.5 }, map: { difficulty: 18.5 } };
assert.strictEqual(m.resultView(result, 'fr').accuracy, '97,42' + nb + '%');
assert.strictEqual(m.resultView(result, 'fr').stars, '18,50');
assert.strictEqual(m.resultView(result, 'de').rate, '1,5x');
assert.strictEqual(m.resultView(result).accuracy, '97.42%');
assert.strictEqual(m.alertView({ is_pb: true, score: { accuracy: 98.4 }, map: {} }, 'de').accuracy, '98,40' + nb + '%');
assert.ok(m.nowPlayingView({ difficulty: 18.5, difficulty_name: 'H' }, 'fr').line.indexOf('18,50' + star) > 0);
const tl = m.timelineView({ points: [99, 98.5, 97.4231], total: 3 }, 480, 150, 'fr');
assert.strictEqual(tl.final, '97,42' + nb + '%');
const c = m.curve([{ accuracy: 96.5 }, { accuracy: 97.4231 }], 480, 150);
assert.strictEqual(m.lastLabel(c, 'fr').text, '96,50' + nb + '%');
assert.strictEqual(m.lastLabel(c).text, '96.50%');
assert.strictEqual(m.bestLine(c, 'de').text, '97,42' + nb + '%');
`)
}

func TestOverlayPagesPassTheLanguage(t *testing.T) {
	for page, want := range map[string]string{
		"result":      "qscModel.resultView(result, params.lang)",
		"alert":       "qscModel.alertView(result, params.lang)",
		"now-playing": "qscModel.nowPlayingView(map, params.lang)",
		"curve":       "qscModel.timelineView(timeline, WIDTH, HEIGHT, params.lang)",
		"history":     "qscModel.bestLine(c, params.lang)",
	} {
		if !strings.Contains(readWeb(t, "overlays/"+page+"/index.html"), want) {
			t.Errorf("%s must call %s", page, want)
		}
	}
}

func TestDashboardNumbersFollowTheLanguage(t *testing.T) {
	runNodeModel(t, "dashboard/pages/history-model.js", `
const assert = require('assert');
const m = require(process.argv[2]);
const nb = String.fromCharCode(160);
assert.strictEqual(m.formatAccuracy(97.4231), '97.42%');
assert.strictEqual(m.formatAccuracy(97.4231, 'fr'), '97,42' + nb + '%');
assert.strictEqual(m.formatAccuracy(97.4231, 'de'), '97,42' + nb + '%');
assert.strictEqual(m.formatStars(18.5), '18.5');
assert.strictEqual(m.formatStars(18.5, 'de'), '18,5');
assert.strictEqual(m.formatRate(1.15, 'fr'), '1,15x');
assert.strictEqual(m.formatRate(1.15), '1.15x');
assert.strictEqual(m.formatDecimal(18.5, 2, 'fr'), '18,50');
`)
	status := readWeb(t, "dashboard/pages/status.js")
	if strings.Contains(status, "toFixed") {
		t.Error("status.js must format numbers through qscHistoryModel")
	}
}

func TestGermanCatalogUsesDuAndNoCatalogEmbedsAnEnglishReadmeTitle(t *testing.T) {
	sie := regexp.MustCompile(`\b(Sie|Ihr|Ihre|Ihren|Ihrem|Ihrer|Ihnen|Ihres)\b`)
	for k, v := range loadCatalog(t, "de") {
		if sie.MatchString(v) {
			t.Errorf("de.json %s uses the formal register: %q", k, v)
		}
	}
	for _, lang := range []string{"en", "fr", "de"} {
		if strings.Contains(loadCatalog(t, lang)["obs.step1.body"], "Run at startup") {
			t.Errorf("%s obs.step1.body must not embed an English README section name", lang)
		}
	}
	if loadCatalog(t, "de")["settings.keys.layout.auto"] == "Auto" || loadCatalog(t, "fr")["settings.keys.layout.auto"] == "Auto" {
		t.Error("the automatic layout option must be translated in fr and de")
	}
}

func TestReadmeStatesTheDashboardIsDark(t *testing.T) {
	if !strings.Contains(readDoc(t, "README.md"), "The dashboard uses a dark theme") {
		t.Error("the README Dashboard section must say the dashboard uses a dark theme")
	}
}

func TestFallbackWidthPrecedesMinWidthAndWordingIsConsistent(t *testing.T) {
	css := readWeb(t, "overlays/overlay.css")
	if !strings.Contains(css, "max-width: 520px;\n  max-width: min(520px, 100%);") {
		t.Error(".card needs a plain max-width line before the min() one")
	}
	if got := loadCatalog(t, "de")["error.theme"]; got != "Unbekanntes Design." {
		t.Errorf("de error.theme = %q", got)
	}
	if got := overlayCatalogs(t)["fr"]["result.miss"]; got != loadCatalog(t, "fr")["judgment.miss"] {
		t.Errorf("fr overlay miss label %q differs from the dashboard", got)
	}
}

func TestNumberFormattingFallsBackWithoutIntl(t *testing.T) {
	for _, file := range []string{"overlays/model.js", "dashboard/pages/history-model.js"} {
		src, call := "formatStars(18.5, 'fr')", "'18,50'"
		if strings.HasPrefix(file, "dashboard") {
			src, call = "formatDecimal(18.5, 2, 'fr')", "'18,50'"
		}
		runNodeModel(t, file, `
const fs = require('fs');
const vm = require('vm');
const assert = require('assert');
const mod = { exports: {} };
const ctx = vm.createContext({ module: mod, Intl: { NumberFormat: function () { throw new Error('no Intl'); } } });
vm.runInContext(fs.readFileSync(process.argv[2], 'utf8'), ctx);
const m = mod.exports;
assert.strictEqual(m.`+src+`, `+call+`);
assert.strictEqual(m.formatAccuracy(97.4231, 'de'), '97,42' + String.fromCharCode(160) + '%');
assert.strictEqual(m.formatAccuracy(97.4231, 'en'), '97.42%');
`)
	}
}
