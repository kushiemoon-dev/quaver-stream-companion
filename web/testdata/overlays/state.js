'use strict';
const assert = require('node:assert/strict');
const { test, run } = require('../../testsupport/run.js');
const { open, stream, visible } = require('./helpers.js');

test('demo shows the playing pill with a pulsing green dot and opens nothing', () => {
  const win = open('state', 'demo=1');
  assert.ok(visible(win.$('#pill')));
  assert.equal(win.$('#label').textContent, 'Playing');
  assert.equal(win.$('#dot').className, 'dot green pulse');
  assert.equal(win.eventSources.length, 0);
  assert.equal(win.document.title, 'QSC game state');
});

test('the live pill waits for Quaver until the stream says more', () => {
  const win = open('state', '');
  assert.equal(win.$('#label').textContent, 'Waiting for Quaver');
  assert.equal(win.$('#dot').className, 'dot muted');
  assert.ok(visible(win.$('#pill')));
});

test('snapshots and state changes update the label and the dot tone', () => {
  const win = open('state', '');
  const es = stream(win);
  es.emit('snapshot', { game: 'select' });
  const select = win.$('#label').textContent;
  es.emit('game.state', { state: 'gameplay' });
  assert.equal(win.$('#label').textContent, 'Playing');
  assert.ok(win.$('#dot').classList.contains('pulse'));
  es.emit('game.state', { state: 'loading' });
  assert.equal(win.$('#dot').className, 'dot amber');
  assert.notEqual(win.$('#label').textContent, select);
  es.emit('game.state', { state: 'closed' });
  assert.equal(win.$('#dot').className, 'dot muted');
});

test('states listed in hide leave the pill hidden and come back when the state changes', () => {
  const win = open('state', 'hide=menu,closed');
  const es = stream(win);
  es.emit('game.state', { state: 'menu' });
  assert.ok(!visible(win.$('#pill')));
  es.emit('game.state', { state: 'results' });
  assert.ok(visible(win.$('#pill')));
  es.emit('game.state', { state: 'closed' });
  assert.ok(!visible(win.$('#pill')));
});

test('hide=unknown covers a state the page does not know', () => {
  const win = open('state', 'hide=unknown');
  assert.ok(!visible(win.$('#pill')));
  const es = stream(win);
  es.emit('game.state', { state: 'something new' });
  assert.ok(!visible(win.$('#pill')));
  es.emit('game.state', { state: 'gameplay' });
  assert.ok(visible(win.$('#pill')));
});

test('the label follows the language', () => {
  const win = open('state', 'lang=de');
  const es = stream(win);
  es.emit('game.state', { state: 'gameplay' });
  assert.equal(win.$('#label').textContent, 'Spielt');
  assert.equal(win.document.documentElement.lang, 'de');
});

run();
