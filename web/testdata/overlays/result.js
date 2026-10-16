'use strict';
const assert = require('node:assert/strict');
const { test, run } = require('../../testsupport/run.js');
const { open, stream, visible } = require('./helpers.js');

const PLAY = {
  score: { id: 7, grade: 'A', accuracy: 91.234, max_combo: 300, rate: 1, mods: [], judgments: { marv: 10, perf: 9, great: 8, good: 7, okay: 6, miss: 5 } },
  map: { title: 'Song', difficulty_name: 'Insane', difficulty: 22.456 },
};
const chips = (win) => win.$$('.chip').map((c) => c.textContent);

test('demo shows the example result with grade, judgments and chips', () => {
  const win = open('result', 'demo=1');
  assert.ok(visible(win.$('#card')));
  assert.equal(win.$('.title').textContent, 'Example Song');
  assert.equal(win.$$('.sub')[0].textContent, 'Hard ' + String.fromCharCode(183) + ' 18.50' + String.fromCharCode(9733));
  assert.equal(win.$('.grade').textContent, 'S');
  assert.ok(win.$('.grade').classList.contains('gold'));
  assert.equal(win.$('.big').textContent, '97.42%');
  assert.equal(win.$('.j-marv').textContent, 'Marvelous640');
  assert.equal(win.$('.j-miss').textContent, 'Miss3');
  assert.deepEqual(chips(win), ['1.15x', 'Mirror', '#1', 'PB', 'Best']);
  assert.equal(win.eventSources.length, 0);
  assert.equal(win.document.title, 'QSC last result');
});

test('demo follows the language', () => {
  const win = open('result', 'demo=1&lang=fr');
  assert.match(win.$('.big').textContent, /^97,42\s%$/);
  assert.match(win.$$('.sub')[1].textContent, /^Combo max 812x$/);
  assert.equal(win.$('.j-marv span').textContent, 'Marvelous');
});

test('the live card waits for a score and ignores a snapshot without one', () => {
  const win = open('result', '');
  const es = stream(win);
  assert.ok(!visible(win.$('#card')));
  es.emit('snapshot', { game: 'menu', last_score: null });
  assert.ok(!visible(win.$('#card')));
});

test('a snapshot with the last score renders it and a later score replaces it', () => {
  const win = open('result', '');
  const es = stream(win);
  es.emit('snapshot', { last_score: PLAY });
  assert.ok(visible(win.$('#card')));
  assert.equal(win.$('.grade').textContent, 'A');
  assert.ok(win.$('.grade').classList.contains('green'));
  assert.equal(win.$('.big').textContent, '91.23%');
  assert.equal(win.$('.title').textContent, 'Song');
  es.emit('score.saved', { ...PLAY, score: { ...PLAY.score, id: 8, grade: 'S', accuracy: 99 } });
  assert.equal(win.$('.grade').textContent, 'S');
  assert.equal(win.$('.big').textContent, '99.00%');
});

test('a reconnect snapshot with the same score id does not rebuild the card', () => {
  const win = open('result', '');
  const es = stream(win);
  es.emit('score.saved', PLAY);
  const before = win.$('.title');
  es.emit('snapshot', { last_score: PLAY });
  assert.equal(win.$('.title'), before);
});

test('autohide hides the card and a new score shows it again', async () => {
  const win = open('result', 'autohide=4');
  const es = stream(win);
  es.emit('score.saved', PLAY);
  await win.advance(3999);
  assert.ok(visible(win.$('#card')));
  await win.advance(1);
  assert.ok(!visible(win.$('#card')));
  es.emit('score.saved', { ...PLAY, score: { ...PLAY.score, id: 9 } });
  assert.ok(visible(win.$('#card')));
});

test('badges, rank, rate and extra mods become chips', () => {
  const win = open('result', '');
  stream(win).emit('score.saved', {
    ...PLAY,
    score: { ...PLAY.score, rate: 1.2, mods: ['A', 'B', 'C', 'D', 'E', 'F'] },
    is_pb: true, full_combo: true, local_rank: 2,
  });
  const got = chips(win);
  assert.equal(got[0], '1.2x');
  assert.ok(got.includes('#2'));
  assert.ok(got.includes('PB'));
  assert.ok(got.includes('FC'));
  assert.ok(got.some((c) => /^\+\d+$/.test(c)), 'extra mods are counted');
});

test('failed, paused and autoplay runs are marked', () => {
  const win = open('result', 'lang=de');
  stream(win).emit('score.saved', { ...PLAY, failed: true, paused: true, autoplay: true });
  assert.equal(chips(win).length, 3);
});

test('an empty or odd score still draws a card and non-objects are ignored', () => {
  const win = open('result', '');
  const es = stream(win);
  es.emit('score.saved', {});
  assert.ok(visible(win.$('#card')));
  assert.equal(win.$('.grade').textContent, '-');
  assert.ok(win.$('.grade').classList.contains('muted'));
  assert.equal(win.$('.big').textContent, '0.00%');
  es.emit('score.saved', 'null');
  es.emit('score.saved', '{"data":"x"}');
  assert.equal(win.$('.grade').textContent, '-');
});

test('map titles are text and cannot add markup', () => {
  const win = open('result', '');
  stream(win).emit('score.saved', { ...PLAY, map: { title: '<img src=x onerror=alert(1)>', difficulty_name: '<b>x</b>' } });
  assert.equal(win.$('.title').textContent, '<img src=x onerror=alert(1)>');
  assert.equal(win.$$('img').length, 0);
  assert.equal(win.$$('b').filter((b) => b.parent.className.startsWith('j-')).length, 6);
});

run();
