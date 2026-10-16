// pages/overlays.js with its controls: selecting an overlay, options, themes, accent, copy buttons, saving the default look and custom overlays.
'use strict';
const assert = require('node:assert/strict');
const { test, run } = require('../../testsupport/run.js');
const H = require('./helpers.js');

const THEMES = [
  { id: 'pixel', colors: { bg: '#000000', fg: '#ffffff', accent: '#ff00ff' } },
  { id: 'neutral-light', colors: { bg: '#ffffff', fg: '#111111', accent: '#a855f7' } },
  { id: 'zen', colors: {} },
];
const CONFIG = { config: { language: '', port: 7878, overlay: { theme: 'neutral-light', accent: '', extra: 1 } }, themes_dir: 'C:/themes' };
const HOSTILE = '<img src=x onerror=alert(1)>';
const ORIGIN = 'http://127.0.0.1:7878';

function setup(over = {}) {
  const puts = [];
  const routes = {
    'GET /api/themes': () => H.json(THEMES),
    'GET /api/config': () => H.json(CONFIG),
    'GET /api/overlays': () => H.json({ dir: 'C:/overlays', custom: [] }),
    'PUT /api/config': (call) => { puts.push(JSON.parse(call.body)); return H.json({}); },
    ...over,
  };
  return { puts, routes };
}

async function openOverlays(over) {
  const s = setup(over);
  const win = await H.open({ hash: '#/overlays', routes: s.routes });
  return Object.assign(s, { win });
}

const text = (el) => el.textContent;
const fire = (win, el, type) => el.dispatchEvent(new win.Event(type));
const typeIn = (win, el, value) => { el.value = value; fire(win, el, 'input'); };
function choose(win, el) { el.value = el.getAttribute('value'); el.checked = true; fire(win, el, 'change'); }
const tick = (win, checked, el) => { el.checked = checked; fire(win, el, 'change'); };
const overlayRadio = (win, id) => win.$('input[name="ov-overlay"][value="' + id + '"]');
const themeRadio = (win, id) => win.$('input[name="ov-theme"][value="' + id + '"]');
const link = (win) => win.$('#ov-url').value;
const query = (win) => new URL(link(win)).search;
const frame = (win) => win.$('.preview-frame');
const shown = (win, id) => !win.$('#' + id).closest('.field').hidden;
const checkbox = (win, label) => win.$$('.overlay-options .check').filter((l) => text(l) === label)[0].querySelector('input');
const copyButton = (win) => win.$('.url-row .btn');
const saveButton = (win) => win.$('.save-row .btn-primary');
const reloadButton = (win) => win.$('.save-row .btn:not(.btn-primary)');
const saveStatus = (win) => win.$('.save-status');
const linkStatus = (win) => text(win.$('#ov-url').closest('.field').querySelector('.status-line'));
const sizeStatus = (win) => text(win.$('.preview .status-line'));
const themeCards = (win) => win.$$('.theme-card').filter((c) => c.querySelector('[name="ov-theme"]'));
const settle = async (win, ms) => { await win.advance(ms); };
const selectOverlay = async (win, id) => { choose(win, overlayRadio(win, id)); await settle(win, 250); };
const never = () => new Promise(() => {});

test('opens on the first overlay with its link, size, preview and the options it honors', async () => {
  const { win } = await openOverlays();
  assert.equal(win.$$('input[name="ov-overlay"]').length, 8);
  assert.ok(overlayRadio(win, 'now-playing').hasAttribute('checked'));
  assert.equal(text(win.$('#ov-name')), 'Now playing');
  assert.equal(text(win.$('#ov-size')), '540 x 140');
  assert.equal(link(win), ORIGIN + '/overlays/now-playing/');
  assert.equal(frame(win).getAttribute('src'), '/overlays/now-playing/?theme=neutral-light&demo=1');
  assert.equal(frame(win).getAttribute('title'), 'Preview of the Now playing overlay with example data');
  assert.equal(frame(win).getAttribute('sandbox'), 'allow-scripts');
  assert.equal(frame(win).className, 'preview-frame size-now-playing');
  assert.equal(copyButton(win).getAttribute('aria-label'), 'Copy the Now playing overlay link');
  assert.deepEqual(['ov-autohide', 'ov-scale', 'ov-count'].map((id) => shown(win, id)), [true, true, false]);
  assert.match(text(win.$('#ov-autohide-hint')), /Leave empty to keep it visible/);
  assert.equal(win.$('#ov-hex').value, '');
  assert.equal(win.$('#ov-accent').value, '#a855f7');
});

test('selecting an overlay changes the link, size, labels and options at once, and the preview after 250 ms', async () => {
  const { win } = await openOverlays();
  const first = frame(win).getAttribute('src');
  choose(win, overlayRadio(win, 'history'));
  assert.equal(link(win), ORIGIN + '/overlays/history/');
  assert.equal(text(win.$('#ov-size')), '520 x 180');
  assert.equal(text(win.$('#ov-name')), 'History');
  assert.equal(frame(win).className, 'preview-frame size-history');
  assert.equal(copyButton(win).getAttribute('aria-label'), 'Copy the History overlay link');
  assert.deepEqual(['ov-autohide', 'ov-scale', 'ov-count'].map((id) => shown(win, id)), [false, true, true]);
  assert.equal(frame(win).getAttribute('src'), first);
  await settle(win, 249);
  assert.equal(frame(win).getAttribute('src'), first);
  await settle(win, 1);
  assert.equal(frame(win).getAttribute('src'), '/overlays/history/?theme=neutral-light&demo=1');
  overlayRadio(win, 'result').checked = false;
  fire(win, overlayRadio(win, 'result'), 'change');
  assert.equal(link(win), ORIGIN + '/overlays/history/');
});

test('rapid changes send a single preview update and the alert overlay explains its own default delay', async () => {
  const { win } = await openOverlays();
  await selectOverlay(win, 'alert');
  assert.match(text(win.$('#ov-autohide-hint')), /hide after 8 seconds/);
  const before = frame(win).getAttribute('src');
  typeIn(win, win.$('#ov-scale'), '2');
  await settle(win, 200);
  typeIn(win, win.$('#ov-scale'), '1.5');
  await settle(win, 200);
  assert.equal(frame(win).getAttribute('src'), before);
  await settle(win, 50);
  assert.match(frame(win).getAttribute('src'), /scale=1\.5/);
  await selectOverlay(win, 'result');
  assert.match(text(win.$('#ov-autohide-hint')), /keep it visible/);
});

test('numeric options are clamped to what the overlay accepts and defaults are left out of the link', async () => {
  const { win } = await openOverlays();
  const cases = [
    ['#ov-autohide', '5000', '?autohide=3600'], ['#ov-autohide', '7.9', '?autohide=7'], ['#ov-autohide', '0', ''], ['#ov-autohide', '-4', ''], ['#ov-autohide', 'abc', ''],
    ['#ov-scale', '10', '?scale=3'], ['#ov-scale', '0.1', '?scale=0.5'], ['#ov-scale', '1.234', '?scale=1.23'], ['#ov-scale', '1', ''], ['#ov-scale', '-2', ''],
  ];
  for (const [sel, value, want] of cases) {
    typeIn(win, win.$(sel), value);
    assert.equal(query(win), want, sel + ' ' + value);
  }
  await selectOverlay(win, 'history');
  for (const [value, want] of [['99', '?n=50'], ['3', '?n=5'], ['20', ''], ['', ''], ['12.9', '?n=12']]) {
    typeIn(win, win.$('#ov-count'), value);
    assert.equal(query(win), want, 'n ' + value);
  }
});

test('an option an overlay does not honor never reaches its link, even when it was set on another overlay', async () => {
  const { win } = await openOverlays();
  typeIn(win, win.$('#ov-autohide'), '9');
  await selectOverlay(win, 'history');
  assert.equal(query(win), '');
  await selectOverlay(win, 'now-playing');
  assert.equal(query(win), '?autohide=9');
});

test('language, checkboxes and hidden states build the link, only when they differ from the default', async () => {
  const { win } = await openOverlays();
  const lang = (code) => choose(win, win.$('input[name="ov-lang"][value="' + code + '"]'));
  lang('fr');
  assert.equal(query(win), '?lang=fr');
  lang('auto');
  assert.equal(query(win), '');
  await selectOverlay(win, 'history');
  tick(win, true, checkbox(win, 'Best line'));
  assert.equal(query(win), '?best=1');
  tick(win, false, checkbox(win, 'Best line'));
  assert.equal(query(win), '');
  await selectOverlay(win, 'alert');
  tick(win, true, checkbox(win, 'Play a chime'));
  assert.equal(query(win), '?sound=1');
  await selectOverlay(win, 'keys');
  tick(win, false, checkbox(win, 'Press counters'));
  tick(win, false, checkbox(win, 'Peak keys per second'));
  assert.equal(query(win), '?counts=0&peak=0');
  tick(win, true, checkbox(win, 'Press counters'));
  assert.equal(query(win), '?peak=0');
  await selectOverlay(win, 'state');
  const state = (label) => win.$$('.check-list .check').filter((l) => text(l) === label)[0].querySelector('input');
  tick(win, true, state('In game'));
  assert.equal(query(win), '?hide=gameplay');
  tick(win, false, state('In game'));
  assert.equal(query(win), '');
  assert.equal(win.$$('.check-list .check').length, 15);
});

test('the theme picker lists known themes first, marks the default and custom ones, and a choice shows in the link and preview', async () => {
  const { win } = await openOverlays();
  const cards = themeCards(win);
  assert.deepEqual(cards.map((c) => text(c.querySelector('.theme-name'))), ['Neutral Light', 'Pixel', 'zen']);
  assert.match(text(cards[0]), /Default/);
  assert.match(text(cards[2]), /Custom/);
  assert.equal(cards[2].querySelectorAll('.theme-swatch.is-empty').length, 3);
  assert.equal(cards[1].querySelector('.theme-swatch').style.getPropertyValue('--sw'), '#000000');
  assert.equal(cards[0].querySelector('.theme-swatches').getAttribute('aria-hidden'), 'true');
  assert.ok(themeRadio(win, 'neutral-light').hasAttribute('checked'));
  assert.ok(win.$('fieldset.theme-picker legend'));
  choose(win, themeRadio(win, 'pixel'));
  assert.equal(link(win), ORIGIN + '/overlays/now-playing/?theme=pixel');
  await settle(win, 250);
  assert.equal(frame(win).getAttribute('src'), '/overlays/now-playing/?theme=pixel&demo=1');
  choose(win, themeRadio(win, 'neutral-light'));
  assert.equal(link(win), ORIGIN + '/overlays/now-playing/');
  themeRadio(win, 'pixel').checked = false;
  fire(win, themeRadio(win, 'pixel'), 'change');
  assert.equal(link(win), ORIGIN + '/overlays/now-playing/');
  assert.ok(text(win.$('.controls .hint code')).includes('C:/themes'));
});

test('a saved theme that is not installed stays selectable and an empty config falls back to neutral-light', async () => {
  const gone = await openOverlays({ 'GET /api/config': () => H.json({ config: { overlay: { theme: 'my-theme', accent: '#ABC' } } }) });
  assert.ok(themeRadio(gone.win, 'my-theme').hasAttribute('checked'));
  assert.equal(gone.win.$('#ov-hex').value, 'abc');
  assert.equal(gone.win.$('#ov-accent').value, '#aabbcc');
  assert.equal(link(gone.win), ORIGIN + '/overlays/now-playing/');
  const bare = await openOverlays({ 'GET /api/config': () => H.json({}), 'GET /api/themes': () => H.json({ not: 'a list' }) });
  assert.equal(bare.win.$$('input[name="ov-theme"]').length, 1);
  assert.ok(themeRadio(bare.win, 'neutral-light'));
  assert.equal(bare.win.$('.controls .hint code'), null);
});

test('the accent follows the hex field and the color picker, and an invalid value is flagged and ignored', async () => {
  const { win } = await openOverlays();
  const hex = win.$('#ov-hex');
  typeIn(win, hex, '#A855F7');
  assert.equal(query(win), '?accent=a855f7');
  assert.equal(hex.getAttribute('aria-invalid'), 'false');
  typeIn(win, hex, ' #abc ');
  assert.equal(win.$('#ov-accent').value, '#aabbcc');
  typeIn(win, hex, 'zzz');
  assert.equal(hex.getAttribute('aria-invalid'), 'true');
  assert.equal(text(win.$('#ov-hex-error')), 'Not a valid color. Use 3, 4, 6 or 8 hex digits.');
  assert.equal(query(win), '?accent=abc');
  typeIn(win, hex, '');
  assert.equal(hex.getAttribute('aria-invalid'), 'false');
  assert.equal(text(win.$('#ov-hex-error')), '');
  assert.equal(query(win), '');
  assert.equal(win.$('#ov-accent').value, '#a855f7');
  typeIn(win, win.$('#ov-accent'), '#00FF88');
  assert.equal(hex.value, '00ff88');
  assert.equal(query(win), '?accent=00ff88');
  typeIn(win, hex, '12345678');
  assert.equal(win.$('#ov-accent').value, '#123456');
  assert.match(hex.getAttribute('aria-describedby'), /ov-hex-hint/);
});

test('copy link: success relabels the button, announces once and goes back after 2 seconds', async () => {
  const { win } = await openOverlays();
  copyButton(win).click();
  await win.flush();
  assert.deepEqual(win.clipboard.written, [ORIGIN + '/overlays/now-playing/']);
  assert.equal(text(copyButton(win)), 'Copied');
  assert.equal(linkStatus(win), '');
  await settle(win, 16);
  assert.equal(linkStatus(win), 'Link copied to the clipboard.');
  await settle(win, 1900);
  assert.equal(text(copyButton(win)), 'Copied');
  await settle(win, 100);
  assert.equal(text(copyButton(win)), 'Copy');
  choose(win, themeRadio(win, 'pixel'));
  assert.equal(linkStatus(win), '');
});

test('copy link: a refused clipboard or a browser without one selects the link and says to press Ctrl+C', async () => {
  const denied = await openOverlays();
  denied.win.clipboard.fail = true;
  copyButton(denied.win).click();
  await denied.win.flush();
  assert.equal(denied.win.document.activeElement, denied.win.$('#ov-url'));
  assert.equal(linkStatus(denied.win), 'Could not copy automatically. The link is selected: press Ctrl+C.');
  assert.equal(text(copyButton(denied.win)), 'Copy');
  const none = await openOverlays();
  none.win.navigator.clipboard = undefined;
  copyButton(none.win).click();
  assert.equal(none.win.document.activeElement, none.win.$('#ov-url'));
  assert.match(linkStatus(none.win), /press Ctrl\+C/);
});

test('copy size: success, refusal and no clipboard each say what to do', async () => {
  const { win } = await openOverlays();
  const button = win.$('.size-row .btn');
  assert.equal(button.getAttribute('aria-label'), 'Copy the recommended size');
  assert.equal(button.getAttribute('aria-describedby'), 'ov-size');
  button.click();
  await win.flush();
  assert.deepEqual(win.clipboard.written, ['540 x 140']);
  assert.equal(sizeStatus(win), 'Size copied to the clipboard.');
  await selectOverlay(win, 'state');
  assert.equal(sizeStatus(win), '');
  win.clipboard.fail = true;
  button.click();
  await win.flush();
  assert.equal(sizeStatus(win), 'Could not copy automatically. Type 280 x 64 in the OBS width and height fields.');
  win.navigator.clipboard = {};
  button.click();
  assert.match(sizeStatus(win), /Type 280 x 64/);
});

test('save as default reads the config, sends it back with the new look and keeps every other setting', async () => {
  const { win, puts } = await openOverlays();
  choose(win, themeRadio(win, 'pixel'));
  typeIn(win, win.$('#ov-hex'), '#FF00FF');
  saveButton(win).click();
  assert.equal(saveButton(win).disabled, true);
  assert.equal(saveButton(win).getAttribute('aria-busy'), 'true');
  assert.equal(text(saveStatus(win)), 'Saving...');
  await win.flush();
  assert.deepEqual(puts, [{ language: '', port: 7878, overlay: { theme: 'pixel', accent: 'ff00ff', extra: 1 } }]);
  const put = win.fetchCalls.find((c) => c.method === 'PUT');
  assert.equal(put.init.headers['X-Requested-With'], 'qsc');
  assert.equal(text(saveStatus(win)), 'Saved as your default look.');
  assert.equal(saveStatus(win).className, 'save-status ok');
  assert.equal(saveButton(win).disabled, false);
  assert.equal(saveButton(win).hasAttribute('aria-busy'), false);
  assert.equal(query(win), '');
  const cards = themeCards(win);
  assert.match(text(cards.find((c) => c.querySelector('[value="pixel"]'))), /Default/);
  assert.doesNotMatch(text(cards.find((c) => c.querySelector('[value="neutral-light"]'))), /Default/);
  choose(win, themeRadio(win, 'zen'));
  assert.equal(text(saveStatus(win)), '');
});

test('save as default adds a missing overlay section, and an invalid accent blocks it without any request', async () => {
  const { win, puts } = await openOverlays({ 'GET /api/config': () => H.json({ config: { port: 1 } }) });
  saveButton(win).click();
  await win.flush();
  assert.deepEqual(puts, [{ port: 1, overlay: { theme: 'neutral-light', accent: '' } }]);
  typeIn(win, win.$('#ov-hex'), 'nope');
  const calls = win.fetchCalls.length;
  saveButton(win).click();
  await win.flush();
  assert.equal(win.fetchCalls.length, calls);
  assert.equal(win.document.activeElement, win.$('#ov-hex'));
});

test('save errors: the server message is translated and tied to its field, a plain failure is shown too', async () => {
  const rejected = await openOverlays({ 'PUT /api/config': () => H.json({ error: 'unknown theme', field: 'overlay.theme' }, 400) });
  saveButton(rejected.win).click();
  await rejected.win.flush();
  assert.equal(text(saveStatus(rejected.win)), 'Could not save: overlay.theme: ' + H.en['error.theme']);
  assert.equal(saveStatus(rejected.win).className, 'save-status bad');
  assert.equal(saveButton(rejected.win).disabled, false);
  assert.equal(reloadButton(rejected.win).hidden, true);
  const down = await openOverlays({ 'PUT /api/config': () => H.json({ error: 'boom' }, 500) });
  saveButton(down.win).click();
  await down.win.flush();
  assert.equal(text(saveStatus(down.win)), 'Could not save: ' + H.t('error.server', { status: 500 }));
  const offline = await openOverlays({ 'PUT /api/config': () => new Error('offline') });
  saveButton(offline.win).click();
  await offline.win.flush();
  assert.equal(text(saveStatus(offline.win)), 'Could not save: ' + H.en['error.network']);
});

test('a save that times out before the write is a plain failure, after the write it offers to reload', async () => {
  const early = await openOverlays();
  early.win.routes['GET /api/config'] = never;
  saveButton(early.win).click();
  await early.win.advance(8000);
  assert.equal(text(saveStatus(early.win)), 'Could not save: The companion did not answer in time.');
  assert.equal(reloadButton(early.win).hidden, true);

  const { win } = await openOverlays({ 'PUT /api/config': never });
  choose(win, themeRadio(win, 'pixel'));
  saveButton(win).click();
  await win.flush();
  await win.advance(8000);
  assert.equal(text(saveStatus(win)), 'The companion did not answer in time. Your look may or may not have been saved.');
  assert.equal(reloadButton(win).hidden, false);
  assert.equal(saveButton(win).disabled, false);
  win.routes['GET /api/config'] = () => H.json({ config: { overlay: { theme: 'pixel', accent: '#AABBCC' } } });
  reloadButton(win).click();
  assert.equal(reloadButton(win).disabled, true);
  await win.flush();
  assert.equal(text(saveStatus(win)), 'Reloaded the saved settings.');
  assert.equal(reloadButton(win).hidden, true);
  assert.equal(reloadButton(win).disabled, false);
  assert.equal(query(win), '');
  assert.match(text(themeCards(win).find((c) => c.querySelector('[value="pixel"]'))), /Default/);
});

test('reload saved settings: an empty answer falls back to the default look and a failure is shown', async () => {
  const { win } = await openOverlays({ 'PUT /api/config': never });
  saveButton(win).click();
  await win.flush();
  await win.advance(8000);
  win.routes['GET /api/config'] = () => H.json({ config: {} });
  reloadButton(win).click();
  await win.flush();
  assert.equal(text(saveStatus(win)), 'Reloaded the saved settings.');
  assert.equal(query(win), '');
  saveButton(win).click();
  await win.flush();
  await win.advance(8000);
  win.routes['GET /api/config'] = () => H.json({ error: 'x' }, 500);
  reloadButton(win).click();
  await win.flush();
  assert.equal(text(saveStatus(win)), 'Could not save: ' + H.t('error.server', { status: 500 }));
  assert.equal(reloadButton(win).disabled, false);
});

test('a load failure shows the message and a retry button that reloads and puts focus in the theme picker', async () => {
  let fail = true;
  const { win } = await openOverlays({ 'GET /api/themes': () => (fail ? H.json({}, 500) : H.json(THEMES)) });
  assert.equal(text(win.$('.empty .save-status')), 'Could not load the overlay settings: ' + H.t('error.server', { status: 500 }));
  assert.equal(win.$('.empty .save-status').getAttribute('role'), 'alert');
  fail = false;
  win.$('.empty .btn').click();
  assert.equal(text(win.$('.overlays-body')), 'Loading...');
  await win.flush();
  assert.equal(win.$('.empty'), null);
  assert.equal(win.document.activeElement, themeRadio(win, 'neutral-light'));
});

test('a broken list of custom overlays does not stop the page', async () => {
  const { win } = await openOverlays({ 'GET /api/overlays': () => H.json({}, 500) });
  assert.equal(win.$$('input[name="ov-overlay"]').length, 8);
  assert.equal(win.$('.controls .hint code').textContent.includes('C:/themes'), true);
  const odd = await openOverlays({ 'GET /api/overlays': () => H.json({ dir: 5, custom: 'x' }) });
  assert.equal(odd.win.$$('input[name="ov-overlay"]').length, 8);
});

test('custom overlays are cards with the folder help, a preview sized from the manifest and no built-in options', async () => {
  const custom = [
    { id: 'mine', name: 'My overlay', description: 'Does things', width: 800, height: 300 },
    { id: 'tall', name: '  Tall\u0007one  ', width: 99999, height: 2000 },
    { id: 'now-playing', name: 'Clash' }, { id: 'mine', name: 'Twice' }, { id: 'Bad Id', name: 'x' }, null,
  ];
  const { win } = await openOverlays({ 'GET /api/overlays': () => H.json({ dir: 'C:/overlays', custom }) });
  assert.equal(win.$$('input[name="ov-overlay"]').length, 10);
  assert.match(text(win.$('.overlay-card > .hint')), /Put a folder with an index.html file/);
  assert.match(text(win.$('.overlay-card > .hint code')), /C:\/overlays/);
  const card = overlayRadio(win, 'mine').closest('label');
  assert.match(text(card), /Custom/);
  assert.match(text(card), /800 x 300/);
  await selectOverlay(win, 'mine');
  assert.equal(text(win.$('#ov-name')), 'My overlay');
  assert.equal(link(win), ORIGIN + '/overlays/mine/');
  assert.equal(frame(win).getAttribute('src'), '/overlays/mine/?theme=neutral-light');
  assert.equal(frame(win).className, 'preview-frame');
  assert.equal(frame(win).style.height, '320px');
  assert.equal(win.$$('.overlay-options .field').every((f) => f.hidden), true);
  await selectOverlay(win, 'tall');
  assert.equal(text(win.$('#ov-name')), 'Tall one');
  assert.equal(text(win.$('#ov-size')), '3840 x 2000');
  assert.equal(frame(win).style.height, '480px');
  await selectOverlay(win, 'history');
  assert.equal(frame(win).style.height, '');
});

test('custom overlay names and descriptions from a hostile manifest are shown as text, never as markup', async () => {
  const custom = [{ id: 'evil', name: HOSTILE, description: '<script>alert(1)</script>', width: 500, height: 100 }];
  const { win } = await openOverlays({ 'GET /api/overlays': () => H.json({ dir: HOSTILE, custom }) });
  await selectOverlay(win, 'evil');
  assert.equal(text(win.$('#ov-name')), HOSTILE);
  assert.equal(text(win.$('.overlay-card > p.muted:not(.hint)')), '<script>alert(1)</script>');
  assert.equal(win.$('.overlay-card').querySelector('img'), null);
  assert.equal(win.$('.overlay-card').querySelector('script'), null);
  assert.equal(frame(win).getAttribute('title'), 'Preview of the ' + HOSTILE + ' overlay with example data');
  assert.equal(copyButton(win).getAttribute('aria-label'), 'Copy the ' + HOSTILE + ' overlay link');
  assert.ok(text(win.$('.overlay-card > .hint code')).includes(HOSTILE));
});

test('hostile theme ids and colors from the server are dropped or blanked', async () => {
  const themes = [{ id: HOSTILE, colors: {} }, { id: 'ok-theme', colors: { bg: 'red; background: url(x)', fg: '#fff', accent: 5 } }];
  const { win } = await openOverlays({ 'GET /api/themes': () => H.json(themes) });
  assert.equal(win.$('.overlay-card').querySelector('img'), null);
  const swatches = themeRadio(win, 'ok-theme').closest('label').querySelectorAll('.theme-swatch');
  assert.deepEqual(swatches.map((s) => s.style.getPropertyValue('--sw')), ['', '#fff', '']);
  assert.equal(win.$$('input[name="ov-theme"]').length, 2);
});

test('leaving the page while it loads or saves ignores the late answers and aborts the requests', async () => {
  const slow = H.deferred();
  const loading = setup({ 'GET /api/themes': () => slow.promise });
  const win = await H.open({ hash: '#/overlays', routes: loading.routes });
  assert.equal(text(win.$('.overlays-body')), 'Loading...');
  win.location.hash = '#/status';
  await win.flush();
  slow.resolve(H.json(THEMES));
  await win.flush();
  assert.equal(win.$('.overlay-card'), null);
  assert.equal(win.logs.filter((l) => l.level === 'error').length, 0);

  const gate = H.deferred();
  const saving = await openOverlays({ 'PUT /api/config': () => gate.promise });
  saveButton(saving.win).click();
  await saving.win.flush();
  saving.win.location.hash = '#/status';
  await saving.win.flush();
  gate.resolve(H.json({}));
  await saving.win.flush();
  assert.equal(saving.win.$('.overlay-card'), null);

  const stuck = await openOverlays({ 'PUT /api/config': never });
  saveButton(stuck.win).click();
  await stuck.win.flush();
  stuck.win.location.hash = '#/status';
  await stuck.win.flush();
  await stuck.win.advance(9000);
  assert.equal(stuck.win.$('.overlay-card'), null);
});

test('leaving the page during a reload or after a failed load ignores the late answers', async () => {
  const { win } = await openOverlays({ 'PUT /api/config': never });
  saveButton(win).click();
  await win.flush();
  await win.advance(8000);
  const gate = H.deferred();
  win.routes['GET /api/config'] = () => gate.promise;
  reloadButton(win).click();
  win.location.hash = '#/status';
  await win.flush();
  gate.resolve(H.json({ config: {} }));
  await win.flush();
  assert.equal(win.$('.overlay-card'), null);

  const failing = H.deferred();
  const bad = setup({ 'GET /api/themes': () => failing.promise });
  const w2 = await H.open({ hash: '#/overlays', routes: bad.routes });
  w2.location.hash = '#/status';
  await w2.flush();
  failing.resolve(H.json({}, 500));
  await w2.flush();
  assert.equal(w2.$('.empty'), null);
});

test('a preview or copy timer firing after the page is closed does nothing, and the page opens again afterwards', async () => {
  const { win } = await openOverlays();
  copyButton(win).click();
  await win.flush();
  choose(win, overlayRadio(win, 'result'));
  win.location.hash = '#/status';
  await win.flush();
  await win.advance(3000);
  assert.equal(win.logs.filter((l) => l.level === 'error').length, 0);
  win.location.hash = '#/overlays';
  await win.flush();
  assert.equal(win.$$('input[name="ov-overlay"]').length, 8);
});

run();
