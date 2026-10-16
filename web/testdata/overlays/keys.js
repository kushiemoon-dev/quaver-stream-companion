'use strict';
const assert = require('node:assert/strict');
const { test, run } = require('../../testsupport/run.js');
const { open, stream, visible, json } = require('./helpers.js');

const frame = (over) => ({ layout: '4k', lanes: 4, scratch: false, down: [true, false, false, true], presses: [5, 6, 7, 8], kps: 3, peak: 9, ...over });
const lanes = (win) => win.$$('.keys-lane');
const counts = (win) => win.$$('.keys-count').map((c) => c.textContent);
const stats = (win) => win.$$('.keys-stats b').map((b) => b.textContent);

test('demo draws four lanes with counts, KPS and peak and opens nothing', () => {
  const win = open('keys', 'demo=1');
  assert.ok(visible(win.$('#card')));
  assert.equal(lanes(win).length, 4);
  assert.deepEqual(counts(win), ['128', '111', '94', '77']);
  assert.deepEqual(stats(win), ['9', '14']);
  assert.deepEqual(lanes(win).map((l) => l.classList.contains('down')), [true, false, true, false]);
  assert.equal(win.eventSources.length, 0);
  assert.equal(win.fetchCalls.length, 0);
  assert.equal(win.document.title, 'QSC lane keys');
});

test('demo layout, scratch, counts and peak parameters shape the lanes', () => {
  assert.equal(lanes(open('keys', 'demo=1&layout=7k')).length, 7);
  assert.equal(lanes(open('keys', 'demo=1&lanes=9')).length, 9);
  assert.equal(open('keys', 'demo=1&lanes=9').$('.keys-lanes').dataset.dense, '1');
  assert.equal(open('keys', 'demo=1').$('.keys-lanes').dataset.dense, '0');
  const scratch = open('keys', 'demo=1&scratch=1');
  assert.equal(lanes(scratch).length, 5);
  assert.ok(lanes(scratch)[0].classList.contains('scratch'));
  assert.equal(lanes(scratch)[0].getAttribute('aria-label'), 'Scratch lane');
  assert.equal(lanes(scratch)[1].getAttribute('aria-label'), 'Lane 1');
  assert.equal(lanes(open('keys', 'demo=1'))[0].getAttribute('aria-label'), 'Lane 1');
  const bare = open('keys', 'demo=1&counts=0&peak=0');
  assert.ok(bare.$$('.keys-count').every((c) => c.hidden));
  assert.deepEqual(stats(bare), ['9']);
});

test('the live page starts hidden and waits for a frame', () => {
  const win = open('keys', '');
  stream(win);
  assert.ok(!visible(win.$('#card')));
  assert.equal(win.eventSources[0].url, '/events?keys=1');
});

test('a frame in a snapshot or a keys.frame event draws and updates the lanes', () => {
  const win = open('keys', '');
  const es = stream(win);
  es.emit('snapshot', { keys: frame() });
  assert.ok(visible(win.$('#card')));
  assert.deepEqual(counts(win), ['5', '6', '7', '8']);
  assert.deepEqual(stats(win), ['3', '9']);
  const first = lanes(win)[0];
  es.emit('keys.frame', frame({ down: [false, true, false, false], presses: [6, 6, 7, 8], kps: 4, peak: 10 }));
  assert.equal(lanes(win)[0], first, 'the lanes are reused while the layout stays');
  assert.deepEqual(lanes(win).map((l) => l.classList.contains('down')), [false, true, false, false]);
  assert.deepEqual(counts(win), ['6', '6', '7', '8']);
  assert.deepEqual(stats(win), ['4', '10']);
});

test('a snapshot without keys draws nothing', () => {
  const win = open('keys', '');
  stream(win).emit('snapshot', { game: 'menu' });
  assert.ok(!visible(win.$('#card')));
});

test('a different lane count or a scratch lane rebuilds the lanes', () => {
  const win = open('keys', '');
  const es = stream(win);
  es.emit('keys.frame', frame());
  es.emit('keys.frame', frame({ lanes: 7, down: [], presses: [], layout: '7k' }));
  assert.equal(lanes(win).length, 7);
  assert.ok(lanes(win).every((l) => !l.classList.contains('down')));
  assert.deepEqual(counts(win), ['0', '0', '0', '0', '0', '0', '0']);
  es.emit('keys.frame', frame({ lanes: 5, scratch: true, down: [true] }));
  assert.equal(lanes(win).length, 5);
  assert.ok(lanes(win)[0].classList.contains('scratch'));
});

test('counts=0 hides the counters and peak=0 drops the peak', () => {
  const win = open('keys', 'counts=0&peak=0');
  stream(win).emit('keys.frame', frame());
  assert.ok(win.$$('.keys-count').every((c) => c.hidden));
  assert.deepEqual(stats(win), ['3']);
});

test('frames that cannot be drawn are ignored and wild numbers are clamped', () => {
  const win = open('keys', '');
  const es = stream(win);
  es.emit('keys.frame', frame({ lanes: 0, layout: '4k' }));
  es.emit('keys.frame', frame({ lanes: 99 }));
  es.emit('keys.frame', frame({ lanes: 2.5 }));
  assert.ok(!visible(win.$('#card')));
  es.emit('keys.frame', frame({ presses: [-5, 1e30, 'x', null], kps: 1e30, peak: -1 }));
  const shown = counts(win).concat(stats(win)).map(Number);
  assert.ok(shown.every((n) => Number.isInteger(n) && n >= 0));
});

test('a map with an unsupported key mode shows the explanation and the next frame recovers', () => {
  const win = open('keys', '');
  const es = stream(win);
  es.emit('keys.frame', frame());
  es.emit('keys.frame', { layout: 'unknown', lanes: 0 });
  assert.equal(win.$('.sub').textContent, 'This map uses a key mode that is not supported yet.');
  assert.equal(lanes(win).length, 0);
  es.emit('keys.frame', frame());
  assert.equal(lanes(win).length, 4);
});

test('when the stream opens the page asks why no frames come and explains it', async () => {
  const off = open('keys', '', { routes: { '/api/status': json({ keys: { enabled: false } }) } });
  stream(off);
  await off.flush();
  assert.equal(off.fetchCalls.filter((c) => c.path === '/api/status').length, 1);
  assert.ok(visible(off.$('#card')));
  assert.equal(off.$('.sub').textContent, 'Lane keys are off. Turn them on in the dashboard settings.');

  const unsupported = open('keys', 'lang=fr', { routes: { '/api/status': json({ keys: { supported: false } }) } });
  stream(unsupported);
  await unsupported.flush();
  assert.match(unsupported.$('.sub').textContent, /^Les touches des pistes ne sont pas disponibles/);
});

test('a working key setup leaves the card hidden and a failed status check changes nothing', async () => {
  const ok = open('keys', '', { routes: { '/api/status': json({ keys: { enabled: true, supported: true } }) } });
  stream(ok);
  await ok.flush();
  assert.ok(!visible(ok.$('#card')));

  for (const routes of [{}, { '/api/status': json({}, 500) }, { '/api/status': json(null) }]) {
    const win = open('keys', '', { routes });
    const es = stream(win);
    es.emit('keys.frame', frame());
    await win.flush();
    assert.equal(lanes(win).length, 4);
    assert.ok(visible(win.$('#card')));
  }
});

test('the status check does not hide lanes that are already drawn', async () => {
  const win = open('keys', '', { routes: { '/api/status': json({ keys: { enabled: true } }) } });
  const es = stream(win);
  es.emit('keys.frame', frame());
  await win.flush();
  assert.ok(visible(win.$('#card')));
});

test('a stream the server ended is reopened with the keys flag', async () => {
  const win = open('keys', '');
  win.eventSources[0].fail(true);
  await win.advance(5000);
  assert.equal(win.eventSources.length, 2);
  assert.equal(win.eventSources[1].url, '/events?keys=1');
});

run();
