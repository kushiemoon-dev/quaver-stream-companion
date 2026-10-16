'use strict';
const assert = require('node:assert/strict');
const { test, run } = require('../../testsupport/run.js');
const { open, stream, visible } = require('./helpers.js');

const STARTED = '2026-01-01T00:00:00.000Z';
const SESSION = {
  started_at: STARTED, plays: 12, passes: 9, fails: 3, abandoned: 1, avg_accuracy: 95.214, best_accuracy: 98.7, best_rating: 24.5,
  full_combos: 2, streak: 4, best_streak: 6,
};
const values = (win) => win.$$('.session-cell b').map((b) => b.textContent);
const labels = (win) => win.$$('.session-cell span').map((b) => b.textContent);

test('demo shows example figures, ticks the clock and opens nothing', async () => {
  const win = open('session', 'demo=1');
  assert.ok(visible(win.$('#card')));
  assert.deepEqual(values(win), ['12', '9 / 3', '95.21%', '98.70%', '4', '6', '2', '24.53']);
  assert.equal(win.$('.session-time').textContent, '1:02:03');
  await win.advance(60000);
  assert.equal(win.$('.session-time').textContent, '1:03:03');
  assert.equal(win.eventSources.length, 0);
  assert.equal(win.fetchCalls.length, 0);
  assert.equal(win.document.title, 'QSC session');
});

test('demo follows the language', () => {
  const win = open('session', 'demo=1&lang=fr');
  assert.match(values(win)[2], /^95,21\s?%$/);
  assert.equal(values(win)[7], '24,53');
  assert.equal(win.document.title, 'QSC session');
  assert.deepEqual(labels(win).slice(0, 2), ['Parties', 'Réussies / ratées']);
});

test('the live card waits for the first snapshot', () => {
  const win = open('session', '');
  const es = stream(win);
  assert.ok(!visible(win.$('#card')));
  es.emit('snapshot', { game: 'menu', session: SESSION });
  assert.ok(visible(win.$('#card')));
  assert.equal(values(win)[0], '12');
});

test('a snapshot without a session leaves the card hidden', () => {
  const win = open('session', '');
  stream(win).emit('snapshot', { game: 'menu' });
  assert.ok(!visible(win.$('#card')));
});

test('session.updated replaces the figures and the clock follows started_at', async () => {
  const win = open('session', '');
  const es = stream(win);
  const now = new win.Date().toISOString();
  es.emit('snapshot', { session: { started_at: now, plays: 0, passes: 0, fails: 0 } });
  assert.deepEqual(values(win), ['0', '0 / 0', '-', '-', '0', '0', '0', '-']);
  assert.equal(win.$('.session-time').textContent, '0:00');
  await win.advance(5000);
  assert.equal(win.$('.session-time').textContent, '0:05');
  es.emit('session.updated', { ...SESSION, started_at: now });
  assert.equal(values(win)[1], '9 / 3');
  es.emit('session.updated', { started_at: new win.Date().toISOString(), plays: 0 });
  assert.equal(win.$('.session-time').textContent, '0:00');
});

test('hostile numbers cannot break the card', () => {
  const win = open('session', '');
  stream(win).emit('snapshot', { session: { started_at: 'x', plays: 'many', passes: -3, fails: 1e30, avg_accuracy: 'a', best_rating: null } });
  assert.ok(visible(win.$('#card')));
  assert.deepEqual(values(win), ['0', '0 / 999999', '-', '-', '0', '0', '0', '-']);
  assert.equal(win.$('.session-time').textContent, '0:00');
});

test('scale is applied', () => {
  const win = open('session', 'demo=1&scale=2');
  assert.equal(win.document.documentElement.style.getPropertyValue('--scale'), '2');
});

run();
