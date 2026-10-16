'use strict';
const assert = require('node:assert/strict');
const dom = require('../testsupport/dom.js');
const { test, run } = require('../testsupport/run.js');

const page = (html, opts) => {
  const win = dom.createWindow(opts);
  dom.parseInto(win.document, html);
  return win;
};
const SAMPLE = '<!doctype html><html><head><title>T</title><link rel="icon" href="data:,"></head>'
  + '<body><div id="a" class="x y"><span class="x" data-k="v-w">one &amp; two</span><p><b>deep</b></p></div>'
  + '<ul><li>1</li><li class="x">2</li></ul><input id="in" value="q"><script src="/s.js"></script><script>var inline = 1;</script></body></html>';

test('markup is parsed into elements, text, attributes and recorded scripts', () => {
  const win = page(SAMPLE);
  assert.equal(win.$('#a').className, 'x y');
  assert.equal(win.$('span').textContent, 'one & two');
  assert.equal(win.$('#a').textContent, 'one & twodeep');
  assert.equal(win.$$('li').length, 2);
  assert.equal(win.$('#in').getAttribute('value'), 'q');
  assert.equal(win.$('link').href, 'data:,');
  assert.deepEqual(win.document.scripts.map((s) => s.src), ['/s.js', null]);
});

test('selectors cover tag, id, class, attributes, descendants, children and lists', () => {
  const win = page(SAMPLE);
  assert.equal(win.$$('.x').length, 3);
  assert.equal(win.$$('li.x').length, 1);
  assert.equal(win.$('[data-k="v-w"]').localName, 'span');
  assert.equal(win.$$('[data-k]').length, 1);
  assert.equal(win.$$('#a b').length, 1);
  assert.equal(win.$$('#a > b').length, 0);
  assert.equal(win.$$('#a > p > b').length, 1);
  assert.equal(win.$$('li, span').length, 3);
  assert.equal(win.$('#missing'), null);
  assert.equal(win.document.getElementById('a'), win.$('#a'));
  assert.equal(win.$('b').closest('#a'), win.$('#a'));
  assert.throws(() => win.$$('li:hover'), /unsupported pseudo class/);
  assert.throws(() => win.$$('li['), /unsupported selector/);
});

test('selectors accept spaces inside quoted values and the supported pseudo classes', () => {
  const win = page('<ul><li data-k="a b">1</li><li class="x">2</li><li disabled>3</li></ul>');
  assert.equal(win.$$('[data-k="a b"]').length, 1);
  assert.equal(win.$$('li:not(.x)').length, 2);
  assert.equal(win.$('li:first-child').textContent, '1');
  assert.equal(win.$('li:last-child').textContent, '3');
  assert.equal(win.$$('li:disabled').length, 1);
});

test('Date follows the fake clock', async () => {
  const win = page("");
  const t0 = win.Date.now();
  await win.advance(1500);
  assert.equal(win.Date.now() - t0, 1500);
  assert.equal(new win.Date().getTime(), win.Date.now());
});

test('classList, attributes, hidden and dataset behave like the DOM', () => {
  const win = page(SAMPLE);
  const el = win.$('#a');
  el.classList.add('z');
  el.classList.remove('x');
  assert.equal(el.className, 'y z');
  assert.equal(el.classList.toggle('y'), false);
  assert.equal(el.classList.toggle('q', true), true);
  assert.equal(el.className, 'z q');
  el.hidden = true;
  assert.equal(el.getAttribute('hidden'), '');
  el.hidden = false;
  assert.equal(el.hasAttribute('hidden'), false);
  el.setAttribute('aria-label', 5);
  assert.equal(el.getAttribute('aria-label'), '5');
  el.dataset.dense = '1';
  assert.equal(el.dataset.dense, '1');
  el.style.setProperty('--scale', '2');
  assert.equal(el.style.getPropertyValue('--scale'), '2');
});

test('textContent and replaceChildren rebuild the children and markup stays text', () => {
  const win = page(SAMPLE);
  const el = win.$('#a');
  el.textContent = '<b>x</b>';
  assert.equal(el.children.length, 0);
  assert.equal(el.textContent, '<b>x</b>');
  const child = win.document.createElement('i');
  el.replaceChildren(child, 'tail');
  assert.equal(el.children.length, 1);
  assert.equal(el.textContent, 'tail');
  assert.equal(child.parent, el);
  el.textContent = '';
  assert.equal(el.childNodes.length, 0);
  assert.equal(child.parent, null);
});

test('events bubble from the target to the document and the window, and can be stopped', () => {
  const win = page(SAMPLE);
  const seen = [];
  win.document.addEventListener('click', () => seen.push('document'));
  win.addEventListener('click', () => seen.push('window'));
  win.$('#a').addEventListener('click', (e) => seen.push('a:' + e.target.localName));
  win.$('b').onclick = () => seen.push('b');
  win.$('b').click();
  assert.deepEqual(seen, ['b', 'a:b', 'document', 'window']);
  seen.length = 0;
  win.$('#a').addEventListener('keydown', (e) => { e.preventDefault(); e.stopPropagation(); });
  win.document.addEventListener('keydown', () => seen.push('late'));
  const ev = new dom.Event('keydown', { key: 'Escape' });
  win.$('b').dispatchEvent(ev);
  assert.equal(ev.defaultPrevented, true);
  assert.equal(ev.key, 'Escape');
  assert.deepEqual(seen, []);
});

test('focus moves document.activeElement', () => {
  const win = page(SAMPLE);
  win.$('#in').focus();
  assert.equal(win.document.activeElement, win.$('#in'));
  win.$('#in').blur();
  assert.equal(win.document.activeElement, win.document.body);
});

test('fake timers run in order, nested timers fit the window and intervals repeat', async () => {
  const win = dom.createWindow();
  const log = [];
  win.setTimeout(() => { log.push('b'); win.setTimeout(() => log.push('c'), 10); }, 20);
  win.setTimeout(() => log.push('a'), 10);
  const iv = win.setInterval(() => log.push('i'), 15);
  const gone = win.setTimeout(() => log.push('never'), 5);
  win.clearTimeout(gone);
  win.clock.advance(30);
  assert.deepEqual(log, ['a', 'i', 'b', 'i', 'c']);
  win.clearInterval(iv);
  assert.equal(win.clock.pending(), 0);
  assert.equal(win.clock.now(), 30);
});

test('fetch answers canned routes, records calls and fails like the network', async () => {
  const win = dom.createWindow({ routes: { '/api/a': dom.json({ n: 1 }), 'POST /api/b': (c) => dom.json({ got: c.body }, 201), '/api/c': dom.text('plain', 500) } });
  const a = await win.fetch('/api/a?x=1');
  assert.deepEqual([a.ok, a.status, await a.json()], [true, 200, { n: 1 }]);
  const b = await win.fetch('/api/b', { method: 'post', body: 'payload' });
  assert.deepEqual([b.status, await b.json()], [201, { got: 'payload' }]);
  const c = await win.fetch('/api/c');
  assert.deepEqual([c.ok, await c.text()], [false, 'plain']);
  await assert.rejects(c.json(), SyntaxError);
  await assert.rejects(win.fetch('/api/none'), TypeError);
  await assert.rejects(win.fetch('/api/b'), TypeError, 'a route bound to POST does not answer GET');
  assert.deepEqual(win.fetchCalls.map((x) => x.method + ' ' + x.url), ['GET /api/a?x=1', 'POST /api/b', 'GET /api/c', 'GET /api/none', 'GET /api/b']);
});

test('fetch honours abort signals before and during the request', async () => {
  const d = dom.deferred();
  const win = dom.createWindow({ routes: { '/slow': () => d.promise } });
  const ctl = new AbortController();
  const pending = win.fetch('/slow', { signal: ctl.signal });
  ctl.abort();
  await assert.rejects(pending, { name: 'AbortError' });
  await assert.rejects(win.fetch('/slow', { signal: ctl.signal }), { name: 'AbortError' });
});

test('EventSource instances are recorded and emit, open and fail on demand', () => {
  const win = dom.createWindow();
  const es = new win.EventSource('/events');
  const got = [];
  let opened = 0, errored = 0;
  es.addEventListener('score.saved', (e) => got.push(e.data));
  es.onopen = () => opened++;
  es.onerror = () => errored++;
  es.open();
  es.emit('score.saved', { id: 1 });
  es.emit('score.saved', 'raw');
  assert.deepEqual(got, ['{"type":"score.saved","data":{"id":1}}', 'raw']);
  es.fail(false);
  assert.equal(es.readyState, 0);
  es.fail(true);
  assert.equal(es.readyState, 2);
  assert.deepEqual([opened, errored, win.eventSources.length], [1, 2, 1]);
});

test('storage, clipboard and console are observable and can be made to fail', async () => {
  const win = dom.createWindow();
  win.localStorage.setItem('k', 5);
  assert.equal(win.localStorage.getItem('k'), '5');
  assert.equal(win.localStorage.getItem('none'), null);
  assert.equal(win.localStorage.length, 1);
  await win.navigator.clipboard.writeText('hello');
  assert.deepEqual(win.clipboard.written, ['hello']);
  win.clipboard.fail = true;
  await assert.rejects(win.navigator.clipboard.writeText('x'), /denied/);
  win.console.error('boom', 1);
  assert.deepEqual(win.logs, [{ level: 'error', args: ['boom', 1] }]);
  const blocked = dom.createWindow({ storageThrows: true });
  assert.throws(() => blocked.localStorage.getItem('k'), /blocked/);
});

test('location parses the url, fires hashchange and records navigations', () => {
  const win = dom.createWindow({ url: 'http://127.0.0.1:7878/app?x=1#/status', language: 'fr-FR' });
  assert.deepEqual([win.location.pathname, win.location.search, win.location.hash, win.location.protocol], ['/app', '?x=1', '#/status', 'http:']);
  assert.equal(win.navigator.language, 'fr-FR');
  const seen = [];
  win.addEventListener('hashchange', (e) => seen.push(e.newURL));
  win.location.hash = '#/history';
  win.location.hash = '#/history';
  assert.deepEqual(seen, ['http://127.0.0.1:7878/app?x=1#/history']);
  win.location.assign('/elsewhere');
  assert.deepEqual(win.location.navigations, ['/elsewhere']);
  win.history.replaceState(null, '', '/app?x=2');
  assert.equal(win.location.search, '?x=2');
});

test('scripts run as classic scripts that share top level declarations', () => {
  const win = dom.createWindow({ globals: { extra: 7 } });
  dom.runScript(win, null, 'const shared = extra + 1; window.seen = typeof module;');
  dom.runScript(win, null, 'window.value = shared * 2;');
  assert.equal(win.value, 16);
  assert.equal(win.seen, 'undefined');
});

run();
