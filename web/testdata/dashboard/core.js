// i18n.js, api.js and offline.js: catalogs and fallbacks, requests and error mapping, offline reporting and the heartbeat.
'use strict';
const assert = require('node:assert/strict');
const { test, run } = require('../../testsupport/run.js');
const H = require('./helpers.js');

test('pick prefers a supported configured language, then the browser language, then English', async () => {
  const { pick } = (await H.open()).qscI18n;
  assert.equal(pick('fr', 'de'), 'fr');
  assert.equal(pick('', 'fr-CA'), 'fr');
  assert.equal(pick('xx', 'DE-ch'), 'de');
  assert.equal(pick('', 'ja-JP'), 'en');
  assert.equal(pick(undefined, undefined), 'en');
  assert.equal(pick('FR', ''), 'en');
});

test('load merges the chosen catalog over English, so untranslated keys keep their English text', async () => {
  const partial = { 'nav.status': 'Etat' };
  const win = await H.open({ routes: { 'i18n/fr.json': () => H.json(partial) } });
  const lang = await win.qscI18n.load('fr');
  assert.equal(lang, 'fr');
  assert.equal(win.qscI18n.lang(), 'fr');
  assert.equal(win.qscI18n.t('nav.status'), 'Etat');
  assert.equal(win.qscI18n.t('nav.obs'), 'OBS');
  assert.equal(win.document.documentElement.lang, 'fr');
});

test('load without any catalog leaves keys as they are and keeps the page language', async () => {
  const dead = () => H.json({}, 500);
  const win = await H.open({ routes: { 'i18n/en.json': dead, 'i18n/de.json': dead } });
  assert.equal(await win.qscI18n.load('de'), 'de');
  assert.equal(win.qscI18n.t('nav.status'), 'nav.status');
  assert.notEqual(win.document.documentElement.lang, 'de');
});

test('a network failure or malformed catalog is treated like a missing one', async () => {
  const win = await H.open({ routes: { 'i18n/en.json': () => new Error('offline'), 'i18n/fr.json': () => H.text('{broken') } });
  await win.qscI18n.load('fr');
  assert.equal(win.qscI18n.t('nav.status'), 'nav.status');
});

test('t fills placeholders, leaves unknown ones, and survives keys named like object members', async () => {
  const win = await H.open();
  const { t } = win.qscI18n;
  assert.equal(t('status.keys', { n: 7 }), '7K');
  assert.equal(t('status.keys', {}), '{n}K');
  assert.equal(t('status.keys'), '{n}K');
  assert.equal(t('status.keys', { n: 0 }), '0K');
  assert.equal(t('no.such.key'), 'no.such.key');
  assert.equal(t('constructor'), 'constructor');
  assert.equal(t('__proto__'), '__proto__');
  assert.equal(t('status.keys', { n: '$&' }), '$&K');
});

test('every catalog defines the same keys as English', () => {
  const keys = (lang) => Object.keys(H.catalog(lang)).sort();
  assert.deepEqual(keys('fr'), keys('en'));
  assert.deepEqual(keys('de'), keys('en'));
});

const api = (win) => win.qscApi;

test('GET sends Accept only; PUT and POST send a JSON body, its type and the anti-forgery header', async () => {
  const win = await H.open({ routes: { '/api/x': () => H.json({ ok: 1 }) } });
  win.fetchCalls.length = 0;
  assert.deepEqual(await api(win).get('/api/x'), { ok: 1 });
  await api(win).put('/api/x', { a: 1 });
  await api(win).post('/api/x', undefined);
  const [get, put, post] = win.fetchCalls;
  assert.equal(get.method, 'GET');
  assert.equal(JSON.stringify(get.init.headers), '{"Accept":"application/json"}');
  assert.equal(get.body, undefined);
  assert.equal(put.method, 'PUT');
  assert.equal(put.init.headers['Content-Type'], 'application/json');
  assert.equal(put.init.headers['X-Requested-With'], 'qsc');
  assert.equal(put.body, '{"a":1}');
  assert.equal(post.method, 'POST');
  assert.equal(post.body, undefined);
});

test('an unreachable companion raises a network error and shows the offline banner; the next answer clears it', async () => {
  let up = false;
  const win = await H.open({ routes: { '/api/x': () => (up ? H.json({}) : new TypeError('Failed to fetch')) } });
  await assert.rejects(api(win).get('/api/x'), (e) => e.network === true && e.message === H.en['error.network']);
  assert.equal(win.$('#offline').textContent, H.en['offline.banner']);
  up = true;
  await api(win).get('/api/x');
  assert.equal(win.$('#offline').textContent, '');
});

test('an aborted request rethrows the abort and does not count as offline', async () => {
  const gate = H.deferred();
  const win = await H.open({ routes: { '/api/slow': () => gate.promise } });
  const ctl = new win.AbortController();
  const p = api(win).get('/api/slow', ctl.signal);
  ctl.abort();
  await assert.rejects(p, (e) => e.name === 'AbortError');
  assert.equal(win.$('#offline').textContent, '');
  assert.equal(win.qscOffline.isOffline(), false);
});

test('known server messages are translated, with the key count filled in', async () => {
  const body = (error, field) => () => H.json({ error, field }, 400);
  const win = await H.open({
    routes: {
      '/api/port': body('must be between 1 and 65535', 'port'),
      '/api/keys': body('must list exactly 4 keys', 'keys'),
      '/api/entry': body('entry 3 is not a supported key name', 'keys'),
      '/api/busy': () => H.json({ error: 'Quaver is writing the database, try again' }, 503),
    },
  });
  await assert.rejects(api(win).put('/api/port', {}), (e) => e.message === H.en['error.port_range'] && e.status === 400 && e.field === 'port');
  await assert.rejects(api(win).put('/api/keys', {}), (e) => e.message === 'Must list exactly 4 keys.');
  await assert.rejects(api(win).put('/api/entry', {}), (e) => e.message === 'Entry 3 is not a supported key name.');
  await assert.rejects(api(win).get('/api/busy'), (e) => e.message === H.en['error.busy'] && e.status === 503 && e.field === '');
});

test('an unknown or unreadable error answer becomes the generic message with the status', async () => {
  const win = await H.open({
    routes: {
      '/api/odd': () => H.json({ error: 'something new' }, 500),
      '/api/html': () => H.text('<html>boom</html>', 502),
      '/api/num': () => H.json({ error: 42 }, 418),
    },
  });
  await assert.rejects(api(win).get('/api/odd'), (e) => e.message === H.t('error.server', { status: 500 }));
  await assert.rejects(api(win).get('/api/html'), (e) => e.message === H.t('error.server', { status: 502 }) && e.status === 502);
  await assert.rejects(api(win).get('/api/num'), (e) => e.message === H.t('error.server', { status: 418 }));
});

test('an error status still proves the companion is online, and an empty success body gives null', async () => {
  const win = await H.open({ routes: { '/api/down': () => new TypeError('x'), '/api/err': () => H.json({}, 500), '/api/empty': () => H.text('', 200) } });
  await assert.rejects(api(win).get('/api/down'));
  assert.equal(win.qscOffline.isOffline(), true);
  await assert.rejects(api(win).get('/api/err'));
  assert.equal(win.qscOffline.isOffline(), false);
  assert.equal(await api(win).get('/api/empty'), null);
});

test('offline reports notify listeners once per change and stop after unsubscribing', async () => {
  const win = await H.open();
  const seen = [];
  const off = win.qscOffline.onChange((v) => seen.push(v));
  win.qscOffline.report(true);
  win.qscOffline.report(false);
  win.qscOffline.report(false);
  win.qscOffline.report(true);
  assert.deepEqual(seen, [true, false]);
  off();
  win.qscOffline.report(false);
  assert.deepEqual(seen, [true, false]);
  assert.equal(win.$('#offline').textContent, H.en['offline.banner']);
  assert.equal(win.$('#offline').getAttribute('role'), 'status');
});

// A page without its own polling, so only the heartbeat of offline.js asks the companion.
async function quietDashboard(routes) {
  const win = await H.open({ routes });
  win.qsc.registerPage('quiet', { title: 'nav.obs', mount() {} });
  win.location.hash = '#/quiet';
  await win.flush();
  win.fetchCalls.length = 0;
  return win;
}

test('the heartbeat asks for the status every 5 seconds when nothing else spoke to the companion', async () => {
  const win = await quietDashboard({});
  await win.advance(5000);
  assert.deepEqual(win.fetchCalls.map((c) => c.path), ['/api/status']);
  await win.advance(5000);
  assert.equal(win.fetchCalls.length, 2);
});

test('the heartbeat stays quiet while the page is hidden or another request was just answered', async () => {
  const win = await quietDashboard({});
  win.document.hidden = true;
  await win.advance(10000);
  assert.equal(win.fetchCalls.length, 0);
  win.document.hidden = false;
  await win.advance(1000);
  win.qscOffline.report(true);
  await win.advance(3000);
  assert.equal(win.fetchCalls.length, 0);
});

test('a heartbeat that does not answer within 4 seconds marks the companion offline', async () => {
  const gate = H.deferred();
  const win = await quietDashboard({ '/api/status': () => gate.promise });
  await win.advance(5000);
  assert.equal(win.qscOffline.isOffline(), false);
  await win.advance(3999);
  assert.equal(win.qscOffline.isOffline(), false);
  await win.advance(1);
  assert.equal(win.qscOffline.isOffline(), true);
  assert.equal(win.$('#offline').textContent, H.en['offline.banner']);
});

test('a heartbeat that fails marks offline and the next successful one recovers', async () => {
  let up = false;
  const win = await quietDashboard({ '/api/status': () => (up ? H.json({}) : new TypeError('x')) });
  await win.advance(5000);
  assert.equal(win.qscOffline.isOffline(), true);
  up = true;
  await win.advance(5000);
  assert.equal(win.qscOffline.isOffline(), false);
  assert.equal(win.$('#offline').textContent, '');
});

run();
