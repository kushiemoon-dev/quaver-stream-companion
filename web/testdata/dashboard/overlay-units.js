// The overlays page helpers on their own: catalog, link builder, theme model, request wrapper, custom overlay labels and the option controls.
'use strict';
const assert = require('node:assert/strict');
const path = require('node:path');
const { test, run } = require('../../testsupport/run.js');
const H = require('./helpers.js');

const pages = path.join(H.WEB, 'dashboard', 'pages');
const cat = require(path.join(pages, 'overlay-catalog.js'));
const url = require(path.join(pages, 'overlay-url.js'));
const tm = require(path.join(pages, 'theme-model.js'));

const BASE = { overlay: 'now-playing', theme: 'neutral-light', accent: '', defaultTheme: 'neutral-light', defaultAccent: '', hide: [], counts: true, peak: true };
const link = (over, demo, origin) => url.buildOverlayUrl({ ...BASE, ...over }, demo, origin);
const ORIGIN = 'http://127.0.0.1:7878';

test('catalog: byId falls back to the first overlay, sizes read as width x height', () => {
  assert.equal(cat.byId('keys').id, 'keys');
  assert.equal(cat.byId('nope').id, 'now-playing');
  assert.equal(cat.byId(undefined).id, 'now-playing');
  assert.equal(cat.sizeText(cat.byId('state')), '280 x 64');
  assert.deepEqual(cat.OVERLAYS.map((o) => o.id), ['now-playing', 'result', 'history', 'state', 'alert', 'keys', 'curve']);
});

test('catalog: custom overlays are cleaned, limited and never shadow a built-in one', () => {
  const long = 'x'.repeat(100);
  cat.setCustom([
    { id: 'ok', name: '  A\u0000b\u007fc  ', description: 'd'.repeat(300), width: 10, height: 5000.6 },
    { id: 'nameless', width: NaN, height: Infinity }, { id: 'nameless', name: 'again' }, { id: 'result' }, { id: 'UP' }, { id: 5 }, null, 'x',
    { id: 'long', name: long, description: 7 },
  ]);
  const list = cat.customList();
  assert.deepEqual(list.map((o) => o.id), ['ok', 'nameless', 'long']);
  assert.equal(list[0].name, 'A b c');
  assert.equal(list[0].description.length, 160);
  assert.equal(list[0].width, 100);
  assert.equal(list[0].height, 2160);
  assert.deepEqual([list[1].name, list[1].width, list[1].height], ['nameless', 500, 200]);
  assert.equal(list[2].name.length, 40);
  assert.equal(list[2].description, '');
  assert.equal(list[0].path, '/overlays/ok/');
  assert.equal(list[0].custom, true);
  assert.deepEqual(list[0].params, []);
  assert.equal(cat.byId('ok').id, 'ok');
  cat.setCustom(Array.from({ length: 150 }, (_, i) => ({ id: 'c' + i })));
  assert.equal(cat.customList().length, 100);
  cat.setCustom('nope');
  assert.deepEqual(cat.customList(), []);
  assert.equal(cat.byId('ok').id, 'now-playing');
});

test('url: accent and theme validation', () => {
  for (const ok of ['abc', 'abcd', 'abcdef', 'abcdef12', '012']) assert.equal(url.isValidAccent(ok), true, ok);
  for (const bad of ['', 'ab', 'abcde', 'ABC', '#abc', 'ggg', 'abcdef123']) assert.equal(url.isValidAccent(bad), false, bad);
  assert.equal(url.normalizeAccent('  #AbC '), 'abc');
  assert.equal(url.normalizeAccent(12), '12');
  assert.equal(url.isValidTheme('neon-cosy'), true);
  for (const bad of ['', 'Neon', 'a b', '../x', 5, null, undefined]) assert.equal(url.isValidTheme(bad), false, String(bad));
});

test('url: a copied link carries only what differs from the saved default, a preview link always carries the look', () => {
  assert.equal(link({}, false, ORIGIN), ORIGIN + '/overlays/now-playing/');
  assert.equal(link({}, true, ORIGIN), '/overlays/now-playing/?theme=neutral-light&demo=1');
  assert.equal(link({ theme: 'pixel' }, false, ORIGIN), ORIGIN + '/overlays/now-playing/?theme=pixel');
  assert.equal(link({ accent: 'ABC', defaultAccent: '#abc' }, false, ORIGIN), ORIGIN + '/overlays/now-playing/');
  assert.equal(link({ accent: '#F0F' }, false, ORIGIN), ORIGIN + '/overlays/now-playing/?accent=f0f');
  assert.equal(link({ accent: 'f0f' }, true, ORIGIN), '/overlays/now-playing/?theme=neutral-light&accent=f0f&demo=1');
  assert.equal(link({ accent: undefined, defaultAccent: undefined }, false, ORIGIN), ORIGIN + '/overlays/now-playing/');
});

test('url: an invalid theme or accent is dropped and an unknown overlay falls back to the first', () => {
  assert.equal(link({ theme: '<x>', defaultTheme: 'pixel' }, false, ORIGIN), ORIGIN + '/overlays/now-playing/');
  assert.equal(link({ theme: 'a&b=c' }, true, ORIGIN), '/overlays/now-playing/?demo=1');
  assert.equal(link({ accent: 'zz' }, true, ORIGIN), '/overlays/now-playing/?theme=neutral-light&demo=1');
  assert.equal(link({ overlay: 'ghost' }, false, ORIGIN), ORIGIN + '/overlays/now-playing/');
});

test('url: option values are read as strings or numbers and the alert default delay is not repeated', () => {
  assert.equal(link({ overlay: 'alert', autohide: '8' }, false, ORIGIN), ORIGIN + '/overlays/alert/');
  assert.equal(link({ overlay: 'alert', autohide: 9 }, false, ORIGIN), ORIGIN + '/overlays/alert/?autohide=9');
  assert.equal(link({ overlay: 'alert', autohide: {} }, false, ORIGIN), ORIGIN + '/overlays/alert/');
  assert.equal(link({ overlay: 'alert', autohide: ' 12 ', scale: ' 2 ' }, false, ORIGIN), ORIGIN + '/overlays/alert/?autohide=12&scale=2');
  assert.equal(link({ overlay: 'history', lang: 'xx', n: '30', best: 'yes' }, false, ORIGIN), ORIGIN + '/overlays/history/?n=30');
  assert.deepEqual(url.overlayOptions({ scale: 'Infinity', lang: 'de' }, cat.byId('result')), { lang: 'de' });
  assert.deepEqual(url.overlayOptions({ hide: 'menu', counts: false }, cat.byId('state')), {});
  assert.deepEqual(url.overlayOptions({ hide: ['bogus', 'menu', 'closed'] }, cat.byId('state')), { hide: 'menu,closed' });
});

test('url: a custom overlay link has no demo flag and no options', () => {
  cat.setCustom([{ id: 'mine' }]);
  assert.equal(link({ overlay: 'mine', scale: '2' }, true, ORIGIN), '/overlays/mine/?theme=neutral-light');
  assert.equal(link({ overlay: 'mine', scale: '2' }, false, ORIGIN), ORIGIN + '/overlays/mine/');
  cat.setCustom([]);
});

test('theme model: names, colors and order', () => {
  assert.equal(tm.displayName('rose-pine'), 'Rose Pine');
  assert.equal(tm.displayName('mine'), 'mine');
  assert.equal(tm.displayName('constructor'), 'constructor');
  assert.equal(tm.displayName(7), '7');
  assert.equal(tm.isHexColor('#abc'), true);
  assert.equal(tm.isHexColor('#abcdef'), true);
  for (const bad of ['abc', '#abcd', '#ggg', 5, null, 'red']) assert.equal(tm.isHexColor(bad), false, String(bad));
  const list = tm.orderThemes([
    { id: 'zeta' }, { id: 'nord', colors: { bg: '#000', fg: 'x', accent: '#fff' } }, { id: 'alpha' }, { id: 'neutral-light' }, { id: 'nord' },
    { id: 'Bad' }, { id: 3 }, null, 'x', { id: 'dracula', colors: 'no' },
  ]);
  assert.deepEqual(list.map((x) => x.id), ['neutral-light', 'dracula', 'nord', 'alpha', 'zeta']);
  assert.deepEqual(list.find((x) => x.id === 'nord').colors, { bg: '#000', fg: '', accent: '#fff' });
  assert.deepEqual(tm.orderThemes('nope'), []);
  assert.deepEqual(tm.orderThemes(undefined), []);
});

test('theme model: cards mark the selected and default theme and keep an uninstalled saved theme selectable', () => {
  const cards = tm.buildCards([{ id: 'pixel' }, { id: 'mine' }], 'pixel', 'mine');
  assert.deepEqual(cards.map((c) => [c.id, c.selected, c.isDefault, c.custom]), [['pixel', true, false, false], ['mine', false, true, true]]);
  const extra = tm.buildCards([{ id: 'pixel' }], 'gone', 'pixel');
  assert.deepEqual(extra.map((c) => [c.id, c.selected, c.custom]), [['pixel', false, false], ['gone', true, true]]);
  assert.deepEqual(extra[1].colors, { bg: '', fg: '', accent: '' });
  assert.equal(tm.buildCards([{ id: 'pixel' }], 'Not Valid', 'pixel').length, 1);
});

test('util: pickerValue expands short colors, cuts alpha and starts from purple when empty or invalid', async () => {
  const { pickerValue } = (await H.open()).qscOverlayUtil;
  assert.equal(pickerValue('abc'), '#aabbcc');
  assert.equal(pickerValue('abcd'), '#aabbcc');
  assert.equal(pickerValue('123456'), '#123456');
  assert.equal(pickerValue('12345678'), '#123456');
  assert.equal(pickerValue(''), '#a855f7');
  assert.equal(pickerValue('xyz'), '#a855f7');
});

test('util: describe prefixes the field when the server named one', async () => {
  const { describe } = (await H.open()).qscOverlayUtil;
  assert.equal(describe({ message: 'bad', field: 'accent' }), 'accent: bad');
  assert.equal(describe({ message: 'bad', field: '' }), 'bad');
  assert.equal(describe({ message: 'bad' }), 'bad');
});

test('util: makeCall passes the result through, times out after 8 seconds and tells a page change from a timeout', async () => {
  const win = await H.open();
  let life = new win.AbortController();
  const call = win.qscOverlayUtil.makeCall(() => life);
  const idle = win.clock.pending();
  assert.equal(await call(async () => 'fine'), 'fine');
  await assert.rejects(call(async () => { throw new Error('boom'); }), { message: 'boom' });
  await assert.rejects(call(async () => { throw null; }), (e) => e === null);
  const hang = (signal) => new Promise((_, rej) => signal.addEventListener('abort', () => rej(Object.assign(new Error('x'), { name: 'AbortError' }))));
  const timed = call(hang);
  const seen = timed.catch((e) => e);
  await win.advance(7999);
  assert.equal(win.clock.pending() > 0, true);
  await win.advance(1);
  const err = await seen;
  assert.equal(err.message, 'The companion did not answer in time.');
  assert.equal(err.timedOut, true);
  const left = call(hang);
  const gone = left.catch((e) => e);
  life.abort();
  const err2 = await gone;
  assert.equal(err2.timedOut, false);
  life = new win.AbortController();
  const fast = await call(async () => 1);
  assert.equal(fast, 1);
  assert.equal(win.clock.pending(), idle);
});

test('custom labels: built-in overlays use the translations, custom ones their own text and a height-based frame', async () => {
  const win = await H.open();
  const c = win.qscOverlayCustom;
  const built = cat.byId('result');
  assert.equal(c.name(built), 'Result');
  assert.equal(c.desc(built), H.en['overlays.result.description']);
  assert.equal(c.frameClass(built), 'preview-frame size-result');
  const mine = { custom: true, id: 'mine', name: 'Mine', description: 'D', height: 100 };
  assert.equal(c.name(mine), 'Mine');
  assert.equal(c.desc(mine), 'D');
  assert.equal(c.frameClass(mine), 'preview-frame');
  const frame = { style: {} };
  c.frameHeight(frame, mine);
  assert.equal(frame.style.height, '120px');
  c.frameHeight(frame, { custom: true, height: 1000 });
  assert.equal(frame.style.height, '480px');
  c.frameHeight(frame, built);
  assert.equal(frame.style.height, '');
  assert.equal(c.dirHelp(''), null);
  assert.equal(c.dirHelp(undefined), null);
  const help = c.dirHelp('C:\\ov\\dir');
  assert.match(help.textContent, /C:.ov.dir/);
  assert.equal(help.querySelectorAll('wbr').length, 2);
});

test('controls: the overlay picker lists built-in then custom cards and reports only a checked choice', async () => {
  const win = await H.open();
  win.qscOverlayCatalog.setCustom([{ id: 'mine', name: 'Mine', width: 300, height: 90 }]);
  const picked = [];
  const el = win.qscOverlayControls.picker('result', (id) => picked.push(id));
  const radios = el.querySelectorAll('input');
  assert.deepEqual(radios.map((r) => r.getAttribute('value')), ['now-playing', 'result', 'history', 'state', 'alert', 'keys', 'curve', 'mine']);
  assert.deepEqual(radios.filter((r) => r.hasAttribute('checked')).map((r) => r.getAttribute('value')), ['result']);
  assert.equal(el.querySelector('legend').textContent, 'Overlay');
  radios[2].checked = false;
  radios[2].dispatchEvent(new win.Event('change'));
  assert.deepEqual(picked, []);
  radios[7].checked = true;
  radios[7].dispatchEvent(new win.Event('change'));
  assert.deepEqual(picked, ['mine']);
  assert.match(radios[7].closest('label').textContent, /Custom300 x 90/);
});

test('controls: options reports changes and shows only the controls an overlay honors', async () => {
  const win = await H.open();
  const changes = [];
  const model = { autohide: '4', scale: '', lang: 'de', n: '', best: true, counts: true, peak: false, sound: false, hide: ['menu'] };
  const { el, sync } = win.qscOverlayControls.options(model, (patch) => changes.push(patch));
  const input = (id) => el.querySelector('#' + id);
  assert.equal(input('ov-autohide').getAttribute('value'), '4');
  assert.equal(input('ov-autohide').getAttribute('min'), '1');
  assert.equal(input('ov-scale').getAttribute('placeholder'), '1');
  assert.equal(input('ov-count').getAttribute('max'), '50');
  assert.ok(el.querySelector('input[name="ov-lang"][value="de"]').hasAttribute('checked'));
  assert.equal(el.querySelectorAll('input[name="ov-lang"][checked]').length, 1);
  const hide = el.querySelectorAll('.check-list input');
  assert.equal(hide.length, 15);
  assert.equal(hide.filter((i) => i.hasAttribute('checked')).map((i) => i.getAttribute('value')).join(), 'menu');
  sync(cat.byId('keys'));
  const visible = () => el.children.slice(1).filter((c) => !c.hidden).map((c) => c.querySelector('label, legend').textContent);
  assert.deepEqual(visible().sort(), ['Overlay language', 'Peak keys per second', 'Press counters', 'Scale']);
  sync(cat.byId('state'));
  assert.equal(visible().length, 3);
  input('ov-count').value = '7';
  input('ov-count').dispatchEvent(new win.Event('input'));
  input('ov-autohide').value = '3';
  input('ov-autohide').dispatchEvent(new win.Event('input'));
  input('ov-scale').value = '2';
  input('ov-scale').dispatchEvent(new win.Event('input'));
  hide.find((i) => i.getAttribute('value') === 'lobby').checked = true;
  hide.find((i) => i.getAttribute('value') === 'lobby').dispatchEvent(new win.Event('change'));
  hide.find((i) => i.getAttribute('value') === 'menu').checked = false;
  hide.find((i) => i.getAttribute('value') === 'menu').dispatchEvent(new win.Event('change'));
  assert.deepEqual(JSON.parse(JSON.stringify(changes)), [{ n: '7' }, { autohide: '3' }, { scale: '2' }, { hide: ['menu', 'lobby'] }, { hide: [] }]);
  const peak = el.querySelectorAll('input[type="checkbox"]').filter((i) => !i.getAttribute('value'));
  peak.forEach((i, k) => { i.checked = k % 2 === 0; i.dispatchEvent(new win.Event('change')); });
  assert.deepEqual(changes.slice(5).map((c) => Object.keys(c)[0] + '=' + Object.values(c)[0]), ['best=true', 'counts=false', 'peak=true', 'sound=false']);
});

run();
