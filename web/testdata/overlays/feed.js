// overlays/common.js with feed=parent: a framed overlay takes its events from the host page of the same origin instead of the event stream.
'use strict';
const assert = require('node:assert/strict');
const path = require('node:path');
const { test, run } = require('../../testsupport/run.js');
const dom = require('../../testsupport/dom.js');

const WEB = path.resolve(__dirname, '..', '..');
const ORIGIN = 'https://example.test';

// open loads the session overlay; framed gives the window a host page, like an iframe has.
function open(query, framed) {
  const host = { listeners: {} };
  const win = dom.openPage(WEB, 'overlays/session/index.html', {
    routes: { '/api/config': dom.json({ config: { overlay: { theme: 'saved' } } }) },
    url: ORIGIN + '/overlays/session/?' + query,
    globals: framed ? { parent: host } : {},
  });
  return { win, host };
}

function post(win, data, opts = {}) {
  win.dispatchEvent(new dom.Event('message', { bubbles: false, origin: opts.origin || ORIGIN, source: opts.source, data }));
}

const session = { started_at: '2026-01-01T00:00:00Z', plays: 3, passes: 2, fails: 1, avg_accuracy: 96.5, best_accuracy: 98.1, best_rating: 20, full_combos: 1, streak: 1, best_streak: 2 };
const plays = (win) => win.$$('.session-cell b')[0].textContent;

test('a framed overlay with feed=parent opens no stream, asks no config and renders host events', async () => {
  const { win, host } = open('feed=parent&theme=nord', true);
  await win.flush();
  assert.equal(win.eventSources.length, 0);
  assert.equal(win.fetchCalls.length, 0);
  assert.deepEqual(win.$$('link').filter((l) => l.href.startsWith('/themes/')).map((l) => l.href), ['/themes/nord.css']);
  post(win, { type: 'qsc-event', event: 'session.updated', data: session }, { source: host });
  assert.equal(plays(win), '3');
  assert.equal(win.$('#card').classList.contains('hidden'), false);
  post(win, { type: 'qsc-event', event: 'snapshot', data: { session: { ...session, plays: 4 } } }, { source: host });
  assert.equal(plays(win), '4');
});

test('messages from another origin, another window or with a wrong shape are ignored', async () => {
  const { win, host } = open('feed=parent', true);
  await win.flush();
  const good = { type: 'qsc-event', event: 'session.updated', data: session };
  post(win, good, { source: host, origin: 'https://evil.test' });
  post(win, good, { source: {} });
  post(win, good, { source: null });
  for (const bad of [null, 'x', 5, { ...good, type: 'other' }, { ...good, event: 'toString' }, { ...good, event: 5 }, { ...good, data: null }, { ...good, data: 'x' }]) post(win, bad, { source: host });
  assert.equal(win.$('#card').classList.contains('hidden'), true);
});

test('close() stops listening to the host page', async () => {
  const { win, host } = open('feed=parent', true);
  const seen = [];
  const conn = win.qscConnect({ 'session.updated': (d) => seen.push(d.plays) });
  assert.equal(conn.onopen, null);
  post(win, { type: 'qsc-event', event: 'session.updated', data: session }, { source: host });
  conn.close();
  post(win, { type: 'qsc-event', event: 'session.updated', data: session }, { source: host });
  assert.deepEqual(seen, [3]);
});

test('without a frame, or without the parameter, the overlay connects to the server as before', async () => {
  for (const [query, framed] of [['feed=parent', false], ['', true], ['feed=other', true]]) {
    const { win, host } = open(query, framed);
    await win.flush();
    assert.deepEqual(win.eventSources.map((e) => e.url), ['/events'], query);
    assert.deepEqual(win.fetchCalls.map((c) => c.path), ['/api/config'], query);
    post(win, { type: 'qsc-event', event: 'session.updated', data: session }, { source: host });
    assert.equal(win.$('#card').classList.contains('hidden'), true, query);
  }
});

run();
