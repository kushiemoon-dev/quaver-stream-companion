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

const DETAIL = { ...MAP, creator: 'Mapper', bpm: 150, length_ms: 125000, regular_notes: 900, long_notes: 150, mines: 0, md5: 'abc' };
const metaText = (win) => win.$('#meta').textContent;
const NB = String.fromCharCode(160);

test('the details line stays hidden unless meta=1', () => {
  const win = open('now-playing', '');
  const es = stream(win);
  es.emit('map.started', DETAIL);
  assert.equal(metaText(win), '');
  assert.ok(win.$('#meta').hidden);
});

test('meta=1 shows mapper, BPM, length and counts and leaves out zero values', () => {
  const win = open('now-playing', 'meta=1');
  const es = stream(win);
  es.emit('map.started', DETAIL);
  assert.ok(!win.$('#meta').hidden);
  assert.equal(metaText(win), ['by Mapper', '150' + NB + 'BPM', '2:05', '900' + NB + 'notes', '150' + NB + 'LN'].join(' · '));
  es.emit('map.started', { ...DETAIL, creator: '', bpm: 0, length_ms: 0, regular_notes: 0, long_notes: 0 });
  assert.equal(metaText(win), '');
  assert.ok(win.$('#meta').hidden);
});

test('a score of the shown map applies its rate to the BPM and the length, a new map resets it', () => {
  const win = open('now-playing', 'meta=1');
  const es = stream(win);
  es.emit('map.started', DETAIL);
  es.emit('score.saved', { score: { rate: 1.5 }, map: { md5: 'abc' } });
  assert.equal(metaText(win), ['by Mapper', '225' + NB + 'BPM', '1:23', '900' + NB + 'notes', '150' + NB + 'LN'].join(' · '));
  es.emit('score.saved', { score: { rate: 2 }, map: { md5: 'other' } });
  assert.match(metaText(win), /150.BPM . 2:05/);
  es.emit('score.saved', { score: { rate: 1.5 }, map: { md5: 'abc' } });
  es.emit('map.started', DETAIL);
  assert.match(metaText(win), /150.BPM . 2:05/);
});

test('the snapshot rate comes from the last score of the same map only', () => {
  const win = open('now-playing', 'meta=1');
  const es = stream(win);
  es.emit('snapshot', { game: 'gameplay', map: DETAIL, last_score: { score: { rate: 1.5 }, map: { md5: 'abc' } } });
  assert.match(metaText(win), /225.BPM . 1:23/);
  es.emit('snapshot', { game: 'gameplay', map: DETAIL, last_score: { score: { rate: 1.5 }, map: { md5: 'zzz' } } });
  assert.match(metaText(win), /150.BPM . 2:05/);
  es.emit('snapshot', { game: 'gameplay', map: DETAIL });
  assert.match(metaText(win), /150.BPM/);
});

test('the details follow the language and the demo shows them', () => {
  const win = open('now-playing', 'demo=1&meta=1&lang=de');
  assert.equal(metaText(win), ['von Example Mapper', '150' + NB + 'BPM', '2:00', '900' + NB + 'Noten', '150' + NB + 'LN'].join(' · '));
  assert.ok(visible(win.$('#card')));
});

run();
