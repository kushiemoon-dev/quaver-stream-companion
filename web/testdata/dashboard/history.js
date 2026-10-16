// pages/history.js and history-model.js: the play history table, its filters, limit, empty and error states.
'use strict';
const assert = require('node:assert/strict');
const path = require('node:path');
const { test, run } = require('../../testsupport/run.js');
const dom = require('../../testsupport/dom.js');
const H = require('./helpers.js');

const model = require(path.join(H.WEB, 'dashboard', 'pages', 'history-model.js'));
const NBSP = String.fromCharCode(160);

const play = (over = {}) => ({
  title: 'Song', artist: 'Artist', difficulty_name: 'Hard', difficulty: 5.43, rate: 1.2, accuracy: 98.567, max_combo: 321, grade: 'S',
  played_at: '2026-01-01T10:00:00Z', mods: ['Mirror'], judgments: { marv: 100, perf: 20, great: 3, good: 2, okay: 1, miss: 0 },
  local_rank: 2, local_total: 5, ...over,
});
const PLAYS = [
  play({ title: 'Best', is_pb: true, is_local_best: true, full_combo: true }),
  play({ title: 'Combo', full_combo: true, grade: 'A', local_rank: 0 }),
  play({ title: 'Broken', failed: true, grade: 'F', mods: [] }),
];

async function open(opts = {}) {
  const state = { calls: 0, ...opts.state };
  const handler = opts.handler || (() => dom.json(opts.plays === undefined ? PLAYS : opts.plays));
  const win = await H.open({ hash: '#/history', language: opts.language, routes: { '/api/history': (c) => { state.calls++; return handler(c, state); }, ...(opts.routes || {}) } });
  return { win, state };
}

const rows = (win) => win.$$('tbody tr');
const titles = (win) => rows(win).map((r) => r.querySelector('.hist-title').textContent);
const live = (win) => win.$('p[aria-live="polite"].sr-only').textContent;
const radio = (win, name) => win.$$('input[type="radio"]').filter((r) => r.value === name)[0];
const choose = (win, name) => { radio(win, name).checked = true; radio(win, name).dispatchEvent(new dom.Event('change')); };
const limitSelect = (win) => win.$('#hist-limit');
const setLimit = (win, v) => { limitSelect(win).value = String(v); limitSelect(win).dispatchEvent(new dom.Event('change')); };
const queries = (win) => win.fetchCalls.filter((c) => c.path === '/api/history').map((c) => c.query);
const results = (win) => win.$('.history-results');

test('the page asks for 50 plays and shows one row per play', async () => {
  const { win } = await open();
  assert.deepEqual(queries(win), ['?limit=50']);
  assert.deepEqual(titles(win), ['Best', 'Combo', 'Broken']);
  assert.equal(results(win).hasAttribute('aria-busy'), false);
  assert.deepEqual(win.$$('th').map((th) => th.textContent), ['Map', 'Difficulty', 'Result', 'Accuracy', 'Max combo', 'Judgments']);
  assert.equal(win.$('caption').textContent, H.en['history.caption']);
  assert.equal(win.$('.table-wrap').getAttribute('tabindex'), '0');
  assert.equal(limitSelect(win).children.length, 4);
});

test('a row shows the map, difficulty with rate, grade, rank, badges, accuracy, combo and judgments', async () => {
  const { win } = await open();
  const cells = rows(win)[0].children;
  assert.equal(cells[0].querySelector('.hist-artist').textContent, 'Artist');
  assert.match(cells[0].querySelector('.hist-date').textContent, /2026/);
  assert.equal(cells[1].children[0].textContent, 'Hard');
  assert.equal(cells[1].querySelector('.num').textContent, String.fromCharCode(9733) + ' 5.4 1.2x');
  assert.deepEqual(cells[1].querySelectorAll('.chip').map((c) => c.textContent), ['Mirror']);
  assert.equal(cells[2].querySelector('.gr').textContent, 'S');
  assert.ok(cells[2].querySelector('.gr').classList.contains('gr-gold'));
  const pills = cells[2].querySelectorAll('.pill');
  assert.deepEqual(pills.map((p) => p.textContent.trim()), ['#2 ' + H.t('history.rank', { rank: 2, total: 5 }), 'PB', 'Best', H.en['status.fc']]);
  assert.equal(cells[3].textContent, '98.57%');
  assert.equal(cells[4].textContent, '321x');
  assert.deepEqual(cells[5].querySelectorAll('.jn').map((j) => j.textContent.trim().split(/\s+/)[0]), ['100', '20', '3', '2', '1', '0']);
  assert.ok(cells[5].querySelector('.j-marv'));
  const noRank = rows(win)[1].children[2].querySelectorAll('.pill');
  assert.deepEqual(noRank.map((p) => p.textContent), [H.en['status.fc']]);
});

test('badge mods are not repeated as chips and extra mods collapse into a plus chip with the names', async () => {
  const { win } = await open({ plays: [play({ mods: ['Paused', 'A', 'B', 'C', 'D', 'E'], paused: true }), play({ mods: ['Autoplay', 'X'] })] });
  const first = rows(win)[0].children[1].querySelectorAll('.chip');
  assert.deepEqual(first.map((c) => c.textContent.trim()), ['A', 'B', 'C', '+2 (D, E)']);
  assert.equal(first[3].getAttribute('title'), 'D, E');
  assert.deepEqual(rows(win)[0].children[2].querySelectorAll('.pill').map((p) => p.textContent).slice(-1), ['Paused']);
  const second = rows(win)[1].children[1].querySelectorAll('.chip').map((c) => c.textContent);
  assert.deepEqual(second, ['Autoplay', 'X']);
});

test('missing values show neutral defaults and hostile text stays text', async () => {
  const evil = '<img src=x onerror=alert(1)>';
  const { win } = await open({ plays: [{ title: evil, grade: '', judgments: null, played_at: 'last tuesday' }, null, 5, 'x'] });
  assert.equal(rows(win).length, 1);
  const cells = rows(win)[0].children;
  assert.equal(cells[0].querySelector('.hist-title').textContent, evil);
  assert.equal(win.$$('img').length, 0);
  assert.equal(cells[0].querySelector('.hist-date').textContent, 'last tuesday');
  assert.equal(cells[2].querySelector('.gr').textContent, '-');
  assert.ok(cells[2].querySelector('.gr').classList.contains('gr-muted'));
  assert.equal(cells[3].textContent, '0.00%');
  assert.equal(cells[4].textContent, '0x');
  assert.deepEqual(cells[5].querySelectorAll('.jn').map((j) => j.textContent.trim().split(/\s+/)[0]), ['0', '0', '0', '0', '0', '0']);
});

test('numbers and the date follow the page language', async () => {
  const { win } = await open({ language: 'fr-FR' });
  const cells = rows(win)[0].children;
  assert.equal(cells[3].textContent, '98,57' + NBSP + '%');
  assert.equal(cells[1].querySelector('.num').textContent, String.fromCharCode(9733) + ' 5,4 1,2x');
  assert.equal(win.$('h1').textContent, H.catalog('fr')['nav.history']);
});

test('the result count is announced for the shown rows', async () => {
  const { win } = await open();
  assert.equal(live(win), '');
  await win.advance(16);
  assert.equal(live(win), H.t('history.count', { shown: 3, total: 3 }));
});

test('filters keep PB, full combo or failed plays and say when nothing matches', async () => {
  const { win } = await open();
  choose(win, 'pb');
  assert.deepEqual(titles(win), ['Best']);
  choose(win, 'fc');
  assert.deepEqual(titles(win), ['Best', 'Combo']);
  await win.advance(16);
  assert.equal(live(win), H.t('history.count', { shown: 2, total: 3 }));
  choose(win, 'failed');
  assert.deepEqual(titles(win), ['Broken']);
  choose(win, 'all');
  assert.equal(rows(win).length, 3);
  assert.equal(radio(win, 'all').checked, true);
});

test('a filter with no match shows its own empty card', async () => {
  const { win } = await open({ plays: [play()] });
  choose(win, 'failed');
  assert.equal(results(win).querySelector('.empty-title').textContent, H.en['history.empty_filtered']);
  assert.equal(win.$$('table').length, 0);
  await win.advance(16);
  assert.equal(live(win), H.en['history.empty_filtered']);
});

test('no plays at all shows the empty card, also for an answer that is not a list', async () => {
  for (const plays of [[], {}, null, 'x']) {
    const { win } = await open({ plays });
    assert.equal(results(win).querySelector('.empty-title').textContent, H.en['history.empty']);
    await win.advance(16);
    assert.equal(live(win), H.en['history.empty']);
  }
});

test('changing the limit loads again with the new value and a value outside the list falls back to 50', async () => {
  const { win } = await open();
  setLimit(win, 100);
  await win.flush();
  setLimit(win, 7);
  await win.flush();
  assert.deepEqual(queries(win), ['?limit=50', '?limit=100', '?limit=50']);
  setLimit(win, 200);
  assert.equal(results(win).getAttribute('aria-busy'), 'true');
  assert.equal(results(win).textContent, H.en['history.loading']);
  await win.flush();
  assert.equal(results(win).hasAttribute('aria-busy'), false);
});

test('a filter chosen while loading waits for the result and then applies', async () => {
  const gate = dom.deferred();
  const { win } = await open({ handler: (c, s) => (s.calls === 1 ? dom.json(PLAYS) : gate.promise) });
  setLimit(win, 25);
  choose(win, 'fc');
  assert.equal(results(win).textContent, H.en['history.loading']);
  gate.resolve(dom.json(PLAYS));
  await win.flush();
  assert.deepEqual(titles(win), ['Best', 'Combo']);
});

test('an older answer never replaces a newer one', async () => {
  const slow = dom.deferred();
  const { win } = await open({ handler: (c, s) => (s.calls === 2 ? slow.promise : dom.json([play({ title: 'New' })])) });
  setLimit(win, 25);
  setLimit(win, 100);
  await win.flush();
  assert.deepEqual(titles(win), ['New']);
  slow.resolve(dom.json([play({ title: 'Old' })]));
  await win.flush();
  assert.deepEqual(titles(win), ['New']);
});

test('a 503 explains that Quaver cannot be read yet and links to Status', async () => {
  const { win } = await open({ handler: () => dom.json({ error: 'Quaver is writing the database, try again' }, 503) });
  const alert = results(win).querySelector('[role="alert"]');
  assert.match(alert.textContent, new RegExp('^' + H.en['history.unavailable'].slice(0, 20)));
  assert.equal(alert.querySelector('a').getAttribute('href'), '#/status');
  assert.equal(alert.querySelector('a').textContent, H.en['history.unavailable_link']);
});

test('another server error shows the message and a retry that reloads and focuses the limit', async () => {
  const { win, state } = await open({ handler: (c, s) => (s.calls === 1 ? dom.json({}, 500) : dom.json(PLAYS)) });
  assert.equal(results(win).querySelector('[role="alert"]').textContent, H.t('history.load_failed', { message: H.t('error.server', { status: 500 }) }));
  assert.equal(results(win).hasAttribute('aria-busy'), false);
  choose(win, 'fc');
  assert.equal(win.$$('table').length, 0);
  results(win).querySelector('button').click();
  await win.flush();
  assert.equal(state.calls, 2);
  assert.deepEqual(titles(win), ['Best', 'Combo']);
  assert.equal(win.document.activeElement, limitSelect(win));
});

test('a failed retry keeps the focus on the new retry button', async () => {
  const { win } = await open({ handler: () => dom.json({}, 500) });
  results(win).querySelector('button').click();
  await win.flush();
  assert.equal(win.document.activeElement, results(win).querySelector('button'));
});

test('an unreachable companion shows the network message', async () => {
  const { win } = await open({ handler: () => new TypeError('Failed to fetch') });
  assert.equal(results(win).querySelector('[role="alert"]').textContent, H.t('history.load_failed', { message: H.en['error.network'] }));
});

test('no answer within 8 seconds shows the timeout', async () => {
  const { win } = await open({ handler: () => new Promise(() => {}) });
  assert.equal(results(win).textContent, H.en['history.loading']);
  await win.advance(8000);
  assert.equal(results(win).querySelector('[role="alert"]').textContent, H.t('history.load_failed', { message: H.en['history.timeout'] }));
});

test('leaving the page while loading shows no error and keeps nothing running', async () => {
  const { win } = await open({ handler: () => new Promise(() => {}) });
  win.location.hash = '#/status';
  await win.flush();
  await win.advance(8000);
  assert.equal(win.$('h1').textContent, H.en['nav.status']);
  assert.equal(win.logs.length, 0);
  assert.equal(win.clock.pending() < 20, true);
});

test('coming back starts again at the all filter and 50 plays', async () => {
  const { win } = await open();
  choose(win, 'pb');
  win.location.hash = '#/status';
  await win.flush();
  win.location.hash = '#/history';
  await win.flush();
  assert.equal(rows(win).length, 3);
  assert.equal(radio(win, 'all').checked, true);
});

test('the model formats rates, stars, accuracy and dates with the fallbacks', () => {
  assert.equal(model.formatRate(1, 'en'), '');
  assert.equal(model.formatRate('x', 'en'), '');
  assert.equal(model.formatRate(0.5, 'fr'), '0,5x');
  assert.equal(model.formatRate(1.254, 'en'), '1.25x');
  assert.equal(model.formatStars('bad', 'en'), '0.0');
  assert.equal(model.formatAccuracy(undefined, 'de'), '0,00' + NBSP + '%');
  assert.equal(model.formatDecimal(1.5, 1, 'fr'), '1,5');
  assert.equal(model.formatPlayedAt(undefined, 'en'), '');
  assert.equal(model.formatPlayedAt('2026-13-45T99:00:00Z', 'en'), '2026-13-45T99:00:00Z');
  assert.notEqual(model.formatPlayedAt('2026-01-01T10:00:00Z', 'xx'), '2026-01-01T10:00:00Z');
  assert.equal(model.gradeClass('s'), 'gold');
  assert.equal(model.gradeClass('constructor'), 'muted');
  assert.equal(model.rankLabel({ local_rank: 5000 }), '#999');
  assert.equal(model.rankLabel({ local_rank: 1.5 }), '');
  assert.equal(model.rankLabel(null), '');
  assert.deepEqual(model.modChips('nope'), { shown: [], extra: 0, hidden: [] });
  assert.deepEqual(model.modsWithoutBadges(['Paused', 'Autoplay', 'X'], { paused: true }), ['Autoplay', 'X']);
  assert.deepEqual(model.modsWithoutBadges(null, null), []);
  assert.deepEqual(model.filterPlays('x', 'pb'), []);
  assert.equal(model.filterPlays([{ is_pb: true }, {}], 'constructor').length, 2);
  assert.equal(model.parseLimit('200'), 200);
  assert.equal(model.parseLimit('abc'), 50);
});

test('the model shows the raw date when the date formatter fails and the comma when the number formatter fails', () => {
  const dateFormat = Intl.DateTimeFormat;
  const numberFormat = Intl.NumberFormat;
  try {
    Intl.DateTimeFormat = function () { throw new RangeError('no'); };
    Intl.NumberFormat = function () { throw new RangeError('no'); };
    assert.equal(model.formatPlayedAt('2026-01-01T10:00:00Z', 'en'), '2026-01-01T10:00:00Z');
    assert.equal(model.formatDecimal(1.5, 1, 'de'), '1,5');
  } finally {
    Intl.DateTimeFormat = dateFormat;
    Intl.NumberFormat = numberFormat;
  }
});

run();
