package web

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestHistoryAndSettingsPagesAreLoadedAndNotPlaceholders(t *testing.T) {
	index, err := fs.ReadFile(FS, "dashboard/index.html")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fs.Stat(FS, "dashboard/pages/placeholders.js"); err == nil {
		t.Error("the placeholder page must be removed once every page is real")
	}
	if strings.Contains(string(index), "placeholders.js") {
		t.Error("index.html must not load the placeholder page")
	}
	for _, name := range []string{"history", "settings"} {
		for _, f := range []string{name + "-model.js", name + ".js"} {
			if !strings.Contains(string(index), `src="pages/`+f+`"`) {
				t.Errorf("index.html must load pages/%s", f)
			}
		}
		page, err := fs.ReadFile(FS, "dashboard/pages/"+name+".js")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(page), "registerPage('"+name+"'") {
			t.Errorf("%s page must register itself", name)
		}
		model := strings.Index(string(index), "pages/"+name+"-model.js")
		page2 := strings.Index(string(index), "pages/"+name+".js")
		if model > page2 {
			t.Errorf("%s model script must load before its page", name)
		}
	}
}

func runNodeModel(t *testing.T, model, script string) {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not on PATH")
	}
	file := filepath.Join(t.TempDir(), "check.js")
	if err := os.WriteFile(file, []byte(script), 0o600); err != nil {
		t.Fatal(err)
	}
	lib, err := filepath.Abs(model)
	if err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(node, file, lib).CombinedOutput()
	if err != nil {
		t.Fatalf("node check failed: %v: %s", err, out)
	}
}

func TestHistoryModel(t *testing.T) {
	runNodeModel(t, "dashboard/pages/history-model.js", historyModelScript)
}

func TestSettingsModel(t *testing.T) {
	runNodeModel(t, "dashboard/pages/settings-model.js", settingsModelScript)
}

const historyModelScript = `
const assert = require('assert');
const m = require(process.argv[2]);

assert.strictEqual(m.formatRate(1), '');
assert.strictEqual(m.formatRate(0), '');
assert.strictEqual(m.formatRate(undefined), '');
assert.strictEqual(m.formatRate(1.15), '1.15x');
assert.strictEqual(m.formatRate(0.9), '0.9x');
assert.strictEqual(m.formatRate('x'), '');
assert.strictEqual(m.formatRate(1.004), '');
assert.strictEqual(m.formatRate(0.996), '');
assert.strictEqual(m.formatRate(1.006), '1.01x');

assert.deepStrictEqual(m.modChips(['A', 'B']), { shown: ['A', 'B'], extra: 0, hidden: [] });
assert.deepStrictEqual(m.modChips(['A', 'B', 'C', 'D', 'E']), { shown: ['A', 'B', 'C'], extra: 2, hidden: ['D', 'E'] });
assert.deepStrictEqual(m.modChips(null), { shown: [], extra: 0, hidden: [] });
assert.deepStrictEqual(m.modChips('nope'), { shown: [], extra: 0, hidden: [] });

for (const [g, c] of [['SS', 'gold'], ['X', 'gold'], ['S', 'gold'], ['A', 'green'], ['B', 'blue'], ['C', 'amber'], ['D', 'amber'], ['F', 'red'], ['-', 'muted'], ['', 'muted'], [null, 'muted'], ['<b>', 'muted']]) {
  assert.strictEqual(m.gradeClass(g), c, String(g));
}
assert.strictEqual(m.gradeText(''), '-');
assert.strictEqual(m.gradeText(null), '-');
assert.strictEqual(m.gradeText('SS'), 'SS');

assert.strictEqual(m.formatStars(5.678), '5.7');
assert.strictEqual(m.formatStars('x'), '0.0');
assert.strictEqual(m.formatAccuracy(98.7654), '98.77%');
assert.strictEqual(m.formatAccuracy(undefined), '0.00%');

assert.strictEqual(m.formatPlayedAt('not a date <b>'), 'not a date <b>');
assert.strictEqual(m.formatPlayedAt(''), '');
assert.strictEqual(m.formatPlayedAt(null), '');
assert.notStrictEqual(m.formatPlayedAt('2026-10-12 21:03:44'), '2026-10-12 21:03:44');
assert.ok(m.formatPlayedAt('2026-10-12T21:03:44Z').length > 0);
const stamp = '2026-10-12 17:21:00';
const en = m.formatPlayedAt(stamp, 'en');
const fr = m.formatPlayedAt(stamp, 'fr');
const de = m.formatPlayedAt(stamp, 'de');
assert.ok(/Oct/.test(en) && /2026/.test(en), en);
assert.ok(/oct/.test(fr) && /2026/.test(fr), fr);
assert.ok(/^12.10.2026/.test(de) && /2026/.test(de), de);
assert.strictEqual(new Set([en, fr, de]).size, 3);
assert.strictEqual(m.formatPlayedAt('nope', 'de'), 'nope');
assert.strictEqual(m.formatPlayedAt(stamp, 'zz-invalid-!'), m.formatPlayedAt(stamp, 'en'));
assert.strictEqual(m.formatPlayedAt(stamp, undefined), m.formatPlayedAt(stamp, 'en'));

const plays = [
  { id: 1, is_pb: true, full_combo: false, failed: false },
  { id: 2, is_pb: false, full_combo: true, failed: false },
  { id: 3, is_pb: false, full_combo: false, failed: true },
  { id: 4 },
];
const ids = (f) => m.filterPlays(plays, f).map((p) => p.id);
assert.deepStrictEqual(ids('all'), [1, 2, 3, 4]);
assert.deepStrictEqual(ids('pb'), [1]);
assert.deepStrictEqual(ids('fc'), [2]);
assert.deepStrictEqual(ids('failed'), [3]);
assert.deepStrictEqual(ids('bogus'), [1, 2, 3, 4]);
assert.deepStrictEqual(m.filterPlays(null, 'pb'), []);
assert.strictEqual(plays.length, 4);

assert.deepStrictEqual(m.LIMITS, [25, 50, 100, 200]);
assert.strictEqual(m.DEFAULT_LIMIT, 50);
assert.strictEqual(m.parseLimit('100'), 100);
assert.strictEqual(m.parseLimit('7'), 50);
assert.strictEqual(m.parseLimit('<x>'), 50);

assert.deepStrictEqual(m.playsOf({ not: 'an array' }), []);
assert.deepStrictEqual(m.playsOf([{ id: 1 }, null, 3]), [{ id: 1 }]);
`

const settingsModelScript = `
const assert = require('assert');
const m = require(process.argv[2]);

assert.deepStrictEqual(m.validatePort('7878'), { ok: true, value: 7878 });
assert.deepStrictEqual(m.validatePort(' 1 '), { ok: true, value: 1 });
assert.deepStrictEqual(m.validatePort('65535'), { ok: true, value: 65535 });
for (const bad of ['', '0', '65536', '-1', '1.5', 'abc', '1e3', '0x10', '80 80', '<script>', null, undefined]) {
  assert.strictEqual(m.validatePort(bad).ok, false, String(bad));
}

const current = { quaver_dir: 'C:\\Q', port: 7878, language: 'fr', overlay: { theme: 'dark', accent: 'abc' } };
const frozen = JSON.stringify(current);
const body = m.buildConfigBody(current, { quaver_dir: '  D:\\Games\\Quaver ', port: 8000, language: 'de' }, []);
assert.deepStrictEqual(body, { quaver_dir: 'D:\\Games\\Quaver', port: 8000, language: 'de', overlay: { theme: 'dark', accent: 'abc' } });
assert.strictEqual(JSON.stringify(current), frozen);
assert.notStrictEqual(body.overlay, current.overlay);

const kept = m.buildConfigBody(current, { quaver_dir: 'X', port: 9, language: '' }, ['port', 'quaver_dir']);
assert.strictEqual(kept.port, 7878);
assert.strictEqual(kept.quaver_dir, 'C:\\Q');
assert.strictEqual(kept.language, '');

const noOverlay = m.buildConfigBody({ quaver_dir: 'a', port: 1, language: '' }, { quaver_dir: 'b', port: 2, language: 'en' }, []);
assert.strictEqual('overlay' in noOverlay, false);
assert.deepStrictEqual(noOverlay, { quaver_dir: 'b', port: 2, language: 'en' });

const extra = m.buildConfigBody({ ...current, future_field: 1 }, { quaver_dir: '', port: 1, language: 'en' }, []);
assert.strictEqual(extra.future_field, 1);
assert.strictEqual(extra.quaver_dir, '');

for (const f of ['port', 'quaver_dir', 'language']) assert.strictEqual(m.fieldFor(f), f);
for (const other of ['overlay.theme', '', undefined, '__proto__', 'constructor']) assert.strictEqual(m.fieldFor(other), '');

assert.deepStrictEqual(m.LANGUAGES, ['', 'en', 'fr', 'de']);
assert.strictEqual(m.isOverridden(['port'], 'port'), true);
assert.strictEqual(m.isOverridden(['port'], 'language'), false);
assert.strictEqual(m.isOverridden(null, 'port'), false);
`

func TestPagesCreateTextOnlyThroughHelper(t *testing.T) {
	for _, name := range []string{"history.js", "settings.js"} {
		data, err := fs.ReadFile(FS, "dashboard/pages/"+name)
		if err != nil {
			t.Fatal(err)
		}
		for _, bad := range []string{".innerHTML", ".outerHTML", "insertAdjacentHTML", "document.write", "eval(", "new Function"} {
			if strings.Contains(string(data), bad) {
				t.Errorf("%s uses %s", name, bad)
			}
		}
	}
}

func TestThemeModel(t *testing.T) {
	runNodeModel(t, "dashboard/pages/theme-model.js", themeModelScript)
}

const themeModelScript = `
const assert = require('assert');
const m = require(process.argv[2]);

const names = { 'neutral-light': 'Neutral Light', 'neutral-dark': 'Neutral Dark', 'catppuccin-mocha': 'Catppuccin Mocha', 'catppuccin-latte': 'Catppuccin Latte', 'tokyo-night': 'Tokyo Night', dracula: 'Dracula', nord: 'Nord', gruvbox: 'Gruvbox', 'rose-pine': 'Rose Pine', pixel: 'Pixel', 'neon-cosy': 'Neon Cosy' };
for (const [id, name] of Object.entries(names)) assert.strictEqual(m.displayName(id), name);
for (const id of ['my-theme', 'toString', '__proto__', 'constructor']) assert.strictEqual(m.displayName(id), id);

for (const ok of ['#fff', '#FFF', '#112233', '#aBcDeF']) assert.ok(m.isHexColor(ok), ok);
for (const bad of ['', 'fff', '#ff', '#ffff', '#fffff', '#fffffff', '#ggg', 'red', 'rgb(1,2,3)', '#fff;x', 'url(x)', null, undefined, 5]) assert.ok(!m.isHexColor(bad), String(bad));

const c = (bg) => ({ bg, fg: '#111', accent: '#222' });
const ids = (l) => m.orderThemes(l).map((t) => t.id);
const shuffled = Object.keys(names).reverse().map((id) => ({ id, colors: c('#000') }));
assert.deepStrictEqual(ids(shuffled), Object.keys(names));
assert.deepStrictEqual(ids([{ id: 'zeta' }, { id: 'pixel' }, { id: 'alpha' }, { id: 'neutral-light' }]), ['neutral-light', 'pixel', 'alpha', 'zeta']);
assert.deepStrictEqual(ids([{ id: 'a' }, { id: 'a' }, { id: '../x' }, { id: '' }, { id: 'A' }, null, 3, { colors: {} }]), ['a']);
assert.deepStrictEqual(m.orderThemes(null), []);
assert.deepStrictEqual(m.orderThemes('nope'), []);

assert.deepStrictEqual(m.orderThemes([{ id: 'x', colors: { bg: 'red', fg: '#abc', accent: 'url(x)' } }])[0].colors, { bg: '', fg: '#abc', accent: '' });
assert.deepStrictEqual(m.orderThemes([{ id: 'x' }])[0].colors, { bg: '', fg: '', accent: '' });
assert.deepStrictEqual(m.orderThemes([{ id: 'x', colors: 'bad' }])[0].colors, { bg: '', fg: '', accent: '' });

const list = [{ id: 'nord', colors: c('#2e3440') }, { id: 'mine', colors: c('#000') }];
const cards = m.buildCards(list, 'mine', 'nord');
assert.deepStrictEqual(cards.map((x) => x.id), ['nord', 'mine']);
assert.deepStrictEqual(cards.map((x) => x.name), ['Nord', 'mine']);
assert.deepStrictEqual(cards.map((x) => x.custom), [false, true]);
assert.deepStrictEqual(cards.map((x) => x.selected), [false, true]);
assert.deepStrictEqual(cards.map((x) => x.isDefault), [true, false]);
assert.strictEqual(cards[0].colors.bg, '#2e3440');

const orphan = m.buildCards(list, 'gone', 'gone');
assert.deepStrictEqual(orphan.map((x) => x.id), ['nord', 'mine', 'gone']);
assert.ok(orphan[2].custom && orphan[2].selected && orphan[2].isDefault);
assert.deepStrictEqual(orphan[2].colors, { bg: '', fg: '', accent: '' });
assert.deepStrictEqual(m.buildCards(list, '../x', 'nord').map((x) => x.id), ['nord', 'mine']);
assert.deepStrictEqual(m.buildCards(undefined, 'neutral-light', 'neutral-light').map((x) => x.id), ['neutral-light']);
assert.ok(!m.buildCards(list, 'nord', 'nord')[0].custom);
`

func TestThemePickerPage(t *testing.T) {
	index, err := fs.ReadFile(FS, "dashboard/index.html")
	if err != nil {
		t.Fatal(err)
	}
	model := strings.Index(string(index), `src="pages/theme-model.js"`)
	page := strings.Index(string(index), `src="pages/overlays.js"`)
	if model < 0 || page < 0 || model > page {
		t.Error("index.html must load pages/theme-model.js before pages/overlays.js")
	}
	src, err := fs.ReadFile(FS, "dashboard/pages/overlays.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"'fieldset'", "'legend'", "type: 'radio'", "setProperty(", "'aria-hidden': 'true'", "qscThemeModel"} {
		if !strings.Contains(string(src), want) {
			t.Errorf("overlays.js is missing %s", want)
		}
	}
	for _, bad := range []string{"h('select'", "h('option'", "style:", "'style'"} {
		if strings.Contains(string(src), bad) {
			t.Errorf("overlays.js must not use %s", bad)
		}
	}
	css, err := fs.ReadFile(FS, "dashboard/style.css")
	if err != nil {
		t.Fatal(err)
	}
	grid := regexp.MustCompile(`\.theme-grid\s*\{[^}]*\}`).FindString(string(css))
	if !strings.Contains(grid, "auto-fill") || !strings.Contains(grid, "min(") {
		t.Errorf(".theme-grid must wrap without overflow, got %q", grid)
	}
	if !strings.Contains(string(css), ".theme-input:focus-visible + .theme-body") {
		t.Error("theme cards need a visible focus ring")
	}
}

func TestOverlaysPageShowsTheThemesFolder(t *testing.T) {
	src, err := fs.ReadFile(FS, "dashboard/pages/overlays.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"themes_dir", "overlays.themes_dir_help", "qsc.pathNodes(model.themesDir)"} {
		if !strings.Contains(string(src), want) {
			t.Errorf("overlays.js is missing %s", want)
		}
	}
	for _, lang := range []string{"en", "fr", "de"} {
		if !strings.Contains(loadCatalog(t, lang)["overlays.themes_dir_help"], ".css") {
			t.Errorf("%s.json is missing overlays.themes_dir_help", lang)
		}
	}
}
