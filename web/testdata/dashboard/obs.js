// pages/obs.js: the OBS setup guide, the overlay link table, the connection check and the custom overlays card.
'use strict';
const assert = require('node:assert/strict');
const { test, run } = require('../../testsupport/run.js');
const dom = require('../../testsupport/dom.js');
const H = require('./helpers.js');

const ORIGIN = 'http://127.0.0.1:7878';
const NAMES = ['now-playing', 'result', 'history', 'state', 'alert', 'keys', 'curve'];

const open = (routes = {}) => H.open({ hash: '#/obs', routes });
const rows = (win) => win.$$('.obs-table tbody tr');
const checkButton = (win) => win.$$('button').filter((b) => b.textContent === H.en['obs.check'])[0];
const result = (win) => win.$('.obs-result');
const folder = (win) => win.$$('.obs-card')[1].querySelector('p[role="status"]');
const copyStatus = (win) => win.$('.obs-step p.status-line');

test('seven steps, the guide texts and the troubleshooting entries are on the page', async () => {
  const win = await open();
  assert.equal(win.$('h1').textContent, H.en['nav.obs']);
  assert.deepEqual(win.$$('.obs-step > h2').map((h) => h.textContent), [1, 2, 3, 4, 5, 6, 7].map((n) => H.en['obs.step' + n + '.title']));
  assert.equal(win.$$('.obs-step').length, 7);
  assert.equal(win.$$('.obs-step')[4].querySelectorAll('li').length, 4);
  assert.equal(win.$$('.obs-trouble').length, 4);
  assert.equal(win.$('.obs-step a.btn').getAttribute('href'), '#/overlays');
});

test('the table lists every overlay with its size and its link on this server', async () => {
  const win = await open();
  assert.equal(rows(win).length, 7);
  assert.deepEqual(rows(win).map((r) => r.querySelector('input').value), NAMES.map((n) => ORIGIN + '/overlays/' + n + '/'));
  assert.equal(rows(win)[0].querySelector('th').textContent, H.en['overlays.now_playing.name']);
  assert.match(rows(win)[0].querySelector('.obs-size').textContent, /540/);
  assert.equal(rows(win)[0].querySelector('input').getAttribute('readonly'), '');
  assert.equal(rows(win)[1].querySelector('input').getAttribute('aria-label'), H.en['overlays.url'] + ': ' + H.en['overlays.result.name']);
  assert.equal(rows(win)[1].querySelector('button').getAttribute('aria-label'), H.t('overlays.copy_aria', { name: H.en['overlays.result.name'] }));
  assert.equal(win.$('.table-wrap').getAttribute('aria-label'), H.en['obs.table_label']);
});

test('copy puts the link on the clipboard, says so, and goes back after 2 seconds', async () => {
  const win = await open();
  const button = rows(win)[2].querySelector('button');
  button.click();
  await win.flush();
  assert.deepEqual(win.clipboard.written, [ORIGIN + '/overlays/history/']);
  assert.equal(button.textContent, H.en['overlays.copied']);
  assert.equal(copyStatus(win).textContent, H.en['overlays.copied_live']);
  await win.advance(1999);
  assert.equal(button.textContent, H.en['overlays.copied']);
  await win.advance(1);
  assert.equal(button.textContent, H.en['overlays.copy']);
  assert.equal(copyStatus(win).textContent, '');
});

test('a second copy restarts the countdown', async () => {
  const win = await open();
  const [a, b] = [rows(win)[0].querySelector('button'), rows(win)[1].querySelector('button')];
  a.click();
  await win.flush();
  await win.advance(1500);
  b.click();
  await win.flush();
  await win.advance(1500);
  assert.equal(b.textContent, H.en['overlays.copied']);
  assert.equal(copyStatus(win).textContent, H.en['overlays.copied_live']);
  await win.advance(500);
  assert.equal(b.textContent, H.en['overlays.copy']);
});

test('a refused copy selects the link for the user', async () => {
  const win = await open();
  win.clipboard.fail = true;
  const row = rows(win)[3];
  row.querySelector('button').click();
  await win.flush();
  assert.equal(win.document.activeElement, row.querySelector('input'));
  assert.equal(row.querySelector('button').textContent, H.en['overlays.copy']);
  assert.equal(copyStatus(win).textContent, '');
});

test('a browser without a clipboard also selects the link', async () => {
  const win = await open();
  win.navigator.clipboard = undefined;
  const row = rows(win)[4];
  row.querySelector('button').click();
  assert.equal(win.document.activeElement, row.querySelector('input'));
  win.navigator.clipboard = {};
  rows(win)[5].querySelector('button').click();
  assert.equal(win.document.activeElement, rows(win)[5].querySelector('input'));
});

test('leaving the page right after copying changes nothing and leaves no timer behind', async () => {
  const win = await open();
  rows(win)[0].querySelector('button').click();
  win.location.hash = '#/status';
  await win.flush();
  await win.advance(2000);
  assert.equal(win.logs.length, 0);
  assert.equal(win.$('h1').textContent, H.en['nav.status']);
});

test('the check lists each result, links nothing when all pass and re-enables the button', async () => {
  const win = await open();
  const button = checkButton(win);
  button.click();
  assert.equal(button.disabled, true);
  assert.equal(result(win).textContent, H.en['obs.check_running']);
  await win.flush();
  assert.equal(button.disabled, false);
  const items = result(win).querySelectorAll('li');
  assert.deepEqual(items.map((li) => li.textContent), H.CHECKS.map((c) => H.en['check.' + c.id] + ': ' + H.en['status.ok']));
  assert.ok(items.every((li) => li.className === 'obs-ok'));
  assert.equal(result(win).querySelector('p').textContent, H.en['obs.check_ok']);
  assert.equal(result(win).querySelector('a'), null);
});

test('a failing check says problem and links to the Status page', async () => {
  const checks = [{ id: 'log', ok: true }, { id: 'port', ok: false }];
  const win = await open({ '/api/status': () => dom.json({ checks }) });
  checkButton(win).click();
  await win.flush();
  const items = result(win).querySelectorAll('li');
  assert.deepEqual(items.map((li) => li.className), ['obs-ok', 'obs-bad']);
  assert.equal(items[1].textContent, H.en['check.port'] + ': ' + H.en['status.problem']);
  assert.match(result(win).querySelector('p').textContent, new RegExp('^' + H.en['obs.check_bad'].slice(0, 15)));
  assert.equal(result(win).querySelector('a').getAttribute('href'), '#/status');
});

test('no checks at all counts as a failure and a status without a list is treated as empty', async () => {
  for (const body of [{ checks: [] }, {}, { checks: 'x' }]) {
    const win = await open({ '/api/status': () => dom.json(body) });
    checkButton(win).click();
    await win.flush();
    assert.equal(result(win).querySelectorAll('li').length, 0);
    assert.ok(result(win).querySelector('a'));
  }
});

test('an unreachable companion or a server error says it could not be reached', async () => {
  for (const answer of [() => new TypeError('Failed to fetch'), () => dom.json({}, 500)]) {
    const win = await open({ '/api/status': answer });
    checkButton(win).click();
    await win.flush();
    assert.equal(result(win).textContent, H.en['obs.check_failed']);
    assert.equal(result(win).querySelector('.obs-bad').textContent, H.en['obs.check_failed']);
    assert.equal(checkButton(win).disabled, false);
  }
});

test('no answer within 8 seconds also counts as unreachable', async () => {
  const win = await open({ '/api/status': () => new Promise(() => {}) });
  checkButton(win).click();
  await win.advance(7999);
  assert.equal(result(win).textContent, H.en['obs.check_running']);
  await win.advance(1);
  assert.equal(result(win).textContent, H.en['obs.check_failed']);
});

test('a newer check wins over an older one that finishes later', async () => {
  const slow = dom.deferred();
  let n = 0;
  const win = await open({ '/api/status': () => (++n === 1 ? slow.promise : dom.json({ checks: [{ id: 'log', ok: true }] })) });
  checkButton(win).click();
  checkButton(win).click();
  await win.flush();
  assert.equal(result(win).querySelectorAll('li').length, 1);
  slow.resolve(dom.json({ checks: [{ id: 'log', ok: false }, { id: 'port', ok: false }] }));
  await win.flush();
  assert.equal(result(win).querySelectorAll('li').length, 1);
  assert.equal(result(win).querySelector('.obs-ok') !== null, true);
});

test('leaving the page during a check ignores its answer', async () => {
  const gate = dom.deferred();
  const win = await open({ '/api/status': () => gate.promise });
  checkButton(win).click();
  win.location.hash = '#/history';
  await win.flush();
  gate.resolve(dom.json({ checks: [] }));
  await win.flush();
  assert.equal(win.logs.length, 0);
  assert.equal(win.$('h1').textContent, H.en['nav.history']);
});

test('the custom overlays card shows the folder the server gives, or says it is missing', async () => {
  const path = ['C:', 'cfg', 'overlays'].join(String.fromCharCode(92));
  const win = await open({ '/api/overlays': () => dom.json({ dir: path }) });
  assert.equal(folder(win).textContent, H.en['obs.custom.folder'] + ' ' + path);
  assert.equal(folder(win).querySelector('code').textContent, path);
  const link = win.$$('.obs-card')[1].querySelector('a');
  assert.equal(link.getAttribute('rel'), 'noopener noreferrer');
  assert.match(link.getAttribute('href'), /^https:\/\/github\.com\/.+\/docs\/OVERLAYS\.md$/);
  for (const answer of [() => dom.json({ dir: 5 }), () => dom.json({}, 500), () => new TypeError('x'), () => dom.json(null)]) {
    const other = await open({ '/api/overlays': answer });
    assert.equal(folder(other).textContent, H.en['obs.custom.folder_unavailable']);
  }
});

test('the folder answer arriving after leaving the page is dropped', async () => {
  const gate = dom.deferred();
  const win = await open({ '/api/overlays': () => gate.promise });
  win.location.hash = '#/status';
  await win.flush();
  gate.resolve(dom.json({ dir: 'C:/x' }));
  await win.flush();
  assert.equal(win.logs.length, 0);
});

run();
