// pages/settings.js and settings-model.js: the settings form, its validation, saving and the restart banner.
'use strict';
const assert = require('node:assert/strict');
const { test, run } = require('../../testsupport/run.js');
const dom = require('../../testsupport/dom.js');
const H = require('./helpers.js');
const S = require('./settings-helpers.js');

const dir = (win) => win.$('#set-dir');
const port = (win) => win.$('#set-port');
const lang = (win) => win.$('#set-lang');
const watch = (win) => win.$('#set-watch');
const text = (win) => win.$('#set-text');
const form = (win) => win.$('.settings-form');
const submit = (win) => form(win).dispatchEvent(new dom.Event('submit'));
const status = (win) => form(win).querySelector('.save-status').textContent;
const banner = (win) => form(win).querySelector('.banner');
const errorOf = (win, id) => win.$('#' + id + '-error').textContent;
const focused = (win) => win.document.activeElement;
const reloadButton = (win) => form(win).querySelectorAll('button')[1];

test('the form shows the saved values, the effective folder and no banner', async () => {
  const { win } = await S.open();
  assert.equal(dir(win).value, S.QDIR);
  assert.equal(port(win).value, '7878');
  assert.equal(lang(win).value, '');
  assert.equal(watch(win).checked, true);
  assert.equal(text(win).checked, false);
  assert.equal(text(win).getAttribute('aria-describedby'), 'set-text-help');
  assert.equal(win.$('#set-text-help').textContent, H.en['settings.text_help']);
  assert.equal(banner(win).hidden, true);
  assert.equal(win.$('#set-dir-help').textContent, H.en['settings.dir_effective'] + ' ' + S.QDIR);
  assert.equal(win.$('#set-port-note').hidden, true);
  assert.equal(win.$$('.settings-form').length, 2);
  assert.equal(lang(win).children.length, 4);
  assert.equal(dir(win).getAttribute('aria-describedby'), 'set-dir-help set-dir-error');
});

test('without an effective folder the help says none was found', async () => {
  const { win } = await S.open({ effective: { quaver_dir: '', port: 7878 } });
  assert.equal(win.$('#set-dir-help').textContent, H.en['settings.dir_none']);
});

test('a language outside the list falls back to automatic and a v0.1.0 config watches Quaver', async () => {
  const { win } = await S.open({ config: S.config({ language: 'xx', watch_quaver_process: undefined }) });
  assert.equal(lang(win).value, '');
  assert.equal(watch(win).checked, true);
  const off = await S.open({ config: S.config({ watch_quaver_process: false, text_files: true }) });
  assert.equal(watch(off.win).checked, false);
  assert.equal(text(off.win).checked, true);
});

test('a field set by a command-line flag is locked and shows the effective value', async () => {
  const { win, state } = await S.open({ overridden: ['port', 'quaver_dir'], effective: { quaver_dir: 'C:/flag', port: 9000 } });
  assert.equal(port(win).value, '9000');
  assert.equal(dir(win).value, 'C:/flag');
  assert.equal(port(win).disabled, true);
  assert.equal(lang(win).disabled, false);
  assert.equal(win.$$('#set-port-note')[1].hidden, false);
  assert.equal(port(win).getAttribute('aria-describedby'), 'set-port-help set-port-note set-port-error');
  assert.equal(dir(win).getAttribute('aria-describedby'), 'set-dir-help set-dir-note set-dir-error');
  submit(win);
  await win.flush();
  assert.equal(state.puts.length, 1);
  assert.equal(state.puts[0].port, 7878);
  assert.equal(state.puts[0].quaver_dir, S.QDIR);
});

test('an invalid port shows the error, focuses the field and sends nothing', async () => {
  const { win, state } = await S.open();
  for (const bad of ['abc', '0', '70000', '12.5', '', '123456']) {
    port(win).value = bad;
    submit(win);
    await win.advance(16);
    assert.equal(errorOf(win, 'set-port'), H.en['settings.port_invalid'], bad);
    assert.equal(port(win).getAttribute('aria-invalid'), 'true');
    assert.equal(focused(win), port(win));
    assert.equal(status(win), H.en['settings.save_failed_short']);
  }
  assert.equal(state.puts.length, 0);
  port(win).value = ' 8080 ';
  submit(win);
  await win.flush();
  assert.equal(errorOf(win, 'set-port'), '');
  assert.equal(port(win).hasAttribute('aria-invalid'), false);
  assert.equal(state.puts[0].port, 8080);
});

test('the OBS note shows only for a valid port that differs from the running one', async () => {
  const { win } = await S.open();
  const note = win.$('#set-port-note');
  for (const [value, hidden] of [['9000', false], ['7878', true], ['nope', true], ['65535', false]]) {
    port(win).value = value;
    port(win).dispatchEvent(new dom.Event('input'));
    assert.equal(note.hidden, hidden, value);
  }
});

test('saving reads the current config first and changes only this form, trimming the folder', async () => {
  const { win, state } = await S.open();
  state.config = S.config({ overlay: { theme: 'night' }, keys: { enabled: true }, extra: 1 });
  dir(win).value = '  C:/Other  ';
  port(win).value = '8000';
  lang(win).value = 'de';
  watch(win).checked = false;
  text(win).checked = true;
  const gate = dom.deferred();
  state.putGate = gate.promise;
  submit(win);
  assert.equal(form(win).querySelector('button').disabled, true);
  assert.equal(form(win).querySelector('button').getAttribute('aria-busy'), 'true');
  await win.flush();
  await win.advance(16);
  assert.equal(status(win), H.en['settings.saving']);
  gate.resolve();
  await win.flush();
  assert.deepEqual(state.puts[0], { quaver_dir: 'C:/Other', port: 8000, language: 'de', watch_quaver_process: false, text_files: true, overlay: { theme: 'night' }, keys: { enabled: true }, extra: 1 });
  assert.equal(win.fetchCalls.filter((c) => c.method === 'PUT')[0].init.headers['X-Requested-With'], 'qsc');
});

test('after a save the button works again, the focus returns to it and the status says saved', async () => {
  const { win } = await S.open();
  win.document.activeElement = win.document.body;
  submit(win);
  await win.flush();
  await win.advance(16);
  const save = form(win).querySelector('button');
  assert.equal(save.disabled, false);
  assert.equal(save.hasAttribute('aria-busy'), false);
  assert.equal(status(win), H.en['settings.saved']);
  assert.equal(form(win).querySelector('.save-status').className, 'save-status ok');
  assert.equal(focused(win), save);
});

test('the focus stays where the user moved it during the save', async () => {
  const { win } = await S.open();
  submit(win);
  dir(win).focus();
  await win.flush();
  assert.equal(focused(win), dir(win));
});

test('a save that needs a restart shows the banner and says so in the status', async () => {
  const { win } = await S.open({ restart: true });
  submit(win);
  await win.flush();
  await win.advance(16);
  assert.equal(banner(win).hidden, false);
  assert.equal(status(win), [H.en['settings.saved'], H.en['settings.restart_title'], H.en['settings.restart_body']].join(' '));
  submit(win);
  await win.flush();
  await win.advance(16);
  assert.equal(status(win), H.en['settings.saved']);
});

test('a second submit while saving is ignored', async () => {
  const { win, state } = await S.open();
  const gate = dom.deferred();
  state.getGate = gate.promise;
  submit(win);
  submit(win);
  await win.flush();
  gate.resolve();
  await win.flush();
  assert.equal(state.puts.length, 1);
});

test('changing the language saves, loads it, redraws the page and confirms in the new language', async () => {
  const { win, state } = await S.open();
  lang(win).value = 'fr';
  submit(win);
  await win.flush();
  await win.flush();
  await win.advance(16);
  const fr = H.catalog('fr');
  assert.equal(state.puts[0].language, 'fr');
  assert.equal(win.document.documentElement.lang, 'fr');
  assert.equal(win.$('h1').textContent, fr['nav.settings']);
  assert.equal(status(win), fr['settings.saved']);
  assert.equal(focused(win), form(win).querySelector('button'));
  assert.equal(lang(win).value, 'fr');
});

test('a field error from the server is translated, shown on the field and focused', async () => {
  const { win, state } = await S.open();
  state.putError = { status: 400, body: { error: 'must be a folder containing Quaver or Quaver.exe', field: 'quaver_dir' } };
  submit(win);
  await win.flush();
  await win.advance(16);
  assert.equal(errorOf(win, 'set-dir'), H.en['error.dir_not_quaver']);
  assert.equal(dir(win).getAttribute('aria-invalid'), 'true');
  assert.equal(focused(win), dir(win));
  assert.equal(status(win), H.en['settings.save_failed_short']);
  assert.equal(form(win).querySelector('button').disabled, false);
  state.putError = null;
  submit(win);
  await win.flush();
  assert.equal(errorOf(win, 'set-dir'), '');
  assert.equal(dir(win).hasAttribute('aria-invalid'), false);
});

test('an error on another field or none goes to the general message', async () => {
  const { win, state } = await S.open();
  const general = () => form(win).querySelectorAll('.field-error')[3].textContent;
  state.putError = { status: 400, body: { error: 'unknown theme', field: 'overlay.theme' } };
  submit(win);
  await win.flush();
  assert.equal(general(), 'overlay.theme: ' + H.en['error.theme']);
  state.putError = { status: 500, body: {} };
  submit(win);
  await win.flush();
  assert.equal(general(), H.t('error.server', { status: 500 }));
  state.putError = { status: 503, body: { error: 'Quaver is writing the database, try again' } };
  submit(win);
  await win.flush();
  assert.equal(general(), H.en['error.busy']);
});

test('a failed read before the save shows the error and unlocks the form', async () => {
  const { win, state } = await S.open();
  state.getAfterLoad = { status: 500, body: {} };
  submit(win);
  await win.flush();
  assert.equal(state.puts.length, 0);
  assert.equal(form(win).querySelector('button').disabled, false);
  assert.equal(form(win).querySelectorAll('.field-error')[3].textContent, H.t('error.server', { status: 500 }));
});

test('no answer to the read gives the timeout message and no reload button', async () => {
  const { win, state } = await S.open();
  state.getGate = new Promise(() => {});
  submit(win);
  await win.flush();
  await win.advance(8000);
  assert.equal(form(win).querySelectorAll('.field-error')[3].textContent, H.en['settings.timeout']);
  assert.equal(reloadButton(win).hidden, true);
  assert.equal(form(win).querySelector('button').disabled, false);
});

test('no answer to the save says it may or may not have been saved and offers a reload', async () => {
  const { win, state } = await S.open();
  state.putGate = new Promise(() => {});
  submit(win);
  await win.flush();
  await win.advance(8000);
  await win.advance(16);
  assert.equal(status(win), H.en['settings.save_unknown']);
  assert.equal(reloadButton(win).hidden, false);
  assert.equal(form(win).querySelector('button').disabled, false);
  submit(win);
  await win.flush();
  assert.equal(reloadButton(win).hidden, true);
});

test('reload brings back the saved values, hides the button and focuses save', async () => {
  const { win, state } = await S.open();
  state.putGate = new Promise(() => {});
  submit(win);
  await win.flush();
  await win.advance(8000);
  state.config = S.config({ port: 7000 });
  state.effective = { quaver_dir: S.QDIR, port: 7000 };
  port(win).value = '1234';
  reloadButton(win).click();
  assert.equal(reloadButton(win).disabled, true);
  await win.flush();
  await win.advance(16);
  assert.equal(port(win).value, '7000');
  assert.equal(reloadButton(win).hidden, true);
  assert.equal(status(win), H.en['settings.reloaded']);
  assert.equal(focused(win), form(win).querySelector('button'));
});

test('a failed reload says why and keeps the focus on the reload button', async () => {
  const { win, state } = await S.open();
  state.putGate = new Promise(() => {});
  submit(win);
  await win.flush();
  await win.advance(8000);
  state.getAfterLoad = { status: 500, body: {} };
  reloadButton(win).click();
  await win.flush();
  await win.advance(16);
  assert.equal(status(win), H.t('settings.load_failed', { message: H.t('error.server', { status: 500 }) }));
  assert.equal(focused(win), reloadButton(win));
  assert.equal(reloadButton(win).disabled, false);
});

test('a failed first load offers a retry that focuses the folder field', async () => {
  const { win, state } = await S.open({ failFirst: true });
  assert.match(win.$('.save-status').textContent, /Could not load the settings/);
  assert.equal(win.$$('.settings-form').length, 0);
  assert.equal(focused(win).textContent, H.en['settings.retry']);
  state.getAfterLoad = null;
  focused(win).click();
  await win.flush();
  assert.equal(win.$$('.settings-form').length, 2);
  assert.equal(focused(win), dir(win));
});

test('leaving the page while a request is open changes nothing and logs nothing', async () => {
  const { win, state } = await S.open();
  const gate = dom.deferred();
  state.putGate = gate.promise;
  submit(win);
  await win.flush();
  win.location.hash = '#/history';
  await win.flush();
  gate.resolve();
  await win.flush();
  await win.advance(8000);
  assert.equal(win.$('h1').textContent, H.en['nav.history']);
  assert.equal(win.logs.length, 0);
});

test('leaving the page before the first load ends shows nothing of the settings', async () => {
  const gate = dom.deferred();
  const { win } = await S.open({ firstGate: gate.promise });
  win.location.hash = '#/history';
  await win.flush();
  gate.resolve();
  await win.flush();
  assert.equal(win.$$('.settings-form').length, 0);
  assert.equal(win.$('h1').textContent, H.en['nav.history']);
});

test('the model rejects bad ports and builds a body without touching the current config', () => {
  const m = require('node:path').join(H.WEB, 'dashboard', 'pages', 'settings-model.js');
  const model = require(m);
  assert.deepEqual(model.validatePort(8080), { ok: false });
  assert.deepEqual(model.validatePort('65535'), { ok: true, value: 65535 });
  assert.equal(model.fieldFor('port'), 'port');
  assert.equal(model.fieldFor('keys.k4'), '');
  const cur = { port: 1, overlay: { a: 1 } };
  const body = model.buildConfigBody(cur, { quaver_dir: ' x ', port: 2, language: 'en', watch_quaver_process: 'yes' }, []);
  assert.deepEqual(body, { port: 2, overlay: { a: 1 }, quaver_dir: 'x', language: 'en' });
  body.overlay.a = 2;
  assert.equal(cur.overlay.a, 1);
  assert.equal('text_files' in body, false);
  assert.equal(model.buildConfigBody(cur, { text_files: true }, []).text_files, true);
  assert.equal(model.textFromConfig(null), false);
  assert.equal(model.textFromConfig({ text_files: 'true' }), false);
});

run();
