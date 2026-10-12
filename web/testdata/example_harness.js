const fs = require('fs');
const vm = require('vm');
const path = require('path');
const assert = require('assert');

const [root, web] = process.argv.slice(2);

function el(tag) {
  const e = {
    tag, children: [], className: '', _text: '', style: { setProperty() {} },
    classList: {
      _s: new Set(),
      add(c) { this._s.add(c); },
      remove(c) { this._s.delete(c); },
      contains(c) { return this._s.has(c); },
    },
    addEventListener() {},
    appendChild(c) { this.children.push(c); return c; },
    remove() {},
    setAttribute() {},
    replaceChildren(...c) { this.children = c; },
  };
  Object.defineProperty(e, 'textContent', { get() { return this._text; }, set(v) { this._text = String(v); } });
  return e;
}

function load(rel, search) {
  const html = fs.readFileSync(path.join(root, rel), 'utf8');
  const els = {};
  const sources = [];
  const re = /<script([^>]*)>([\s\S]*?)<\/script>/g;
  let m;
  while ((m = re.exec(html))) {
    const src = /src="([^"]+)"/.exec(m[1]);
    sources.push(src ? fs.readFileSync(path.join(web, src[1].replace(/^\/overlays\//, 'overlays/')), 'utf8') : m[2]);
  }
  const streams = [];
  class EventSource {
    constructor(url) { this.url = url; this.l = {}; this.readyState = 1; streams.push(this); }
    addEventListener(n, f) { this.l[n] = f; }
    close() {}
  }
  EventSource.CLOSED = 2;
  const timers = [];
  const document = {
    documentElement: el('html'),
    head: el('head'),
    title: '',
    getElementById(id) { return els[id] || (els[id] = el('div')); },
    createElement: el,
  };
  const ctx = vm.createContext({
    document, EventSource, console: { warn() {}, log() {} }, URLSearchParams, Number, JSON, Promise, AbortController,
    location: { search, protocol: 'file:' }, navigator: { language: 'en' },
    setTimeout(f, ms) { timers.push([f, ms]); return timers.length; },
    clearTimeout() {},
    fetch: () => Promise.reject(new Error('no network')),
  });
  ctx.window = ctx;
  els.card = el('div');
  els.card.classList.add('hidden');
  sources.forEach((s) => vm.runInContext(s, ctx));
  return { els, streams, timers };
}

const ev = (type, data) => ({ data: JSON.stringify({ v: 1, type, time: 'x', data }) });
const map = { title: 'Some Title', artist: 'A', difficulty: 1 };
const result = { score: { accuracy: 97.5, grade: 'SS' }, map };

let p = load('docs/examples/mini-card/index.html', '');
assert.strictEqual(p.streams.length, 1);
assert.strictEqual(p.streams[0].url, '/events');
assert.strictEqual(p.els.title.textContent, 'Waiting for Quaver');
p.streams[0].l.snapshot(ev('snapshot', { game: 'menu', map, last_score: null }));
assert.strictEqual(p.els.title.textContent, 'Some Title');
p.streams[0].l['score.saved'](ev('score.saved', result));
assert.strictEqual(p.els.acc.textContent, 'Last play: 97.50%');
p.streams[0].l['map.started'](ev('map.started', { ...map, title: 'Next' }));
assert.strictEqual(p.els.title.textContent, 'Next');
p.streams[0].l['score.saved']({ data: 'not json' });
p.streams[0].l['score.saved'](ev('score.saved', null));
assert.strictEqual(p.els.title.textContent, 'Next');
p.streams[0].readyState = 2;
p.streams[0].onerror();
assert.strictEqual(p.els.status.textContent, 'Reconnecting');
assert.strictEqual(p.timers.length, 1);
p.timers[0][0]();
assert.strictEqual(p.streams.length, 2);

p = load('docs/examples/theme-card/index.html', '?demo=1');
assert.strictEqual(p.els.card.children[0].textContent, 'Example Song');
assert.strictEqual(p.els.card.children[1].textContent, '98.76% S');
assert.ok(!p.els.card.classList.contains('hidden'));

p = load('docs/examples/theme-card/index.html', '');
assert.strictEqual(p.streams.length, 1);
assert.ok(p.els.card.classList.contains('hidden'));
p.streams[0].l.snapshot(ev('snapshot', { game: 'menu', map: null, last_score: null }));
assert.ok(p.els.card.classList.contains('hidden'));
p.streams[0].l['score.saved'](ev('score.saved', result));
assert.strictEqual(p.els.card.children[1].textContent, '97.50% SS');
console.log('ok');
