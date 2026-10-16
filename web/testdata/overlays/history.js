'use strict';
const assert = require('node:assert/strict');
const { test, run } = require('../../testsupport/run.js');
const { open, stream, visible, json, text, deferred } = require('./helpers.js');

const PLAYS = [
  { accuracy: 99.1, grade: 'S' },
  { accuracy: 88.2, grade: 'A' },
  { accuracy: 97.5, grade: 'SS' },
];
const histFetches = (win) => win.fetchCalls.filter((c) => c.path === '/api/history');
const route = (body) => ({ '/api/history': json(body) });

test('demo draws ten plays with the last accuracy and never asks the server', () => {
  const win = open('history', 'demo=1');
  assert.ok(visible(win.$('#card')));
  assert.equal(win.$$('circle').length, 10);
  assert.equal(win.$$('polyline').length, 1);
  assert.equal(win.$$('polyline')[0].getAttribute('points').split(' ').length, 10);
  assert.equal(win.$$('text').pop().textContent, '93.00%');
  assert.equal(win.$('svg').getAttribute('role'), 'img');
  assert.equal(win.$('svg').getAttribute('aria-label'), 'Accuracy of your last plays');
  assert.equal(win.eventSources.length, 0);
  assert.equal(histFetches(win).length, 0);
});

test('best=1 adds the best line and its label in the page language', () => {
  assert.equal(open('history', 'demo=1').$$('.best').length, 0);
  const win = open('history', 'demo=1&best=1&lang=fr');
  assert.equal(win.$$('line.best').length, 1);
  assert.match(win.$('.best-label').textContent, /^Meilleur 98,30\s?%$/);
});

test('the live page fetches the bounded list once and draws it oldest first', async () => {
  const win = open('history', 'n=12', { routes: route(PLAYS) });
  await win.flush();
  assert.equal(histFetches(win).length, 1);
  assert.equal(histFetches(win)[0].url, '/api/history?limit=12');
  assert.equal(histFetches(win)[0].init.credentials, 'same-origin');
  assert.ok(visible(win.$('#card')));
  assert.equal(win.$$('circle').length, 3);
  assert.deepEqual(win.$$('circle').map((c) => c.className), ['dot-gold', 'dot-green', 'dot-gold']);
  assert.equal(win.$$('circle')[2].getAttribute('r'), '6');
  assert.equal(win.$$('text').pop().textContent, '99.10%');
});

test('a failed or empty history leaves the card hidden', async () => {
  for (const routes of [{}, { '/api/history': json({}, 500) }, route([]), route(null), { '/api/history': text('nope') }]) {
    const win = open('history', '', { routes });
    await win.flush();
    assert.ok(!visible(win.$('#card')));
  }
});

test('a score.saved reloads after the debounce, once for a burst', async () => {
  const win = open('history', '', { routes: route(PLAYS) });
  await win.flush();
  const es = stream(win);
  es.emit('score.saved', { score: { id: 1 } });
  await win.advance(300);
  es.emit('score.saved', { score: { id: 2 } });
  await win.advance(599);
  assert.equal(histFetches(win).length, 1);
  await win.advance(1);
  assert.equal(histFetches(win).length, 2);
});

test('opening the stream retries a first load that failed, but not after a good one', async () => {
  const failing = open('history', '', { routes: {} });
  await failing.flush();
  stream(failing);
  assert.equal(histFetches(failing).length, 2);
  failing.routes['/api/history'] = json(PLAYS);
  failing.eventSources[0].open();
  await failing.flush();
  assert.ok(visible(failing.$('#card')));

  const good = open('history', '', { routes: route(PLAYS) });
  await good.flush();
  stream(good);
  assert.equal(histFetches(good).length, 1);
});

test('a newer load aborts the one still running and only the newer draws', async () => {
  const win = open('history', '', { routes: route(PLAYS) });
  await win.flush();
  const es = stream(win);
  const slow = deferred();
  win.routes['/api/history'] = () => slow.promise;
  es.emit('score.saved', { score: { id: 1 } });
  await win.advance(600);
  const running = histFetches(win)[1].init.signal;
  win.routes['/api/history'] = json([{ accuracy: 80, grade: 'B' }]);
  es.emit('score.saved', { score: { id: 2 } });
  await win.advance(600);
  assert.ok(running.aborted);
  assert.equal(win.$$('circle').length, 1);
  slow.resolve(json(PLAYS));
  await win.flush();
  assert.equal(win.$$('circle').length, 1);
});

test('a load that takes longer than four seconds is aborted', async () => {
  const win = open('history', '', { routes: { '/api/history': () => deferred().promise } });
  const signal = histFetches(win)[0].init.signal;
  await win.advance(3999);
  assert.ok(!signal.aborted);
  await win.advance(1);
  assert.ok(signal.aborted);
  assert.ok(!visible(win.$('#card')));
});

test('odd accuracies are clamped and unusable rows are skipped', async () => {
  const rows = [{ accuracy: 1e30, grade: 'S' }, { accuracy: 'x' }, null, { accuracy: -50, grade: '<b>' }, { accuracy: 90 }];
  const win = open('history', '', { routes: route(rows) });
  await win.flush();
  assert.equal(win.$$('circle').length, 3);
  for (const p of win.$('polyline').getAttribute('points').split(/[ ,]/)) assert.ok(Number.isFinite(Number(p)));
});

run();
