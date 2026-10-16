'use strict';
const assert = require('node:assert/strict');
const { test, run } = require('../../testsupport/run.js');
const { open, stream, visible, json, text, deferred } = require('./helpers.js');

const TIMELINE = { total: 1000, points: [100, 99, 98, 97.5], misses: [100, 500, 5000] };
const SCORE = (id) => ({ score: { id }, map: { title: 'Song' } });
const timelineCalls = (win) => win.fetchCalls.filter((c) => /\/timeline$/.test(c.path));
const route = (id, body) => ({ ['/api/scores/' + id + '/timeline']: body });

test('demo draws the example curve with its misses and never asks the server', () => {
  const win = open('curve', 'demo=1');
  assert.ok(visible(win.$('#card')));
  assert.equal(win.$$('line.miss').length, 5);
  assert.equal(win.$('text.final').textContent, '98.21%');
  assert.equal(win.$('svg').getAttribute('aria-label'), 'Accuracy over the judgments of the last play 98.21%, Misses 5');
  assert.equal(win.$('svg title').textContent, win.$('svg').getAttribute('aria-label'));
  assert.equal(win.eventSources.length, 0);
  assert.equal(win.fetchCalls.length, 0);
  assert.equal(win.document.title, 'QSC judgment curve');
});

test('demo follows the language', () => {
  const win = open('curve', 'demo=1&lang=de');
  assert.match(win.$('text.final').textContent, /^98,21\s?%$/);
});

test('a saved score loads its timeline and draws it', async () => {
  const win = open('curve', '', { routes: route(3, json(TIMELINE)) });
  const es = stream(win);
  assert.ok(!visible(win.$('#card')));
  es.emit('score.saved', SCORE(3));
  await win.flush();
  assert.equal(timelineCalls(win).length, 1);
  assert.equal(timelineCalls(win)[0].url, '/api/scores/3/timeline');
  assert.equal(timelineCalls(win)[0].init.credentials, 'same-origin');
  assert.ok(visible(win.$('#card')));
  assert.equal(win.$('text.final').textContent, '97.50%');
  assert.equal(win.$$('line.miss').length, 2, 'a miss beyond the total is dropped');
});

test('a snapshot loads the last score once and a reconnect does not reload it', async () => {
  const win = open('curve', '', { routes: route(3, json(TIMELINE)) });
  const es = stream(win);
  es.emit('snapshot', { last_score: SCORE(3) });
  await win.flush();
  es.emit('snapshot', { last_score: SCORE(3) });
  await win.flush();
  assert.equal(timelineCalls(win).length, 1);
  es.emit('snapshot', { last_score: null });
  assert.equal(timelineCalls(win).length, 1);
});

test('scores without a usable id never reach the server', async () => {
  const win = open('curve', '');
  const es = stream(win);
  for (const id of [undefined, 0, -1, 1.5, '3', null]) es.emit('score.saved', { score: { id } });
  es.emit('score.saved', {});
  await win.flush();
  assert.equal(timelineCalls(win).length, 0);
});

test('a failed or empty timeline hides the card', async () => {
  const win = open('curve', '', { routes: { ...route(1, json(TIMELINE)), ...route(2, json({}, 404)), ...route(3, json({ points: [] })), ...route(4, text('bad')) } });
  const es = stream(win);
  es.emit('score.saved', SCORE(1));
  await win.flush();
  assert.ok(visible(win.$('#card')));
  for (const id of [2, 3, 4, 5]) {
    es.emit('score.saved', SCORE(id));
    await win.flush();
    assert.ok(!visible(win.$('#card')), 'score ' + id);
  }
});

test('a newer score aborts the running load and the older answer is dropped', async () => {
  const slow = deferred();
  const win = open('curve', '', { routes: { ...route(1, () => slow.promise), ...route(2, json(TIMELINE)) } });
  const es = stream(win);
  es.emit('score.saved', SCORE(1));
  const first = timelineCalls(win)[0].init.signal;
  es.emit('score.saved', SCORE(2));
  await win.flush();
  assert.ok(first.aborted);
  slow.resolve(json({ total: 10, points: [50] }));
  await win.flush();
  assert.equal(win.$('text.final').textContent, '97.50%');
});

test('a load slower than four seconds is aborted', async () => {
  const win = open('curve', '', { routes: route(1, () => deferred().promise) });
  stream(win).emit('score.saved', SCORE(1));
  const signal = timelineCalls(win)[0].init.signal;
  await win.advance(3999);
  assert.ok(!signal.aborted);
  await win.advance(1);
  assert.ok(signal.aborted);
  assert.ok(!visible(win.$('#card')));
});

test('autohide hides the curve after the delay', async () => {
  const win = open('curve', 'autohide=6', { routes: route(1, json(TIMELINE)) });
  stream(win).emit('score.saved', SCORE(1));
  await win.flush();
  await win.advance(5999);
  assert.ok(visible(win.$('#card')));
  await win.advance(1);
  assert.ok(!visible(win.$('#card')));
});

run();
