'use strict';
const assert = require('node:assert/strict');
const { test, run } = require('../../testsupport/run.js');
const { open, stream, visible } = require('./helpers.js');

const MAP = { title: 'Song', artist: 'Artist', difficulty_name: 'Hard', difficulty: 18.5, keys: 4 };

test('demo shows the example card without opening a stream or asking the server', () => {
  const win = open('now-playing', 'demo=1');
  assert.ok(visible(win.$('#card')));
  assert.equal(win.$('#title').textContent, 'Example Song');
  assert.equal(win.$('#artist').textContent, 'Example Artist');
  assert.match(win.$('#diff').textContent, /^Hard .* 18\.50/);
  assert.equal(win.eventSources.length, 0);
  assert.equal(win.fetchCalls.length, 0);
  assert.equal(win.document.title, 'QSC now playing');
});

test('demo follows the language parameter', () => {
  const win = open('now-playing', 'demo=1&lang=fr');
  assert.equal(win.document.title, 'QSC en cours de lecture');
  assert.equal(win.document.documentElement.lang, 'fr');
  assert.match(win.$('#diff').textContent, /18,50/);
});

test('live page starts hidden and shows a map once the game is playing', () => {
  const win = open('now-playing', '');
  const es = stream(win);
  assert.ok(!visible(win.$('#card')));
  es.emit('snapshot', { game: 'gameplay', map: MAP });
  assert.ok(visible(win.$('#card')));
  assert.equal(win.$('#title').textContent, 'Song');
  assert.equal(win.$('#artist').textContent, 'Artist');
});

test('the card hides in the menu, while loading the game and after Quaver closes', () => {
  const win = open('now-playing', '');
  const es = stream(win);
  es.emit('snapshot', { game: 'gameplay', map: MAP });
  for (const state of ['menu', 'initialization', 'closed']) {
    es.emit('game.state', { state: 'gameplay' });
    assert.ok(visible(win.$('#card')), 'visible before ' + state);
    es.emit('game.state', { state });
    assert.ok(!visible(win.$('#card')), state);
  }
});

test('a snapshot without a map hides the card', () => {
  const win = open('now-playing', '');
  const es = stream(win);
  es.emit('snapshot', { game: 'gameplay', map: MAP });
  es.emit('snapshot', { game: 'gameplay', map: null });
  assert.ok(!visible(win.$('#card')));
});

test('map.started and score.saved show the card again', () => {
  const win = open('now-playing', '');
  const es = stream(win);
  es.emit('map.started', MAP);
  assert.ok(visible(win.$('#card')));
  assert.equal(win.$('#title').textContent, 'Song');
  es.emit('game.state', { state: 'menu' });
  assert.ok(!visible(win.$('#card')));
  es.emit('score.saved', { score: {} });
  es.emit('game.state', { state: 'results' });
  assert.ok(visible(win.$('#card')));
});

test('autohide hides the card after the delay and a new map brings it back', async () => {
  const win = open('now-playing', 'autohide=5');
  const es = stream(win);
  es.emit('map.started', MAP);
  await win.advance(4999);
  assert.ok(visible(win.$('#card')));
  await win.advance(1);
  assert.ok(!visible(win.$('#card')));
  es.emit('game.state', { state: 'gameplay' });
  assert.ok(!visible(win.$('#card')), 'a state change does not undo the autohide');
  es.emit('map.started', { ...MAP, title: 'Next' });
  assert.ok(visible(win.$('#card')));
  assert.equal(win.$('#title').textContent, 'Next');
  await win.advance(5000);
  assert.ok(!visible(win.$('#card')));
});

test('later snapshots after a reconnect do not restart the autohide timer', async () => {
  const win = open('now-playing', 'autohide=5');
  const es = stream(win);
  es.emit('snapshot', { game: 'gameplay', map: MAP });
  await win.advance(3000);
  es.emit('snapshot', { game: 'gameplay', map: MAP });
  await win.advance(2000);
  assert.ok(!visible(win.$('#card')));
});

test('titles are shown as text and long values are cut', () => {
  const win = open('now-playing', '');
  const es = stream(win);
  es.emit('map.started', { ...MAP, title: '<b>bold</b> & "q"', artist: 'x'.repeat(5000) });
  assert.equal(win.$('#title').textContent, '<b>bold</b> & "q"');
  assert.equal(win.$('#title').children.length, 0);
  assert.ok(win.$('#artist').textContent.length <= 200);
});

test('malformed frames are ignored', () => {
  const win = open('now-playing', '');
  const es = stream(win);
  es.emit('map.started', MAP);
  es.emit('map.started', 'not json');
  es.emit('map.started', 'null');
  es.emit('map.started', '{"data":5}');
  assert.equal(win.$('#title').textContent, 'Song');
});

test('a stream the server ends is reopened after five seconds and keeps delivering', async () => {
  const win = open('now-playing', '');
  const first = stream(win);
  first.fail(true);
  assert.equal(win.eventSources.length, 1);
  await win.advance(5000);
  assert.equal(win.eventSources.length, 2);
  assert.ok(first.closed);
  const second = stream(win);
  second.emit('map.started', MAP);
  assert.equal(win.$('#title').textContent, 'Song');
});

run();
