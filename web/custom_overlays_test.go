package web

import (
	"io/fs"
	"strings"
	"testing"
)

func TestCustomOverlayCatalog(t *testing.T) {
	runNodeModel(t, "dashboard/pages/overlay-catalog.js", `
const assert = require('assert');
const cat = require(process.argv[2]);
const builtin = cat.OVERLAYS.map(function (o) { return o.id; });

assert.deepStrictEqual(cat.customList(), []);
cat.setCustom([
  { id: 'zeta', name: 'Zeta', description: 'd', width: 640, height: 120 },
  { id: 'now-playing', name: 'hijack' },
  { id: 'Upper', name: 'x' }, { id: '../x', name: 'x' }, { id: 'a b', name: 'x' }, { id: 5 }, { id: '' }, null, 'str', [],
  { id: 'zeta', name: 'dup' },
  { id: 'plain' },
  { id: 'hostile', name: '<img src=x onerror=alert(1)>\n\u0000' + 'n'.repeat(100), description: 'd'.repeat(500), width: 'wide', height: -4 },
  { id: 'huge', name: 'h', width: 1e9, height: Infinity },
  { id: 'nan', name: 'h', width: NaN, height: 5000 },
]);
const got = cat.customList();
assert.deepStrictEqual(got.map(function (o) { return o.id; }), ['zeta', 'plain', 'hostile', 'huge', 'nan']);
assert.deepStrictEqual(cat.OVERLAYS.map(function (o) { return o.id; }), builtin, 'built-ins are untouched');
const zeta = cat.byId('zeta');
assert.strictEqual(zeta.path, '/overlays/zeta/');
assert.strictEqual(zeta.custom, true);
assert.deepStrictEqual(zeta.params, []);
assert.strictEqual(cat.sizeText(zeta), '640 x 120');
const plain = cat.byId('plain');
assert.strictEqual(plain.name, 'plain');
assert.strictEqual(plain.description, '');
assert.strictEqual(cat.sizeText(plain), '500 x 200');
const hostile = cat.byId('hostile');
assert.ok(Array.from(hostile.name).length <= 40 && !/[\u0000-\u001f]/.test(hostile.name));
assert.ok(Array.from(hostile.description).length <= 160);
assert.strictEqual(cat.sizeText(hostile), '500 x 40');
assert.strictEqual(cat.sizeText(cat.byId('huge')), '3840 x 200');
assert.strictEqual(cat.sizeText(cat.byId('nan')), '500 x 2160');
assert.strictEqual(cat.byId('now-playing').custom, undefined);
assert.strictEqual(cat.byId('missing').id, 'now-playing');

cat.setCustom(Array.from({ length: 150 }, function (_, i) { return { id: 'o' + i }; }));
assert.strictEqual(cat.customList().length, 100);
for (const bad of [undefined, null, 'x', 5, {}]) {
  cat.setCustom(bad);
  assert.deepStrictEqual(cat.customList(), []);
}
`)
}

func TestCustomOverlayLinks(t *testing.T) {
	runNodeModel(t, "dashboard/pages/overlay-catalog.js", `
const assert = require('assert');
const cat = require(process.argv[2]);
const u = require(require('path').join(require('path').dirname(process.argv[2]), 'overlay-url.js'));
const O = 'http://127.0.0.1:7878';
const base = { theme: 'neutral-light', accent: '', defaultTheme: 'neutral-light', defaultAccent: '' };
cat.setCustom([{ id: 'mine', name: 'Mine' }]);

assert.strictEqual(u.buildOverlayUrl({ ...base, overlay: 'mine' }, false, O), O + '/overlays/mine/');
assert.strictEqual(u.buildOverlayUrl({ ...base, overlay: 'mine', theme: 'dark', accent: 'ff00aa' }, false, O), O + '/overlays/mine/?theme=dark&accent=ff00aa');
assert.strictEqual(u.buildOverlayUrl({ ...base, overlay: 'mine' }, true, O), '/overlays/mine/?theme=neutral-light');
const withOptions = u.buildOverlayUrl({ ...base, overlay: 'mine', autohide: '5', scale: '2', lang: 'fr', n: '30', hide: ['menu'] }, false, O);
assert.strictEqual(withOptions, O + '/overlays/mine/', 'a custom overlay honors no built-in option');
assert.ok(!u.buildOverlayUrl({ ...base, overlay: 'mine' }, true, O).includes('demo'));
assert.strictEqual(u.buildOverlayUrl({ ...base, overlay: 'now-playing' }, true, O), '/overlays/now-playing/?theme=neutral-light&demo=1');
assert.strictEqual(u.buildOverlayUrl({ ...base, overlay: '../mine' }, false, O), O + '/overlays/now-playing/');
cat.setCustom([]);
assert.strictEqual(u.buildOverlayUrl({ ...base, overlay: 'mine' }, false, O), O + '/overlays/now-playing/', 'a vanished overlay falls back');
`)
}

func TestCustomOverlayPageSources(t *testing.T) {
	read := func(p string) string {
		data, err := fs.ReadFile(FS, p)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	index := read("dashboard/index.html")
	last := -1
	for _, s := range []string{"overlay-catalog.js", "overlay-url.js", "overlay-util.js", "overlay-custom.js", "overlay-controls.js", "overlays.js"} {
		i := strings.Index(index, `src="pages/`+s+`"`)
		if i < 0 || i < last {
			t.Errorf("index.html must load pages/%s after the previous overlay scripts", s)
		}
		last = i
	}
	page := read("dashboard/pages/overlays.js")
	for _, want := range []string{"'/api/overlays'", "cat.setCustom(", "custom.dirHelp(model.overlaysDir)", "custom.frameHeight("} {
		if !strings.Contains(page, want) {
			t.Errorf("overlays.js is missing %s", want)
		}
	}
	if !strings.Contains(read("dashboard/pages/overlay-custom.js"), "qsc.pathNodes(dir)") || !strings.Contains(read("dashboard/pages/overlay-custom.js"), "overlays.custom_dir_help") {
		t.Error("overlay-custom.js must show the folder through pathNodes with the help text")
	}
	if !strings.Contains(read("dashboard/pages/overlay-controls.js"), "t('overlays.custom')") {
		t.Error("custom cards need the Custom pill")
	}
	for _, p := range []string{"overlays.js", "overlay-custom.js", "overlay-util.js", "overlay-controls.js", "overlay-catalog.js"} {
		if n := strings.Count(read("dashboard/pages/"+p), "\n"); n > 330 {
			t.Errorf("%s has %d lines, split it", p, n)
		}
	}
	for _, p := range []string{"overlays.js", "overlay-custom.js", "overlay-controls.js", "overlay-catalog.js"} {
		src := read("dashboard/pages/" + p)
		if strings.Contains(src, "sandbox: 'allow-scripts allow-same-origin'") || strings.Contains(src, "allow-same-origin") {
			t.Errorf("%s must keep the preview sandbox without allow-same-origin", p)
		}
	}
	if !strings.Contains(page, "sandbox: 'allow-scripts'") {
		t.Error("the preview iframe must stay sandboxed with allow-scripts only")
	}
}

func TestCustomOverlayStringsExistInEveryLanguage(t *testing.T) {
	for _, lang := range []string{"en", "fr", "de"} {
		c := loadCatalog(t, lang)
		if c["overlays.custom"] == "" || !strings.Contains(c["overlays.custom_dir_help"], "index.html") {
			t.Errorf("%s.json is missing the custom overlay strings", lang)
		}
	}
}
