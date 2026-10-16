package web

import (
	"io/fs"
	"strings"
	"testing"
)

func TestOverlayCatalogAndOptions(t *testing.T) {
	runNodeModel(t, "dashboard/pages/overlay-url.js", `
const assert = require('assert');
const u = require(process.argv[2]);
const cat = require(require('path').join(require('path').dirname(process.argv[2]), 'overlay-catalog.js'));
const O = 'http://127.0.0.1:7878';
const base = { theme: 'neutral-light', accent: '', defaultTheme: 'neutral-light', defaultAccent: '' };
const link = function (m, demo) { return u.buildOverlayUrl({ ...base, ...m }, !!demo, O); };

assert.deepStrictEqual(cat.OVERLAYS.map(function (o) { return o.id; }), ['now-playing', 'result', 'history', 'state', 'alert', 'keys', 'curve', 'session']);
assert.strictEqual(cat.sizeText(cat.byId('now-playing')), '540 x 140');
assert.strictEqual(cat.byId('nope').id, 'now-playing');
for (const o of cat.OVERLAYS) {
  assert.ok(o.path.startsWith('/overlays/') && o.path.endsWith('/'), o.id);
  assert.strictEqual(link({ overlay: o.id }), O + o.path, 'defaults carry nothing for ' + o.id);
  assert.strictEqual(link({ overlay: o.id }, true), '/overlays/' + o.id + '/?theme=neutral-light&demo=1');
}

assert.strictEqual(link({ overlay: 'result', autohide: '8', scale: '1.5', lang: 'fr' }), O + '/overlays/result/?autohide=8&scale=1.5&lang=fr');
assert.strictEqual(link({ overlay: 'history', n: '30', theme: 'dark' }), O + '/overlays/history/?theme=dark&n=30');
assert.strictEqual(link({ overlay: 'state', hide: ['unknown', 'menu', 'bogus', 'menu'] }), O + '/overlays/state/?hide=menu%2Cunknown');
assert.strictEqual(link({ overlay: 'state', scale: 2 }, true), '/overlays/state/?theme=neutral-light&scale=2&demo=1');

// options an overlay does not honor never leak into its link
assert.strictEqual(link({ overlay: 'history', autohide: '5', hide: ['menu'] }), O + '/overlays/history/');
assert.strictEqual(link({ overlay: 'now-playing', n: '30', hide: ['menu'] }), O + '/overlays/now-playing/');

// the map details line is opt-in and only for now playing
assert.strictEqual(link({ overlay: 'now-playing', meta: true }), O + '/overlays/now-playing/?meta=1');
assert.strictEqual(link({ overlay: 'now-playing', meta: false }), O + '/overlays/now-playing/');
assert.strictEqual(link({ overlay: 'result', meta: true }), O + '/overlays/result/');

// alert hides after 8 seconds by default, so only a different value is carried
assert.strictEqual(link({ overlay: 'alert' }), O + '/overlays/alert/');
assert.strictEqual(link({ overlay: 'alert', autohide: '8' }), O + '/overlays/alert/');
assert.strictEqual(link({ overlay: 'alert', autohide: '12', sound: true, scale: '2' }), O + '/overlays/alert/?autohide=12&scale=2&sound=1');
assert.strictEqual(link({ overlay: 'alert', sound: false }), O + '/overlays/alert/');
assert.strictEqual(link({ overlay: 'alert', best: true }), O + '/overlays/alert/');
assert.strictEqual(link({ overlay: 'history', best: true }), O + '/overlays/history/?best=1');
assert.strictEqual(link({ overlay: 'history', best: 'yes' }), O + '/overlays/history/');
assert.strictEqual(link({ overlay: 'history', sound: true }), O + '/overlays/history/');
assert.strictEqual(link({ overlay: 'alert' }, true), '/overlays/alert/?theme=neutral-light&demo=1');

// keys: counts and peak are on by default, only an explicit off is carried
assert.strictEqual(link({ overlay: 'keys' }), O + '/overlays/keys/');
assert.strictEqual(link({ overlay: 'keys', counts: true, peak: true }), O + '/overlays/keys/');
assert.strictEqual(link({ overlay: 'keys', counts: false }), O + '/overlays/keys/?counts=0');
assert.strictEqual(link({ overlay: 'keys', counts: false, peak: false, scale: '1.5', lang: 'de' }), O + '/overlays/keys/?scale=1.5&lang=de&counts=0&peak=0');
assert.strictEqual(link({ overlay: 'keys', n: '30', best: true, autohide: '5' }), O + '/overlays/keys/');
assert.strictEqual(link({ overlay: 'history', counts: false, peak: false }), O + '/overlays/history/');
assert.strictEqual(link({ overlay: 'keys' }, true), '/overlays/keys/?theme=neutral-light&demo=1');

// defaults and clamping
assert.strictEqual(link({ overlay: 'result', scale: '1', lang: 'auto', autohide: '0' }), O + '/overlays/result/');
assert.strictEqual(link({ overlay: 'history', n: '20' }), O + '/overlays/history/');
assert.strictEqual(link({ overlay: 'result', scale: '99' }), O + '/overlays/result/?scale=3');
assert.strictEqual(link({ overlay: 'result', scale: '0.1' }), O + '/overlays/result/?scale=0.5');
assert.strictEqual(link({ overlay: 'result', autohide: '99999' }), O + '/overlays/result/?autohide=3600');
assert.strictEqual(link({ overlay: 'result', autohide: '7.9' }), O + '/overlays/result/?autohide=7');
assert.strictEqual(link({ overlay: 'history', n: '1' }), O + '/overlays/history/?n=5');
assert.strictEqual(link({ overlay: 'history', n: '500' }), O + '/overlays/history/?n=50');
assert.strictEqual(link({ overlay: 'result', scale: '1.234' }), O + '/overlays/result/?scale=1.23');

// hostile values never reach the link
for (const bad of ['', ' ', 'abc', '-5', '0', 'NaN', 'Infinity', '"><script>', '5&x=1', null, undefined, {}, []]) {
  assert.strictEqual(link({ overlay: 'result', autohide: bad, scale: bad }), O + '/overlays/result/', String(bad));
}
for (const bad of ['', ' ', 'abc', 'NaN', '"><script>', '5&x=1', null, undefined, {}, []]) {
  assert.strictEqual(link({ overlay: 'history', n: bad }), O + '/overlays/history/', String(bad));
}
for (const bad of ['xx', 'EN', 'en,fr', '"><script>', '', null, 5, 'auto']) {
  assert.strictEqual(link({ overlay: 'result', lang: bad }), O + '/overlays/result/', String(bad));
}
for (const bad of ['menu', 'a,b', null, 5, {}, '"><script>']) {
  assert.strictEqual(link({ overlay: 'state', hide: bad }), O + '/overlays/state/', String(bad));
}
assert.strictEqual(link({ overlay: '../../etc', scale: 2 }), O + '/overlays/now-playing/?scale=2');
`)
}

func TestOverlayOptionsPageUsesSafeControls(t *testing.T) {
	index, err := fs.ReadFile(FS, "dashboard/index.html")
	if err != nil {
		t.Fatal(err)
	}
	cat := strings.Index(string(index), `src="pages/overlay-catalog.js"`)
	url := strings.Index(string(index), `src="pages/overlay-url.js"`)
	if cat < 0 || url < 0 || cat > url {
		t.Error("index.html must load pages/overlay-catalog.js before pages/overlay-url.js")
	}
	src, err := fs.ReadFile(FS, "dashboard/pages/overlays.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"qscOverlayCatalog", "sizeText(", "qscOverlayControls"} {
		if !strings.Contains(string(src), want) {
			t.Errorf("overlays.js is missing %s", want)
		}
	}
	controls, err := fs.ReadFile(FS, "dashboard/pages/overlay-controls.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"type: 'radio'", "name: 'ov-overlay'", "'fieldset'", "'legend'", "type: 'checkbox'"} {
		if !strings.Contains(string(controls), want) {
			t.Errorf("overlay-controls.js is missing %s", want)
		}
	}
}

func TestOverlayRecommendedSizes(t *testing.T) {
	runNodeModel(t, "dashboard/pages/overlay-catalog.js", `
const assert = require('assert');
const cat = require(process.argv[2]);
const sizes = cat.OVERLAYS.map(function (o) { return o.id + ' ' + cat.sizeText(o); });
assert.deepStrictEqual(sizes, ['now-playing 540 x 140', 'result 520 x 230', 'history 520 x 180', 'state 280 x 64', 'alert 520 x 120', 'keys 480 x 200', 'curve 520 x 180', 'session 480 x 170']);
`)
}
