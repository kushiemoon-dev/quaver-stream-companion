// pages/status.js and status-model.js: live state from the event stream, the diagnostics cards and the copy flow.
'use strict';
const assert = require('node:assert/strict');
const path = require('node:path');
const { test, run } = require('../../testsupport/run.js');
const H = require('./helpers.js');

const model = require(path.join(H.WEB, 'dashboard', 'pages', 'status-model.js'));

const MAP = { md5: 'aaa', title: 'Song', artist: 'Artist', difficulty_name: 'Hard', difficulty: 5.43, keys: 4 };
const PLAY = (over = {}) => ({
  map: { md5: 'aaa', title: 'Song' },
  score: { grade: 'S', accuracy: 98.5, max_combo: 321, rate: 1.2, mods: ['Mirror', 'Speed'], judgments: { marv: 100, perf: 20, great: 3, good: 2, okay: 1, miss: 0 } },
  ...over,
});

const text = (el) => el.textContent;
const conn = (win) => text(win.$('.head-pills span[role="status"]'));
const game = (win) => win.$('.pill-state');
const unsaved = (win) => text(win.$('.unsaved-note'));
const cards = (win) => win.$$('.diag').map((c) => ({
  label: text(c.querySelector('.card-label')), state: text(c.querySelector('.diag-state')), detail: text(c.querySelector('.diag-detail')),
  stale: !!c.querySelector('.stale-label'), cls: c.className,
}));
const copyButton = (win) => win.$$('button').filter((b) => text(b) === 'Copy diagnostics')[0];

async function live(opts) {
  const win = await H.open(opts);
  const es = H.stream(win);
  es.open();
  return { win, es };
}

const resetButton = (win) => win.$$('button').filter((b) => text(b) === H.en['status.session_reset'])[0];
const resetCalls = (win) => win.fetchCalls.filter((c) => c.path === '/api/session/reset');
const sessionLine = (win) => text(win.$('.session-status'));
const OK = { 'POST /api/session/reset': () => H.json({ status: 'reset' }) };

test('the session card posts the reset with the guard headers and says so', async () => {
  const win = await H.open({ routes: OK });
  assert.equal(text(win.$('#status-session-title')), H.en['status.session_title']);
  assert.equal(win.$('.session-status').getAttribute('aria-live'), 'polite');
  const btn = resetButton(win);
  btn.click();
  assert.equal(btn.getAttribute('aria-disabled'), 'true');
  await win.flush();
  assert.equal(resetCalls(win).length, 1);
  assert.equal(resetCalls(win)[0].method, 'POST');
  assert.equal(resetCalls(win)[0].init.headers['X-Requested-With'], 'qsc');
  assert.equal(resetCalls(win)[0].body, '{}');
  assert.equal(btn.hasAttribute('aria-disabled'), false);
  assert.equal(sessionLine(win), H.en['status.session_done']);
});

test('a second click while the reset runs does nothing', async () => {
  const gate = H.deferred();
  const win = await H.open({ routes: { 'POST /api/session/reset': () => gate.promise } });
  resetButton(win).click();
  resetButton(win).click();
  gate.resolve(H.json({ status: 'reset' }));
  await win.flush();
  assert.equal(resetCalls(win).length, 1);
});

test('reset failures name the cause: server error, timeout', async () => {
  let mode = 'server';
  const win = await H.open({
    routes: {
      'POST /api/session/reset': (c) => {
        if (mode === 'server') return H.json({ error: 'the session statistics are not available' }, 503);
        return new Promise((_, rej) => c.init.signal.addEventListener('abort', () => rej(Object.assign(new Error('x'), { name: 'AbortError' }))));
      },
    },
  });
  resetButton(win).click();
  await win.flush();
  assert.equal(sessionLine(win), H.t('status.session_failed', { message: H.t('error.server', { status: 503 }) }));
  mode = 'hang';
  resetButton(win).click();
  await win.advance(8000);
  await win.flush();
  assert.equal(sessionLine(win), H.t('status.session_failed', { message: H.en['settings.timeout'] }));
});

test('a reset answered after the page was left does not touch the new page', async () => {
  const gate = H.deferred();
  const win = await H.open({ routes: { 'POST /api/session/reset': () => gate.promise } });
  resetButton(win).click();
  win.location.hash = '#/overlays';
  await win.flush();
  gate.resolve(H.json({ status: 'reset' }));
  await win.flush();
  assert.equal(win.$('.session-status'), null);
});

test('before any data: connecting, unknown game, empty cards and the checks asked for', async () => {
  const win = await H.open();
  assert.equal(conn(win), 'Connecting');
  assert.equal(text(game(win)), 'Unknown');
  assert.equal(game(win).getAttribute('data-state'), 'unknown');
  assert.match(text(win.$('.card-now')), /No map yet/);
  assert.match(text(win.$('.card-last')), /No play yet/);
  assert.equal(win.eventSources[0].url, '/events');
  assert.ok(win.fetchCalls.some((c) => c.path === '/api/status'));
});

test('the connection pill follows the stream: live when open, reconnecting after an error', async () => {
  const { win, es } = await live();
  assert.equal(conn(win), 'Live');
  assert.equal(win.$('.head-pills span[role="status"]').className, 'pill pill-ok');
  es.fail(false);
  assert.equal(conn(win), 'Reconnecting');
  assert.equal(win.$('.head-pills span[role="status"]').className, 'pill pill-warn');
  assert.equal(win.eventSources.length, 1);
});

test('a stream the server closed is reopened after 3 seconds and the old one is ignored', async () => {
  const { win, es } = await live();
  es.fail(true);
  assert.equal(es.closed, true);
  await win.advance(2999);
  assert.equal(win.eventSources.length, 1);
  await win.advance(1);
  assert.equal(win.eventSources.length, 2);
  es.emit('game.state', { state: 'menu' });
  assert.equal(text(game(win)), 'Unknown');
  const next = win.eventSources[1];
  next.open();
  next.emit('game.state', { state: 'menu' });
  assert.equal(text(game(win)), 'Menu');
  assert.equal(conn(win), 'Live');
});

test('a snapshot fills the game state, the map card and the last play card', async () => {
  const { win, es } = await live();
  es.emit('snapshot', { game: 'menu', map: MAP, last_score: PLAY() });
  assert.equal(text(game(win)), 'Menu');
  assert.equal(game(win).getAttribute('data-state'), 'menu');
  assert.equal(game(win).className, 'pill pill-state state-menu');
  const now = win.$('.card-now');
  assert.equal(text(now.querySelector('.map-title')), 'Song');
  assert.equal(text(now.querySelector('.map-artist')), 'Artist');
  assert.deepEqual(now.querySelectorAll('.chip').map(text), ['Hard', '\u2605 5.4', '4K']);
  const last = win.$('.card-last');
  assert.equal(text(last.querySelector('.grade')).trim(), 'Grade S');
  assert.equal(last.querySelector('.grade').getAttribute('data-grade'), 's');
  assert.equal(text(last.querySelector('.acc')), '98.50%');
  assert.equal(text(last.querySelector('.last-meta .muted')), 'Max combo 321');
  assert.deepEqual(last.querySelectorAll('.judgment').map(text), ['Marvelous100', 'Perfect20', 'Great3', 'Good2', 'Okay1', 'Miss0']);
  assert.deepEqual(last.querySelectorAll('.chips .chip').map(text), ['1.2x', 'Mirror', 'Speed']);
});

test('result flags show as badges, and mods already shown as a badge are not repeated', async () => {
  const { win, es } = await live();
  const flags = { is_pb: true, is_local_best: true, full_combo: true, failed: true, paused: true, autoplay: true };
  es.emit('snapshot', { game: 'results', map: MAP, last_score: PLAY({ ...flags, score: { ...PLAY().score, mods: ['Autoplay', 'Mirror'] } }) });
  assert.deepEqual(win.$('.card-last').querySelectorAll('.pill').map(text), ['PB', 'Best', 'FC', 'Failed', 'Paused', 'Autoplay']);
  assert.deepEqual(win.$('.card-last').querySelectorAll('.chips .chip').map(text), ['1.2x', 'Mirror']);
  es.emit('snapshot', { game: 'results', map: MAP, last_score: PLAY() });
  assert.equal(win.$('.card-last').querySelectorAll('.chips .pill').length, 0);
});

test('game states outside the known list show as Unknown, with the state kept off the class name', async () => {
  const { win, es } = await live();
  es.emit('game.state', { state: 'gameplay' });
  assert.equal(text(game(win)), H.en['game.gameplay']);
  es.emit('game.state', { state: 'weird state' });
  assert.equal(text(game(win)), 'Unknown');
  assert.equal(game(win).className, 'pill pill-state state-unknown');
});

test('map.started replaces the map, score.saved replaces the last play', async () => {
  const { win, es } = await live();
  es.emit('map.started', { ...MAP, title: 'Other', difficulty_name: '' });
  assert.equal(text(win.$('.card-now .map-title')), 'Other');
  assert.deepEqual(win.$('.card-now').querySelectorAll('.chip').map(text), ['\u2605 5.4', '4K']);
  es.emit('score.saved', PLAY({ score: { grade: 'A', accuracy: 91, max_combo: 5 } }));
  assert.equal(text(win.$('.card-last .acc')), '91.00%');
  assert.deepEqual(win.$('.card-last').querySelectorAll('.judgment .j-count').map(text), ['0', '0', '0', '0', '0', '0']);
  assert.equal(win.$('.card-last').querySelectorAll('.chips .chip').length, 0);
});

test('a grade that is missing shows a question mark', async () => {
  const { win, es } = await live();
  es.emit('score.saved', PLAY({ score: { accuracy: 50 } }));
  assert.equal(text(win.$('.card-last .grade')).trim(), 'Grade ?');
  assert.equal(win.$('.card-last .grade').getAttribute('data-grade'), '');
});

test('numbers follow the language: comma decimals and a spaced percent sign in French', async () => {
  const { win, es } = await live({ routes: { '/api/config': () => H.json({ config: { language: 'fr' } }) } });
  es.emit('snapshot', { game: 'menu', map: MAP, last_score: PLAY() });
  assert.equal(text(win.$('.card-last .acc')), '98,50\u00a0%');
  assert.equal(win.$('.card-now').querySelectorAll('.chip')[1].textContent, '\u2605 5,4');
  assert.equal(win.$('.card-last').querySelectorAll('.chips .chip')[0].textContent, '1,2x');
});

test('the unsaved note appears only after a play that was not saved, for the map shown, and only with a last play', async () => {
  const { win, es } = await live();
  es.emit('snapshot', { game: 'gameplay', map: MAP, last_score: PLAY() });
  assert.equal(unsaved(win), '');
  es.emit('play.ended', { saved: false, map: { md5: 'zzz' } });
  assert.equal(unsaved(win), '');
  es.emit('play.ended', { saved: true, map: { md5: 'aaa' } });
  assert.equal(unsaved(win), '');
  es.emit('play.ended', { saved: false, map: { md5: 'aaa' } });
  assert.equal(unsaved(win), 'Last play was not saved');
  assert.equal(win.$('.unsaved-note').getAttribute('aria-live'), 'polite');
  es.emit('map.started', { ...MAP, md5: 'bbb' });
  assert.equal(unsaved(win), '');
});

test('without a previous play the unsaved note stays hidden, and a saved score clears it', async () => {
  const { win, es } = await live();
  es.emit('map.started', MAP);
  es.emit('play.ended', { saved: false, map: { md5: 'aaa' } });
  assert.equal(unsaved(win), '');
  es.emit('score.saved', PLAY());
  es.emit('play.ended', { saved: false, map: { md5: 'aaa' } });
  assert.equal(unsaved(win), 'Last play was not saved');
  es.emit('score.saved', PLAY());
  assert.equal(unsaved(win), '');
});

test('hostile titles and artists are shown as text', async () => {
  const { win, es } = await live();
  const evil = '<img src=x onerror=alert(1)><script>boom()</script>';
  es.emit('snapshot', { game: 'menu', map: { ...MAP, title: evil, artist: evil }, last_score: PLAY({ map: { title: evil } }) });
  assert.equal(win.$$('img').length, 0);
  assert.equal(text(win.$('.card-now .map-title')), evil);
  assert.equal(text(win.$('.card-last .map-title')), evil);
});

test('malformed or wrongly shaped events are ignored and leave the view as it was', async () => {
  const { win, es } = await live();
  es.emit('snapshot', { game: 'menu', map: MAP, last_score: PLAY() });
  es.emit('snapshot', '{not json');
  es.emit('snapshot', JSON.stringify({ type: 'snapshot', data: 5 }));
  es.emit('snapshot', JSON.stringify({ type: 'snapshot', data: null }));
  es.emit('game.state', { state: 7 });
  es.emit('score.saved', { score: 1 });
  es.emit('score.saved', []);
  assert.equal(text(game(win)), 'Menu');
  assert.equal(text(win.$('.card-now .map-title')), 'Song');
  assert.equal(text(win.$('.card-last .acc')), '98.50%');
  assert.equal(win.logs.filter((l) => l.level === 'error').length, 0);
});

test('an event that cannot be drawn is dropped and the previous state is drawn again', async () => {
  const { win, es } = await live();
  es.emit('snapshot', { game: 'menu', map: MAP, last_score: PLAY() });
  const create = win.document.createElement;
  let boom = true;
  win.document.createElement = (tag) => { if (boom) { boom = false; throw new Error('cannot draw'); } return create.call(win.document, tag); };
  es.emit('snapshot', { game: 'results', map: { ...MAP, title: 'Never shown' }, last_score: null });
  win.document.createElement = create;
  assert.equal(text(game(win)), 'Menu');
  assert.equal(text(win.$('.card-now .map-title')), 'Song');
  assert.equal(text(win.$('.card-last .acc')), '98.50%');
  es.emit('game.state', { state: 'lobby' });
  assert.equal(text(game(win)), 'Lobby');
});

test('a snapshot with odd parts keeps only the valid ones', async () => {
  const { win, es } = await live();
  es.emit('snapshot', { game: 3, map: 'x', last_score: { score: {} } });
  assert.equal(text(game(win)), 'Unknown');
  assert.match(text(win.$('.card-now')), /No map yet/);
  assert.match(text(win.$('.card-last')), /No play yet/);
});

test('the diagnostics cards show each check, the version and the config path', async () => {
  const win = await H.open();
  assert.deepEqual(cards(win).map((c) => [c.label, c.state, c.detail]), [
    ['Quaver folder', 'OK', H.QDIR], ['Game log', 'OK', 'Readable'], ['Score database', 'OK', 'Readable'], ['Local port', 'OK', 'In use by this qsc'],
  ]);
  assert.ok(cards(win).every((c) => c.cls.includes('diag-ok') && !c.stale));
  assert.equal(text(win.$('.footer')), 'Version 1.2.3Config file ' + H.CFG);
  assert.equal(win.$('.footer .sep').getAttribute('aria-hidden'), 'true');
});

test('a failing check without a translation keeps its raw text after a generic line', async () => {
  const checks = [{ id: 'quaver_dir', ok: true, detail: 'x' }, { id: 'log', ok: false, detail: 'weird <b>detail</b>' }, { id: 'database', ok: false, detail: 'busy: Quaver is writing the database' }, { id: 'port', ok: true, detail: 'free' }];
  const win = await H.open({ routes: { '/api/status': () => H.json(H.status({ checks })) } });
  const [, log, db, port] = cards(win);
  assert.equal(log.state, 'Problem');
  assert.equal(log.detail, 'The check reported: weird <b>detail</b>');
  assert.ok(log.cls.includes('diag-bad'));
  assert.equal(db.detail, 'Quaver is writing the database');
  assert.equal(port.detail, 'Free');
  assert.equal(win.$$('.diag b').length, 0);
});

test('Quaver folder missing shows the degraded banner with the steps and a link to Settings', async () => {
  const checks = [{ id: 'quaver_dir', ok: false, detail: 'Quaver folder not found' }, { id: 'log', ok: false, detail: 'nope' }, { id: 'database', ok: false, detail: 'nope' }, { id: 'port', ok: true, detail: 'free' }];
  const win = await H.open({ routes: { '/api/status': () => H.json(H.status({ checks })) } });
  const banner = win.$('.banner-degraded');
  assert.equal(banner.hidden, false);
  assert.equal(text(banner.querySelector('strong')), 'Quaver was not found');
  assert.equal(banner.querySelectorAll('.degraded-steps li').length, 4);
  assert.match(text(banner.querySelector('.degraded-steps')), /Steam.*Quaver/s);
  const link = banner.querySelector('a');
  assert.equal(link.getAttribute('href'), '#/settings');
  assert.equal(text(link), 'Open Settings');
  const demo = banner.querySelectorAll('a')[1];
  assert.equal(demo.getAttribute('href'), '#/overlays');
  assert.equal(text(demo), 'Preview the overlays');
  assert.match(text(banner), /previews them with demo data/);
  assert.match(text(banner), /Quaver folder not found/);
});

test('unreadable game files show a shorter banner without the install steps', async () => {
  const checks = [{ id: 'quaver_dir', ok: true, detail: 'x' }, { id: 'log', ok: false, detail: 'access denied' }, { id: 'database', ok: true, detail: 'readable' }, { id: 'port', ok: true, detail: 'free' }];
  const win = await H.open({ routes: { '/api/status': () => H.json(H.status({ checks })) } });
  const banner = win.$('.banner-degraded');
  assert.equal(banner.hidden, false);
  assert.equal(text(banner.querySelector('strong')), 'Quaver data cannot be read yet');
  assert.equal(banner.querySelectorAll('.degraded-steps').length, 0);
  assert.equal(banner.querySelectorAll('a').length, 0);
  assert.match(text(banner), /The check reported: access denied/);
});

test('the degraded banner disappears by itself once the checks pass again', async () => {
  let ok = false;
  const win = await H.open({ routes: { '/api/status': () => H.json(H.status({ checks: [{ id: 'quaver_dir', ok, detail: ok ? 'D:' : 'Quaver folder not found' }] })) } });
  assert.equal(win.$('.banner-degraded').hidden, false);
  ok = true;
  await win.advance(5000);
  assert.equal(win.$('.banner-degraded').hidden, true);
  assert.equal(win.$('.banner-degraded').childNodes.length, 0);
});

test('when the first status request fails the cards say the companion cannot be reached', async () => {
  const win = await H.open({ routes: { '/api/status': () => new TypeError('x') } });
  assert.ok(cards(win).every((c) => c.state === 'Unknown' && c.detail === 'Could not reach the companion.' && c.cls.includes('diag-unknown')));
  assert.equal(win.$('.diag-error').textContent, '');
});

test('after a failed refresh the last known checks stay, marked as not refreshed', async () => {
  let up = true;
  const win = await H.open({ routes: { '/api/status': () => (up ? H.json(H.status()) : new TypeError('x')) } });
  up = false;
  await win.advance(5000);
  assert.ok(cards(win).every((c) => c.stale && c.state === 'OK' && c.cls.includes('is-stale')));
  assert.equal(cards(win)[0].label, 'Quaver folder');
  assert.equal(win.$('.diag-error').textContent, H.en['status.refresh_failed']);
  assert.match(win.$('.stale-label').textContent, /Not refreshed/);
  up = true;
  await win.advance(5000);
  assert.ok(cards(win).every((c) => !c.stale));
  assert.equal(win.$('.diag-error').textContent, '');
});

test('being offline marks the cards as not refreshed and recovery clears the mark', async () => {
  const win = await H.open();
  win.qscOffline.report(false);
  assert.ok(cards(win).every((c) => c.stale));
  win.qscOffline.report(true);
  assert.ok(cards(win).every((c) => !c.stale));
});

test('the checks are refreshed every 5 seconds while visible, not while hidden, and again when the tab returns', async () => {
  const win = await H.open();
  const count = () => win.fetchCalls.filter((c) => c.path === '/api/status').length;
  const base = count();
  win.document.hidden = true;
  await win.advance(10000);
  assert.equal(count(), base);
  win.document.hidden = false;
  win.document.dispatchEvent(new win.Event('visibilitychange'));
  await win.flush();
  assert.equal(count(), base + 1);
  win.document.hidden = true;
  win.document.dispatchEvent(new win.Event('visibilitychange'));
  await win.flush();
  assert.equal(count(), base + 1);
});

test('only one status request is in flight at a time', async () => {
  const gate = H.deferred();
  const win = await H.open({ routes: { '/api/status': () => gate.promise } });
  await win.advance(4600);
  win.qscOffline.report(true);
  await win.advance(2000);
  assert.equal(win.fetchCalls.filter((c) => c.path === '/api/status').length, 1);
});

test('a status request that takes longer than 8 seconds is cancelled and shown as a failure', async () => {
  const win = await H.open({ routes: { '/api/status': (c) => new Promise((_, rej) => c.init.signal.addEventListener('abort', () => rej(Object.assign(new Error('x'), { name: 'AbortError' })))) } });
  await win.advance(8000);
  assert.ok(cards(win).every((c) => c.detail === 'Could not reach the companion.'));
});

test('an answer that arrives after leaving the page is ignored', async () => {
  const gate = H.deferred();
  const win = await H.open({ routes: { '/api/status': () => gate.promise } });
  win.qsc.registerPage('quiet', { title: 'nav.obs', mount: (r) => { r.textContent = 'quiet'; } });
  win.location.hash = '#/quiet';
  await win.flush();
  gate.resolve(H.json(H.status()));
  await win.flush();
  assert.equal(win.$('#main').textContent, 'quiet');
  assert.equal(win.logs.filter((l) => l.level === 'error').length, 0);
});

test('copy diagnostics puts the report on the clipboard and says so', async () => {
  const win = await H.open({ routes: { '/api/diagnostics': () => H.json({ report: 'REPORT TEXT' }) } });
  const btn = copyButton(win);
  btn.click();
  assert.equal(btn.getAttribute('aria-disabled'), 'true');
  await win.flush();
  await win.advance(16);
  assert.deepEqual(win.clipboard.written, ['REPORT TEXT']);
  assert.equal(btn.hasAttribute('aria-disabled'), false);
  assert.equal(text(win.$('.status-line')), H.en['status.copy_diag_done']);
  assert.equal(win.$('.copy-box').hidden, true);
  assert.equal(win.fetchCalls.filter((c) => c.path === '/api/diagnostics').length, 1);
});

test('a second click while copying does nothing', async () => {
  const gate = H.deferred();
  const win = await H.open({ routes: { '/api/diagnostics': () => gate.promise } });
  const btn = copyButton(win);
  btn.click();
  btn.click();
  gate.resolve(H.json({ report: 'R' }));
  await win.flush();
  assert.equal(win.fetchCalls.filter((c) => c.path === '/api/diagnostics').length, 1);
});

test('without clipboard access the report is offered in a selected text box instead', async () => {
  const win = await H.open({ routes: { '/api/diagnostics': () => H.json({ report: 'REPORT <b>TEXT</b>' }) } });
  win.clipboard.fail = true;
  copyButton(win).click();
  await win.flush();
  await win.advance(16);
  const box = win.$('.copy-box');
  const area = box.querySelector('textarea');
  assert.equal(box.hidden, false);
  assert.equal(area.textContent, 'REPORT <b>TEXT</b>');
  assert.equal(area.getAttribute('readonly'), '');
  assert.equal(area.getAttribute('aria-label'), 'Copy diagnostics');
  assert.equal(win.document.activeElement, area);
  assert.equal(text(win.$('.status-line')), H.en['settings.copy_manual']);
  win.clipboard.fail = false;
  copyButton(win).click();
  await win.flush();
  assert.equal(win.$('.copy-box').hidden, true);
});

test('a missing clipboard object is handled like a refused copy', async () => {
  const win = await H.open({ routes: { '/api/diagnostics': () => H.json({ report: 'R' }) } });
  win.navigator.clipboard = undefined;
  copyButton(win).click();
  await win.flush();
  assert.equal(win.$('.copy-box').hidden, false);
});

test('copy failures name the cause: server error, no answer, or timeout', async () => {
  let mode = 'server';
  const win = await H.open({
    routes: {
      '/api/diagnostics': (c) => {
        if (mode === 'server') return H.json({ error: 'Quaver is writing the database, try again' }, 503);
        if (mode === 'network') return new TypeError('x');
        return new Promise((_, rej) => c.init.signal.addEventListener('abort', () => rej(Object.assign(new Error('x'), { name: 'AbortError' }))));
      },
    },
  });
  const line = async () => { await win.flush(); await win.advance(16); return text(win.$('.status-line')); };
  copyButton(win).click();
  assert.equal(await line(), H.t('status.copy_diag_failed', { message: H.en['error.busy'] }));
  mode = 'network';
  copyButton(win).click();
  assert.equal(await line(), H.t('status.copy_diag_failed', { message: H.en['status.unreachable'] }));
  mode = 'hang';
  copyButton(win).click();
  await win.advance(8000);
  assert.equal(await line(), H.t('status.copy_diag_failed', { message: H.en['settings.timeout'] }));
  assert.equal(win.$('.copy-box').hidden, true);
});

test('model: snapshot, game, map and score events reduce to the state', () => {
  let s = model.initial();
  assert.deepEqual(s, { game: 'unknown', map: null, last_score: null, unsaved: false });
  s = model.reduce(s, 'game.state', { state: 'menu' });
  s = model.reduce(s, 'map.started', { md5: 'a' });
  assert.equal(s.game, 'menu');
  assert.equal(s.map.md5, 'a');
  s = model.reduce(s, 'score.saved', PLAY());
  assert.equal(s.last_score.score.grade, 'S');
  const snap = model.reduce(s, 'snapshot', { game: 'results', map: { md5: 'b' }, last_score: { score: 1 } });
  assert.deepEqual(snap, { game: 'results', map: { md5: 'b' }, last_score: null, unsaved: false });
});

test('model: payloads of the wrong shape are rejected so the old state stays', () => {
  const s = model.initial();
  for (const name of ['snapshot', 'game.state', 'map.started', 'score.saved', 'play.ended']) {
    for (const bad of [null, 5, 'x', [], undefined]) assert.throws(() => model.reduce(s, name, bad), /bad payload/, name);
  }
  assert.throws(() => model.reduce(s, 'game.state', { state: 1 }), /bad payload/);
  assert.throws(() => model.reduce(s, 'score.saved', { map: {} }), /bad payload/);
  assert.throws(() => model.reduce(s, 'score.saved', { score: {} }), /bad payload/);
});

test('model: play.ended only flags the map that is shown, and showUnsaved needs a last play', () => {
  let s = model.reduce(model.initial(), 'map.started', { md5: 'a' });
  assert.equal(model.reduce(s, 'play.ended', { saved: true, map: { md5: 'a' } }).unsaved, false);
  assert.equal(model.reduce(s, 'play.ended', { saved: false, map: { md5: 'b' } }).unsaved, false);
  assert.equal(model.reduce(s, 'play.ended', { saved: false, map: {} }).unsaved, false);
  assert.equal(model.reduce(s, 'play.ended', { saved: false }).unsaved, false);
  s = model.reduce(s, 'play.ended', { saved: false, map: { md5: 'a' } });
  assert.equal(s.unsaved, true);
  assert.equal(model.showUnsaved(s), false);
  assert.equal(model.showUnsaved(model.reduce(s, 'snapshot', { map: { md5: 'a' }, last_score: PLAY() })), false);
  assert.equal(model.showUnsaved({ ...s, last_score: PLAY() }), true);
});

run();
