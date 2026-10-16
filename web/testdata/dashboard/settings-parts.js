// settings-keys.js, settings-quit.js and settings-info.js: the lane keys card, the quit confirmation and the paths card.
'use strict';
const assert = require('node:assert/strict');
const { test, run } = require('../../testsupport/run.js');
const dom = require('../../testsupport/dom.js');
const H = require('./helpers.js');
const S = require('./settings-helpers.js');

const keysForm = (win) => win.$$('.settings-form')[1];
const submitKeys = (win) => keysForm(win).dispatchEvent(new dom.Event('submit'));
const keysStatus = (win) => keysForm(win).querySelector('.save-status').textContent;
const keysError = (win, name) => win.$('#set-keys-' + name + '-error').textContent;
const lane = (win, layout, n) => keysForm(win).querySelector('select[aria-label="' + H.t('settings.keys.lane', { layout, n }) + '"]');
const focused = (win) => win.document.activeElement;
const putOf = (state) => state.puts[state.puts.length - 1];

test('keys card: support line follows the status answer', async () => {
  const ok = await S.open();
  assert.equal(keysForm(ok.win).querySelector('.hint[role="status"]').textContent, H.en['settings.keys.status.ok']);
  const again = await S.open({ routes: { '/api/status': () => dom.json({ keys: { supported: false } }) } });
  assert.equal(keysForm(again.win).querySelector('.hint[role="status"]').textContent, H.en['settings.keys.status.unsupported']);
  const bad = await S.open({ routes: { '/api/status': () => dom.json({}, 500) } });
  assert.equal(keysForm(bad.win).querySelector('.hint[role="status"]').textContent, H.en['settings.keys.status.unknown']);
});

test('keys card: unusable config values fall back to the defaults, valid ones are shown', async () => {
  const { win } = await S.open({ config: S.config({ keys: { enabled: true, layout: '7k', scratch: 'F5', k4: ['Q', 'W', 'E', 'R'], k5: ['A', 'A'], k2: 'no' } }) });
  assert.equal(win.$('#set-keys-enabled').checked, true);
  assert.equal(win.$('#set-keys-layout').value, '7k');
  assert.equal(win.$('#set-keys-scratch').value, 'F5');
  assert.deepEqual([1, 2, 3, 4].map((n) => lane(win, '4K', n).value), ['Q', 'W', 'E', 'R']);
  assert.deepEqual([1, 2, 3, 4, 5].map((n) => lane(win, '5K', n).value), ['A', 'S', 'SPACE', 'K', 'L']);
  assert.deepEqual([1, 2].map((n) => lane(win, '2K', n).value), ['A', 'L']);
  assert.equal(keysForm(win).querySelectorAll('fieldset').length, 10);
  const plain = await S.open({ config: S.config({ keys: null }) });
  assert.equal(plain.win.$('#set-keys-enabled').checked, false);
  assert.equal(plain.win.$('#set-keys-layout').value, 'auto');
  assert.equal(plain.win.$('#set-keys-scratch').value, 'SHIFT_L');
});

test('keys card: saving sends only the keys section and keeps the rest of the config', async () => {
  const { win, state } = await S.open({ config: S.config({ keys: { enabled: false, scratch: 'F5' } }) });
  state.config = S.config({ port: 9999, keys: { enabled: false, scratch: 'F5', other: 1 } });
  win.$('#set-keys-enabled').checked = true;
  win.$('#set-keys-layout').value = '4k';
  lane(win, '4K', 1).value = 'Q';
  win.$('#set-keys-scratch').value = 'F6';
  submitKeys(win);
  assert.equal(keysForm(win).querySelector('button').disabled, true);
  await win.flush();
  const body = putOf(state);
  assert.equal(body.port, 9999);
  assert.deepEqual(body.keys.k4, ['Q', 'S', 'K', 'L']);
  assert.deepEqual([body.keys.enabled, body.keys.layout, body.keys.scratch, body.keys.other], [true, '4k', 'F6', 1]);
  assert.equal(Object.keys(body.keys).filter((k) => /^k\d+$/.test(k)).length, 10);
  assert.equal(keysStatus(win), H.en['settings.keys.saved']);
  assert.equal(keysForm(win).querySelector('button').disabled, false);
  assert.equal(keysForm(win).querySelector('.banner').hidden, true);
});

test('keys card: a scratch key never chosen stays out of the saved config', async () => {
  const { win, state } = await S.open();
  submitKeys(win);
  await win.flush();
  assert.equal('scratch' in putOf(state).keys, false);
});

test('keys card: a save that needs a restart shows the banner, the next one hides it', async () => {
  const { win, state } = await S.open({ restart: true });
  submitKeys(win);
  await win.flush();
  assert.equal(keysForm(win).querySelector('.banner').hidden, false);
  state.restart = false;
  submitKeys(win);
  await win.flush();
  assert.equal(keysForm(win).querySelector('.banner').hidden, true);
});

test('keys card: a second submit while saving is ignored', async () => {
  const { win, state } = await S.open();
  const gate = dom.deferred();
  state.putGate = gate.promise;
  submitKeys(win);
  submitKeys(win);
  await win.flush();
  gate.resolve();
  await win.flush();
  assert.equal(state.puts.length, 1);
});

test('keys card: errors land on the right field, open the lists and focus the first control', async () => {
  const { win, state } = await S.open();
  const fail = async (field, error) => {
    state.putError = { status: 400, body: { field, error } };
    submitKeys(win);
    await win.flush();
  };
  await fail('keys.k4', 'must list exactly 4 keys');
  assert.equal(keysError(win, 'k4'), H.t('error.keys_count', { n: 4 }));
  assert.equal(lane(win, '4K', 1).getAttribute('aria-invalid'), 'true');
  assert.equal(lane(win, '4K', 4).getAttribute('aria-invalid'), 'true');
  assert.equal(keysForm(win).querySelector('details').open, true);
  assert.equal(focused(win), lane(win, '4K', 1));
  assert.equal(keysStatus(win), H.en['settings.save_failed_short']);
  await fail('keys.k5', 'entry 3 is not a supported key name');
  assert.equal(keysError(win, 'k4'), '');
  assert.equal(lane(win, '4K', 1).hasAttribute('aria-invalid'), false);
  assert.equal(keysError(win, 'k5'), H.t('error.keys_entry', { n: 3 }));
  await fail('keys.layout', 'must not list the same key twice');
  assert.equal(keysError(win, 'layout'), H.en['error.keys_dup']);
  assert.equal(focused(win), win.$('#set-keys-layout'));
  await fail('keys.scratch', 'must differ from the lane keys of the layout in use');
  assert.equal(keysError(win, 'scratch'), H.en['error.keys_scratch_clash']);
  assert.equal(focused(win), win.$('#set-keys-scratch'));
  assert.equal(keysForm(win).querySelector('button').disabled, false);
});

test('keys card: an unknown field or none goes to the general message', async () => {
  const { win, state } = await S.open();
  const general = () => keysForm(win).querySelectorAll('.field-error')[keysForm(win).querySelectorAll('.field-error').length - 1].textContent;
  state.putError = { status: 400, body: { field: 'port', error: 'must be between 1 and 65535' } };
  submitKeys(win);
  await win.flush();
  assert.equal(general(), 'port: ' + H.en['error.port_range']);
  state.putError = { status: 500, body: {} };
  submitKeys(win);
  await win.flush();
  assert.equal(general(), H.t('error.server', { status: 500 }));
});

test('keys card: leaving the page during a save does not touch the page that replaced it', async () => {
  const { win, state } = await S.open();
  const gate = dom.deferred();
  state.putGate = gate.promise;
  submitKeys(win);
  await win.flush();
  win.location.hash = '#/status';
  await win.flush();
  gate.resolve();
  await win.flush();
  assert.equal(win.logs.length, 0);
});

const quit = (win) => win.$('section[aria-labelledby="set-quit-title"]');
const quitButtons = (win) => quit(win).querySelectorAll('button');
const box = (win) => win.$('#set-quit-confirm');
const quitCalls = (win) => win.fetchCalls.filter((c) => c.path === '/api/quit');

test('quit: the first button asks, focuses Cancel and hides itself', async () => {
  const { win } = await S.open();
  assert.equal(box(win).hidden, true);
  const [open, cancel] = quitButtons(win);
  open.click();
  assert.equal(box(win).hidden, false);
  assert.equal(open.hidden, true);
  assert.equal(focused(win), cancel);
  assert.equal(quitCalls(win).length, 0);
});

test('quit: Cancel and Escape both close the question and give the focus back', async () => {
  const { win } = await S.open();
  const [open, cancel] = quitButtons(win);
  for (const close of [() => cancel.click(), () => box(win).dispatchEvent(new dom.Event('keydown', { key: 'Escape' }))]) {
    open.click();
    close();
    assert.equal(box(win).hidden, true);
    assert.equal(open.hidden, false);
    assert.equal(focused(win), open);
  }
  open.click();
  box(win).dispatchEvent(new dom.Event('keydown', { key: 'Tab' }));
  assert.equal(box(win).hidden, false);
});

test('quit: confirming posts to /api/quit and shows the stopped message with the focus on the title', async () => {
  const { win } = await S.open({ routes: { 'POST /api/quit': () => dom.json({}) } });
  const [open, cancel, now] = quitButtons(win);
  open.click();
  now.click();
  assert.equal(now.disabled, true);
  assert.equal(now.getAttribute('aria-busy'), 'true');
  assert.equal(cancel.disabled, true);
  now.click();
  await win.flush();
  assert.equal(quitCalls(win).length, 1);
  assert.equal(quitCalls(win)[0].init.headers['X-Requested-With'], 'qsc');
  assert.equal(quit(win).querySelector('.save-status').textContent, H.en['settings.quit.stopped']);
  assert.equal(quit(win).querySelector('.save-status').className, 'save-status ok');
  assert.equal(box(win).hidden, true);
  assert.equal(open.parent.hidden, true);
  assert.equal(focused(win), win.$('#set-quit-title'));
});

test('quit: Escape does nothing while qsc is stopping', async () => {
  const gate = dom.deferred();
  const { win } = await S.open({ routes: { 'POST /api/quit': () => gate.promise } });
  const [open, , now] = quitButtons(win);
  open.click();
  now.click();
  box(win).dispatchEvent(new dom.Event('keydown', { key: 'Escape' }));
  assert.equal(box(win).hidden, false);
  assert.equal(quit(win).querySelector('.save-status').textContent, H.en['settings.quit.stopping']);
  gate.resolve(dom.json({}));
  await win.flush();
  assert.equal(quit(win).querySelector('.save-status').textContent, H.en['settings.quit.stopped']);
});

test('quit: the connection closing while it stops counts as stopped', async () => {
  const { win } = await S.open({ routes: { 'POST /api/quit': () => new TypeError('Failed to fetch') } });
  const [open, , now] = quitButtons(win);
  open.click();
  now.click();
  await win.flush();
  assert.equal(quit(win).querySelector('.save-status').textContent, H.en['settings.quit.stopped']);
});

test('quit: a server error is shown, the buttons work again and the focus goes to Quit now', async () => {
  const { win } = await S.open({ routes: { 'POST /api/quit': () => dom.json({}, 500) } });
  const [open, cancel, now] = quitButtons(win);
  open.click();
  now.click();
  await win.flush();
  assert.equal(quit(win).querySelector('.field-error').textContent, H.t('settings.quit.failed', { message: H.t('error.server', { status: 500 }) }));
  assert.equal(quit(win).querySelector('.save-status').textContent, '');
  assert.equal(now.disabled, false);
  assert.equal(cancel.disabled, false);
  assert.equal(now.hasAttribute('aria-busy'), false);
  assert.equal(focused(win), now);
  assert.equal(box(win).hidden, false);
  cancel.click();
  assert.equal(quit(win).querySelector('.field-error').textContent, '');
  open.click();
  assert.equal(quit(win).querySelector('.field-error').textContent, '');
});

test('quit: no answer in time is a failure, not a stop', async () => {
  const { win } = await S.open({ routes: { 'POST /api/quit': () => new Promise(() => {}) } });
  const [open, , now] = quitButtons(win);
  open.click();
  now.click();
  await win.advance(8000);
  assert.equal(quit(win).querySelector('.field-error').textContent, H.t('settings.quit.failed', { message: H.en['settings.timeout'] }));
  assert.equal(now.disabled, false);
});

test('quit: an answer after leaving the page changes nothing', async () => {
  const gate = dom.deferred();
  const { win } = await S.open({ routes: { 'POST /api/quit': () => gate.promise } });
  const [open, , now] = quitButtons(win);
  const title = win.$('#set-quit-title');
  open.click();
  now.click();
  win.location.hash = '#/status';
  await win.flush();
  gate.resolve(dom.json({}));
  await win.flush();
  assert.notEqual(focused(win), title);
  assert.equal(win.logs.length, 0);
});

const info = (win) => win.$('section[aria-labelledby="set-info-title"]');
const infoLive = (win) => info(win).querySelector('.status-line');

test('about card: one row per known path and the version with the releases link', async () => {
  const { win } = await S.open();
  assert.deepEqual(info(win).querySelectorAll('input').map((i) => i.id), ['set-path-config', 'set-path-themes']);
  assert.equal(win.$('#set-path-config').value, H.CFG);
  assert.equal(win.$('#set-path-config').getAttribute('aria-label'), H.en['settings.config_file']);
  assert.equal(info(win).querySelectorAll('button')[0].getAttribute('aria-label'), H.t('settings.copy_aria', { name: H.en['settings.config_file'] }));
  const link = info(win).querySelector('a');
  assert.equal(link.getAttribute('rel'), 'noopener noreferrer');
  assert.equal(link.getAttribute('target'), '_blank');
  assert.match(link.getAttribute('href'), /^https:\/\/github\.com\/kushiemoon-dev\/quaver-stream-companion\/releases$/);
  assert.match(info(win).textContent, /1\.2\.3/);
});

test('about card: no version and no paths leave an empty list', async () => {
  const bare = await S.open({ routes: { 'GET /api/config': () => dom.json({ config: S.config(), config_path: 5, themes_dir: '' }) } });
  assert.equal(info(bare.win).querySelectorAll('input').length, 0);
  assert.equal(info(bare.win).querySelectorAll('a').length, 0);
});

test('about card: copy writes the path, says copied for 1.5 seconds and announces it', async () => {
  const { win } = await S.open();
  const button = info(win).querySelectorAll('button')[0];
  button.click();
  await win.flush();
  assert.deepEqual(win.clipboard.written, [H.CFG]);
  assert.equal(button.textContent, H.en['settings.copied']);
  assert.equal(infoLive(win).textContent, '');
  await win.advance(16);
  assert.equal(infoLive(win).textContent, H.t('settings.copied_live', { name: H.en['settings.config_file'] }));
  await win.advance(1500);
  assert.equal(button.textContent, H.en['settings.copy']);
});

test('about card: when copying fails the field is selected and the user is told to copy it', async () => {
  const { win } = await S.open();
  win.clipboard.fail = true;
  info(win).querySelectorAll('button')[1].click();
  await win.flush();
  assert.equal(focused(win), win.$('#set-path-themes'));
  assert.equal(infoLive(win).textContent, H.en['settings.copy_manual']);
  assert.equal(info(win).querySelectorAll('button')[1].textContent, H.en['settings.copy']);
});

run();
