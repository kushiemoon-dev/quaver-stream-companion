// overlays/common.js: the query parameters, the look (theme and accent), the translations and the event stream connection shared by the overlays.
'use strict';
const assert = require('node:assert/strict');
const path = require('node:path');
const { test, run } = require('../../testsupport/run.js');
const dom = require('../../testsupport/dom.js');

const WEB = path.resolve(__dirname, '..', '..');

// open loads the state overlay (it only uses what common.js provides). Options: routes (fetch table), language, url (full address).
function open(query, opts = {}) {
  return dom.openPage(WEB, 'overlays/state/index.html', {
    language: opts.language, routes: opts.routes || { '/api/config': dom.json({ config: { overlay: {} } }) },
    url: opts.url || 'http://127.0.0.1:7878/overlays/state/?' + query,
  });
}

const looks = (win) => win.$$('link').filter((l) => l.href.startsWith('/themes/'));
const root = (win) => win.document.documentElement;
const config = (overlay) => ({ '/api/config': dom.json({ config: { overlay } }) });

test('without parameters the saved look is fetched and applied', async () => {
  const win = open('', { routes: config({ theme: 'night', accent: 'ff8800' }) });
  assert.equal(root(win).style.visibility, 'hidden');
  await win.flush();
  assert.deepEqual(looks(win).map((l) => l.href), ['/themes/night.css']);
  assert.equal(root(win).style.getPropertyValue('--accent'), '#ff8800');
  assert.deepEqual(win.fetchCalls.map((c) => c.path), ['/api/config']);
  assert.equal(win.fetchCalls[0].init.credentials, 'same-origin');
});

test('the URL wins over the saved look and a complete URL look skips the request', async () => {
  const win = open('theme=mine&accent=0af', { routes: config({ theme: 'night', accent: 'ff8800' }) });
  await win.flush();
  assert.deepEqual(looks(win).map((l) => l.href), ['/themes/mine.css']);
  assert.equal(root(win).style.getPropertyValue('--accent'), '#0af');
  assert.equal(win.fetchCalls.length, 0);
  const half = open('theme=mine', { routes: config({ theme: 'night', accent: 'ff8800' }) });
  await half.flush();
  assert.deepEqual(looks(half).map((l) => l.href), ['/themes/mine.css']);
  assert.equal(root(half).style.getPropertyValue('--accent'), '#ff8800');
});

test('demo mode uses the URL look only and never asks the server', async () => {
  const win = open('demo=1&theme=mine');
  await win.flush();
  assert.equal(win.fetchCalls.length, 0);
  assert.deepEqual(looks(win).map((l) => l.href), ['/themes/mine.css']);
  assert.equal(root(win).style.getPropertyValue('--accent'), '');
});

test('hostile theme and accent values are ignored', async () => {
  const bad = ['../../etc/passwd', 'a b', 'UPPER', 'x.css', '<script>', 'a/b', '%2e%2e'];
  for (const theme of bad) {
    const win = open('demo=1&theme=' + encodeURIComponent(theme) + '&accent=' + encodeURIComponent('zz; background:red'));
    await win.flush();
    assert.deepEqual(looks(win).map((l) => l.href), ['/themes/neutral-light.css'], theme);
    assert.equal(root(win).style.getPropertyValue('--accent'), '');
  }
  const win = open('demo=1&accent=12345678901');
  await win.flush();
  assert.equal(root(win).style.getPropertyValue('--accent'), '');
});

test('a saved look that is malformed is ignored too', async () => {
  const win = open('', { routes: config({ theme: '../x', accent: 'nope' }) });
  await win.flush();
  assert.deepEqual(looks(win).map((l) => l.href), ['/themes/neutral-light.css']);
  assert.equal(root(win).style.getPropertyValue('--accent'), '');
  const typed = open('', { routes: config({ theme: 5, accent: { a: 1 } }) });
  await typed.flush();
  assert.deepEqual(looks(typed).map((l) => l.href), ['/themes/neutral-light.css']);
});

test('a server that is down, slow or answering badly gives the default look', async () => {
  const answers = [{ '/api/config': dom.json({}, 500) }, { '/api/config': new TypeError('Failed to fetch') }, { '/api/config': dom.json(null) }, { '/api/config': dom.json({ config: null }) }, { '/api/config': dom.json({ config: { overlay: 'x' } }) }];
  for (const routes of answers) {
    const win = open('', { routes });
    await win.flush();
    assert.deepEqual(looks(win).map((l) => l.href), ['/themes/neutral-light.css']);
  }
  const win = open('', { routes: { '/api/config': new Promise(() => {}) } });
  await win.flush();
  assert.equal(looks(win).length, 0);
  await win.advance(1199);
  assert.equal(looks(win).length, 0);
  await win.advance(1);
  assert.deepEqual(looks(win).map((l) => l.href), ['/themes/neutral-light.css']);
});

test('a browser that cannot start the request still shows the page with the default look', async () => {
  const win = dom.openPage(WEB, 'overlays/state/index.html', {
    url: 'http://127.0.0.1:7878/overlays/state/', globals: { AbortController: function () { throw new Error('unsupported'); } },
  });
  await win.flush();
  assert.equal(root(win).style.visibility, '');
  assert.deepEqual(looks(win).map((l) => l.href), ['/themes/neutral-light.css']);
});

test('a page opened from a file does not fetch the config', async () => {
  const win = open('', { url: 'file:///C:/overlay/index.html' });
  await win.flush();
  assert.equal(win.fetchCalls.length, 0);
  assert.deepEqual(looks(win).map((l) => l.href), ['/themes/neutral-light.css']);
});

test('the page is hidden until the stylesheet loads, or 1.5 seconds pass', async () => {
  const win = open('demo=1&theme=mine');
  await win.flush();
  assert.equal(root(win).style.visibility, 'hidden');
  looks(win)[0].onload();
  assert.equal(root(win).style.visibility, '');
  const late = open('demo=1&theme=mine');
  await late.advance(1499);
  assert.equal(root(late).style.visibility, 'hidden');
  await late.advance(1);
  assert.equal(root(late).style.visibility, '');
});

test('a theme that fails to load is removed and the default look takes over, once', async () => {
  const win = open('demo=1&theme=missing&accent=abc');
  await win.flush();
  looks(win)[0].onerror();
  assert.deepEqual(looks(win).map((l) => l.href), ['/themes/neutral-light.css']);
  assert.equal(root(win).style.visibility, 'hidden');
  looks(win)[0].onerror();
  assert.equal(looks(win).length, 0);
  assert.equal(root(win).style.visibility, '');
});

test('the default theme failing reveals the page without looping', async () => {
  const win = open('demo=1');
  await win.flush();
  looks(win)[0].onerror();
  assert.equal(looks(win).length, 0);
  assert.equal(root(win).style.visibility, '');
});

test('parameters are parsed with limits and safe defaults', () => {
  const win = open('demo=1&scale=9&autohide=99999&hide=Menu,bogus,menu,%20gameplay&n=1000&best=1&sound=1&counts=0&peak=0&meta=1&lang=de');
  assert.deepEqual(JSON.parse(JSON.stringify(win.qscParams)), {
    demo: true, lang: 'de', scale: 3, autohide: 3600, hide: ['menu', 'gameplay'], n: 50, best: true, sound: true, counts: false, peak: false, meta: true,
  });
  assert.equal(root(win).lang, 'de');
  assert.equal(root(win).style.getPropertyValue('--scale'), '3');
});

test('missing or hostile parameters fall back to the defaults', () => {
  for (const query of ['', 'scale=abc&autohide=-5&n=NaN&demo=yes&best=0&sound=true&lang=xx', 'scale=0&n=-3&hide=,,&counts=1&peak=1', 'scale=__proto__&lang=%3Cb%3E']) {
    const p = open(query).qscParams;
    assert.equal(p.demo, false, query);
    assert.equal(p.scale, 1, query);
    assert.equal(p.autohide, 0, query);
    assert.equal(p.lang, 'en', query);
    assert.deepEqual(Array.from(p.hide), [], query);
    assert.equal(p.best, false);
    assert.equal(p.sound, false);
    assert.equal(p.counts, true);
    assert.equal(p.peak, true);
  }
  assert.equal(open('n=0').qscParams.n, 5);
  assert.equal(open('scale=0.1').qscParams.scale, 0.5);
});

test('the language comes from the URL, then the browser, then English', () => {
  assert.equal(open('', { language: 'fr-FR' }).qscParams.lang, 'fr');
  assert.equal(open('', { language: 'de' }).qscParams.lang, 'de');
  assert.equal(open('', { language: 'ja' }).qscParams.lang, 'en');
  assert.equal(open('lang=en', { language: 'fr' }).qscParams.lang, 'en');
});

test('qscT translates, falls back to English and then to the key', () => {
  const fr = open('lang=fr');
  assert.equal(fr.qscT('state.menu'), fr.qscCatalogs.fr['state.menu']);
  assert.notEqual(fr.qscT('state.menu'), 'state.menu');
  const only = open('lang=de');
  only.qscCatalogs.en['only.english'] = 'English only';
  assert.equal(only.qscT('only.english'), 'English only');
  assert.equal(only.qscT('missing.key'), 'missing.key');
  assert.equal(only.qscT('constructor'), 'constructor');
  assert.equal(only.qscT('__proto__'), '__proto__');
});

test('qscEl builds an element with a class and text, as text', () => {
  const win = open('demo=1');
  const el = win.qscEl('span', 'a b', '<b>x</b>');
  assert.equal(el.tagName, 'SPAN');
  assert.equal(el.className, 'a b');
  assert.equal(el.textContent, '<b>x</b>');
  assert.equal(win.qscEl('div').className, '');
  assert.equal(win.qscEl('div', '', '').textContent, '');
});

test('qscAutohide uses the URL delay, or the page fallback when the URL has none', async () => {
  const hide = (query, fallback) => {
    const win = open(query);
    let hidden = 0;
    const timer = fallback === undefined ? win.qscAutohide(() => { hidden++; }) : win.qscAutohide(() => { hidden++; }, fallback);
    timer.touch();
    return { win, timer, count: () => hidden };
  };
  const a = hide('demo=1&autohide=2');
  await a.win.advance(1999);
  assert.equal(a.count(), 0);
  await a.win.advance(1);
  assert.equal(a.count(), 1);
  const b = hide('demo=1', 8);
  await b.win.advance(7999);
  assert.equal(b.count(), 0);
  await b.win.advance(1);
  assert.equal(b.count(), 1);
  const c = hide('demo=1&autohide=3', 8);
  await c.win.advance(3000);
  assert.equal(c.count(), 1);
  const d = hide('demo=1');
  await d.win.advance(100000);
  assert.equal(d.count(), 0);
  const e = hide('demo=1&autohide=2');
  e.timer.stop();
  await e.win.advance(5000);
  assert.equal(e.count(), 0);
});

const sources = (win) => win.eventSources;

test('qscConnect opens /events, or /events?keys=1 when lane keys are wanted', () => {
  const win = open('demo=1');
  win.qscConnect({});
  win.qscConnect({}, {});
  win.qscConnect({}, { keys: true });
  win.qscConnect({}, { keys: 0 });
  assert.deepEqual(sources(win).map((s) => s.url), ['/events', '/events', '/events?keys=1', '/events']);
});

test('qscConnect hands object payloads to the handler and drops malformed ones', () => {
  const win = open('demo=1');
  const seen = [];
  const conn = win.qscConnect({ 'game.state': (d) => seen.push(d) });
  const es = sources(win)[0];
  es.emit('game.state', { state: 'menu' });
  es.emit('game.state', 'not json');
  es.emit('game.state', JSON.stringify({ data: 5 }));
  es.emit('game.state', JSON.stringify({ data: null }));
  es.emit('game.state', JSON.stringify(null));
  es.emit('other', { x: 1 });
  assert.deepEqual(JSON.parse(JSON.stringify(seen)), [{ state: 'menu' }]);
  let opened = 0;
  conn.onopen = () => { opened++; };
  es.open();
  assert.equal(opened, 1);
});

test('a stream the server ended is reopened after 5, 10, 20, 30 and 30 seconds, and an open stream resets the delay', async () => {
  const win = open('demo=1');
  win.qscConnect({ 'game.state': () => {} });
  const fail = async (wait) => {
    const es = sources(win)[sources(win).length - 1];
    const before = sources(win).length;
    es.fail(true);
    await win.advance(wait - 1);
    assert.equal(sources(win).length, before, 'too early at ' + wait);
    await win.advance(1);
    assert.equal(sources(win).length, before + 1, 'not reopened at ' + wait);
    assert.equal(es.closed, true);
  };
  await fail(5000);
  await fail(10000);
  await fail(20000);
  await fail(30000);
  await fail(30000);
  sources(win)[sources(win).length - 1].open();
  await fail(5000);
});

test('a retryable error and repeated errors do not stack reconnects, and close() stops everything', async () => {
  const win = open('demo=1');
  const conn = win.qscConnect({});
  const es = sources(win)[0];
  es.fail(false);
  await win.advance(60000);
  assert.equal(sources(win).length, 1);
  es.fail(true);
  es.fail(true);
  await win.advance(5000);
  assert.equal(sources(win).length, 2);
  const next = sources(win)[1];
  next.fail(true);
  conn.close();
  assert.equal(next.closed, true);
  await win.advance(60000);
  assert.equal(sources(win).length, 2);
  next.fail(true);
  await win.advance(60000);
  assert.equal(sources(win).length, 2);
});

run();
