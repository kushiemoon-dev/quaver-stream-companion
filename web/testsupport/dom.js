// A small browser stand-in for node:vm tests: a DOM with events and selectors (quoted values, a few pseudo classes), fake timers and Date, fetch and EventSource stubs, storage and clipboard, able to run the real page scripts.
'use strict';
const fs = require('node:fs');
const path = require('node:path');
const url = require('node:url');
const vm = require('node:vm');

const VOID = new Set(['meta', 'link', 'img', 'input', 'br', 'hr', 'source']);
const REFLECT = ['id', 'lang', 'href', 'rel', 'src', 'title', 'role', 'type', 'name', 'placeholder', 'alt', 'for'];
const flush = async () => { for (let i = 0; i < 3; i++) await new Promise((r) => setImmediate(r)); };

class Event {
  constructor(type, init = {}) { Object.assign(this, { bubbles: true, cancelable: true }, init, { type, defaultPrevented: false, target: null }); }
  preventDefault() { this.defaultPrevented = true; }
  stopPropagation() { this.stopped = true; }
}

class Target {
  constructor() { this.listeners = {}; this.parent = null; }
  addEventListener(type, fn) { (this.listeners[type] ||= []).push(fn); }
  removeEventListener(type, fn) { this.listeners[type] = (this.listeners[type] || []).filter((f) => f !== fn); }
  dispatchEvent(ev) {
    if (!(ev instanceof Event)) ev = new Event(ev.type, ev);
    ev.target ||= this;
    for (let n = this; n && !ev.stopped; n = ev.bubbles ? n.parent : null) {
      ev.currentTarget = n;
      if (typeof n['on' + ev.type] === 'function') n['on' + ev.type](ev);
      for (const fn of [...(n.listeners[ev.type] || [])]) fn.call(n, ev);
    }
    return !ev.defaultPrevented;
  }
}

class Text extends Target {
  constructor(data) { super(); this.nodeType = 3; this.data = String(data); }
  remove() { Element.prototype.remove.call(this); }
  get textContent() { return this.data; }
  set textContent(v) { this.data = String(v); }
}

class Element extends Target {
  constructor(tag, ns) {
    super();
    this.nodeType = 1; this.tagName = tag.toUpperCase(); this.localName = tag.toLowerCase(); this.namespaceURI = ns || null;
    this.childNodes = []; this.attributes = new Map(); this.dataset = {}; this.disabled = false;
    const props = new Map();
    this.style = { setProperty: (k, v) => props.set(k, String(v)), getPropertyValue: (k) => props.get(k) || '', removeProperty: (k) => props.delete(k) };
    this.classList = {
      add: (...c) => this.#setClasses([...this.#classes(), ...c]),
      remove: (...c) => this.#setClasses(this.#classes().filter((x) => !c.includes(x))),
      contains: (c) => this.#classes().includes(c),
      toggle: (c, force) => {
        const on = force === undefined ? !this.#classes().includes(c) : !!force;
        this.#setClasses(on ? [...this.#classes(), c] : this.#classes().filter((x) => x !== c));
        return on;
      },
    };
  }
  #classes() { return (this.attributes.get('class') || '').split(/\s+/).filter(Boolean); }
  #setClasses(list) { this.attributes.set('class', [...new Set(list)].join(' ')); }
  get className() { return this.attributes.get('class') || ''; }
  set className(v) { this.attributes.set('class', String(v)); }
  // value and checked start from their attributes, like the default of a form control.
  get value() { return this._value ?? this.attributes.get('value') ?? ''; }
  set value(v) { this._value = String(v); }
  get checked() { return this._checked ?? this.attributes.has('checked'); }
  set checked(v) { this._checked = !!v; }
  get hidden() { return this.attributes.has('hidden'); }
  set hidden(v) { if (v) this.attributes.set('hidden', ''); else this.attributes.delete('hidden'); }
  get children() { return this.childNodes.filter((n) => n.nodeType === 1); }
  get firstChild() { return this.childNodes[0] || null; }
  get lastChild() { return this.childNodes[this.childNodes.length - 1] || null; }
  get textContent() { return this.childNodes.map((n) => n.textContent).join(''); }
  set textContent(v) { this.replaceChildren(...(String(v) === '' ? [] : [String(v)])); }
  getAttribute(k) { return this.attributes.has(k) ? this.attributes.get(k) : null; }
  setAttribute(k, v) { this.attributes.set(k, String(v)); }
  removeAttribute(k) { this.attributes.delete(k); }
  hasAttribute(k) { return this.attributes.has(k); }
  appendChild(n) { n.remove?.(); n.parent = this; this.childNodes.push(n); return n; }
  append(...nodes) { nodes.forEach((n) => this.appendChild(typeof n === 'object' ? n : new Text(n))); }
  replaceChildren(...nodes) { this.childNodes.forEach((c) => { c.parent = null; }); this.childNodes = []; this.append(...nodes); }
  remove() { if (this.parent) this.parent.childNodes = this.parent.childNodes.filter((c) => c !== this); this.parent = null; }
  focus() { const doc = this.ownerDocument; if (doc) doc.activeElement = this; this.dispatchEvent(new Event('focus', { bubbles: false })); }
  select() {}
  blur() { const doc = this.ownerDocument; if (doc && doc.activeElement === this) doc.activeElement = doc.body; }
  click() { this.dispatchEvent(new Event('click')); }
  get ownerDocument() { let n = this; while (n && n.nodeType !== 9) n = n.parent; return n; }
  contains(n) { for (; n; n = n.parent) if (n === this) return true; return false; }
  querySelectorAll(sel) { return select(this, sel); }
  querySelector(sel) { return select(this, sel)[0] || null; }
  closest(sel) { for (let n = this; n && n.nodeType === 1; n = n.parent) if (matchesAny(n, sel)) return n; return null; }
  matches(sel) { return matchesAny(this, sel); }
}
for (const k of REFLECT) Object.defineProperty(Element.prototype, k, { get() { return this.attributes.get(k) ?? ''; }, set(v) { this.attributes.set(k, String(v)); } });

function walk(root, fn) { for (const c of root.childNodes) if (c.nodeType === 1) { fn(c); walk(c, fn); } }

const SIMPLE = /^(?:([a-zA-Z][\w-]*|\*)|#([\w-]+)|\.([\w-]+)|\[([\w-]+)(?:([~^$*]?=)(?:"([^"]*)"|'([^']*)'|([^\]]*)))?\]|:([\w-]+)(?:\(([^)]*)\))?)/;
const nth = (el, last) => { const sib = el.parent ? el.parent.children : [el]; return sib[last ? sib.length - 1 : 0] === el; };
const PSEUDO = {
  'first-child': (el) => nth(el, false), 'last-child': (el) => nth(el, true), disabled: (el) => el.disabled || el.hasAttribute('disabled'),
  checked: (el) => !!el.checked, empty: (el) => el.childNodes.length === 0, not: (el, arg) => !matchesAny(el, arg),
};
function parseCompound(s) {
  const parts = [];
  while (s) {
    const m = SIMPLE.exec(s);
    if (!m) throw new Error('unsupported selector near "' + s + '"');
    parts.push(m); s = s.slice(m[0].length);
  }
  return parts;
}
function matchCompound(el, parts) {
  return parts.every((m) => {
    if (m[1]) return m[1] === '*' || el.localName === m[1].toLowerCase();
    if (m[2]) return el.id === m[2];
    if (m[3]) return el.classList.contains(m[3]);
    if (m[9]) { if (!PSEUDO[m[9]]) throw new Error('unsupported pseudo class :' + m[9]); return PSEUDO[m[9]](el, m[10]); }
    const v = el.getAttribute(m[4]), want = m[6] ?? m[7] ?? m[8];
    if (v === null) return false;
    return { undefined: () => true, '=': () => v === want, '^=': () => v.startsWith(want), '$=': () => v.endsWith(want), '*=': () => v.includes(want), '~=': () => v.split(/\s+/).includes(want) }[m[5]]();
  });
}
// tokenize splits a selector list into chains of compound selectors and '>' markers, keeping quotes, brackets and parentheses whole.
function tokenize(sel) {
  const lists = [[]]; let cur = '', quote = '', depth = 0;
  const end = () => { if (cur) lists[lists.length - 1].push(cur); cur = ''; };
  for (const ch of sel) {
    if (quote) { cur += ch; if (ch === quote) quote = ''; continue; }
    if (ch === '"' || ch === "'") { quote = ch; cur += ch; continue; }
    if (ch === '[' || ch === '(') depth++;
    if (ch === ']' || ch === ')') depth--;
    if (depth > 0 || !/[\s>,]/.test(ch)) { cur += ch; continue; }
    end();
    if (ch === '>') lists[lists.length - 1].push('>');
    if (ch === ',') lists.push([]);
  }
  end();
  return lists.filter((l) => l.length);
}
function parseSelector(sel) { return tokenize(sel).map((l) => l.map((tok) => (tok === '>' ? '>' : parseCompound(tok)))); }
function matchChain(el, chain) {
  if (!matchCompound(el, chain[chain.length - 1])) return false;
  if (chain.length === 1) return true;
  const direct = chain[chain.length - 2] === '>';
  const rest = chain.slice(0, direct ? -2 : -1);
  for (let p = el.parent; p && p.nodeType === 1; p = p.parent) {
    if (matchChain(p, rest)) return true;
    if (direct) break;
  }
  return false;
}
function matchesAny(el, sel) { return parseSelector(sel).some((c) => matchChain(el, c)); }
function select(root, sel) {
  const chains = parseSelector(sel), out = [];
  walk(root, (el) => { if (chains.some((c) => matchChain(el, c))) out.push(el); });
  return out;
}

class Document extends Target {
  constructor() {
    super();
    this.nodeType = 9; this.childNodes = []; this.title = ''; this.readyState = 'complete'; this.hidden = false; this.scripts = [];
    this.documentElement = this.createElement('html');
    this.head = this.documentElement.appendChild(this.createElement('head'));
    this.body = this.documentElement.appendChild(this.createElement('body'));
    this.childNodes.push(this.documentElement); this.documentElement.parent = this;
    this.activeElement = this.body;
  }
  createElement(tag) { return new Element(tag); }
  createElementNS(ns, tag) { return new Element(tag, ns); }
  createTextNode(t) { return new Text(t); }
  getElementById(id) { return select(this.documentElement, '#' + id)[0] || (this.documentElement.id === id ? this.documentElement : null); }
  querySelector(sel) { return this.querySelectorAll(sel)[0] || null; }
  querySelectorAll(sel) { return select(this, sel); }
  get children() { return [this.documentElement]; }
}

const ENTITIES = { '&amp;': '&', '&lt;': '<', '&gt;': '>', '&quot;': '"', '&#39;': "'", '&nbsp;': String.fromCharCode(160) };
const decode = (s) => s.replace(/&(?:amp|lt|gt|quot|#39|nbsp);/g, (m) => ENTITIES[m]);

// parseInto fills doc from markup; scripts are recorded in doc.scripts, not run.
function parseInto(doc, html) {
  const re = /<!--[\s\S]*?-->|<!doctype[^>]*>|<\/([a-z0-9-]+)\s*>|<([a-z0-9-]+)((?:\s+[^\s=>/]+(?:\s*=\s*(?:"[^"]*"|'[^']*'|[^\s>]+))?)*)\s*(\/?)>|[^<]+|</gi;
  let cur = doc.documentElement; const stack = [];
  for (let m; (m = re.exec(html));) {
    if (m[0].startsWith('<!')) continue;
    if (m[1]) { const i = stack.map((e) => e.localName).lastIndexOf(m[1].toLowerCase()); if (i >= 0) { stack.length = i; cur = stack[i - 1] || doc.documentElement; } continue; }
    if (!m[2]) { if (m[0] !== '<') cur.appendChild(new Text(decode(m[0]))); continue; }
    const tag = m[2].toLowerCase(), attrs = new Map();
    for (const a of m[3].matchAll(/([^\s=/]+)(?:\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+)))?/g)) attrs.set(a[1].toLowerCase(), decode(a[2] ?? a[3] ?? a[4] ?? ''));
    if (tag === 'html') continue;
    if (tag === 'head' || tag === 'body') { cur = tag === 'head' ? doc.head : doc.body; stack.length = 0; stack.push(cur); continue; }
    if (tag === 'script' || tag === 'style') {
      const end = html.indexOf('</' + tag, re.lastIndex), body = html.slice(re.lastIndex, end);
      re.lastIndex = html.indexOf('>', end) + 1;
      if (tag === 'script') doc.scripts.push({ src: attrs.get('src') || null, text: body });
      continue;
    }
    const el = doc.createElement(tag);
    attrs.forEach((v, k) => el.attributes.set(k, v));
    cur.appendChild(el);
    if (!VOID.has(tag) && !m[4]) { stack.push(el); cur = el; }
  }
}

function createClock() {
  let now = 0, seq = 0; const timers = new Map();
  const add = (fn, ms, every, args) => { const id = ++seq; timers.set(id, { fn, at: now + Math.max(0, Number(ms) || 0), every, args }); return id; };
  return {
    now: () => now, pending: () => timers.size,
    setTimeout: (fn, ms, ...args) => add(fn, ms, 0, args), setInterval: (fn, ms, ...args) => add(fn, ms, Math.max(1, Number(ms) || 1), args),
    clearTimeout: (id) => { timers.delete(id); }, clearInterval: (id) => { timers.delete(id); },
    advance(ms) {
      const end = now + ms;
      for (;;) {
        let next = null;
        for (const [id, t] of timers) if (t.at <= end && (!next || t.at < next[1].at)) next = [id, t];
        if (!next) break;
        now = Math.max(now, next[1].at);
        if (next[1].every) next[1].at += next[1].every; else timers.delete(next[0]);
        next[1].fn(...next[1].args);
      }
      now = end;
    },
  };
}

const abortError = () => Object.assign(new Error('The operation was aborted.'), { name: 'AbortError' });
const response = (status, body, raw) => ({
  ok: status >= 200 && status < 300, status, headers: { get: () => null },
  json: async () => { if (raw !== undefined) throw new SyntaxError('Unexpected token'); return structuredClone(body); },
  text: async () => (raw !== undefined ? raw : JSON.stringify(body)),
});
const json = (body, status = 200) => response(status, body);
const text = (raw, status = 200) => response(status, undefined, raw);
function deferred() { const d = {}; d.promise = new Promise((res, rej) => { d.resolve = res; d.reject = rej; }); return d; }

function createFetch(routes, calls) {
  return async function fetch(url, init = {}) {
    const u = new URL(String(url), 'http://localhost');
    const call = { url: String(url), path: u.pathname, query: u.search, method: (init.method || 'GET').toUpperCase(), init, body: init.body === undefined ? undefined : init.body };
    calls.push(call);
    const signal = init.signal;
    if (signal && signal.aborted) throw abortError();
    const key = [call.method + ' ' + call.url, call.url, call.method + ' ' + call.path, call.path].find((k) => k in routes);
    if (key === undefined) throw new TypeError('Failed to fetch');
    let r = routes[key];
    if (typeof r === 'function') r = r(call);
    const gone = new Promise((_, rej) => { if (signal) signal.addEventListener('abort', () => rej(abortError())); });
    r = await Promise.race([Promise.resolve(r), gone]);
    if (r instanceof Error) throw r;
    return r;
  };
}

function createEventSourceClass(instances) {
  return class EventSource {
    static CONNECTING = 0; static OPEN = 1; static CLOSED = 2;
    constructor(url) { this.url = String(url); this.readyState = 0; this.listeners = {}; this.onopen = null; this.onerror = null; this.closed = false; instances.push(this); }
    addEventListener(n, f) { (this.listeners[n] ||= []).push(f); }
    removeEventListener(n, f) { this.listeners[n] = (this.listeners[n] || []).filter((x) => x !== f); }
    close() { this.readyState = 2; this.closed = true; }
    open() { this.readyState = 1; if (this.onopen) this.onopen({ type: 'open' }); }
    // emit delivers payload as {type, data: payload}; a string payload is sent as is, to exercise malformed frames.
    emit(name, payload) { const data = typeof payload === 'string' ? payload : JSON.stringify({ type: name, data: payload }); (this.listeners[name] || []).forEach((f) => f({ type: name, data })); }
    // fail ends the stream for good when fatal, like a 503 does; otherwise the browser would retry on its own.
    fail(fatal) { this.readyState = fatal ? 2 : 0; if (this.onerror) this.onerror({ type: 'error' }); }
  };
}

function createStorage(throws) {
  const m = new Map(); const guard = (v) => { if (throws) throw new Error('storage is blocked'); return v; };
  return {
    getItem: (k) => guard(m.has(k) ? m.get(k) : null), setItem: (k, v) => guard(m.set(k, String(v))), removeItem: (k) => guard(m.delete(k)),
    clear: () => guard(m.clear()), key: (i) => guard([...m.keys()][i] ?? null), get length() { return m.size; },
  };
}

function createLocation(win, href) {
  let u = new URL(href);
  const nav = [];
  const loc = {
    get href() { return u.href; }, get origin() { return u.origin; }, get protocol() { return u.protocol; }, get host() { return u.host; },
    get hostname() { return u.hostname; }, get port() { return u.port; }, get pathname() { return u.pathname; }, get search() { return u.search; }, get hash() { return u.hash; },
    set hash(v) { const old = u.href; u.hash = v; if (u.href !== old) win.dispatchEvent(new Event('hashchange', { bubbles: false, oldURL: old, newURL: u.href })); },
    assign: (v) => nav.push(String(v)), replace: (v) => nav.push(String(v)), reload: () => nav.push('reload'),
    navigations: nav, setUrl: (v) => { u = new URL(v, u); },
  };
  return loc;
}

// createWindow builds the globals of a page. Options: url, language, routes (fetch table), clipboardFails, storageThrows, globals (extra window properties).
function createWindow(opts = {}) {
  const win = new Target(); const clock = createClock();
  const calls = [], sources = [], logs = [], written = [];
  const doc = new Document();
  doc.parent = win;
  const clipboard = { fail: !!opts.clipboardFails, written, writeText: async (t) => { if (clipboard.fail) throw new Error('clipboard denied'); written.push(t); } };
  const base = Date.UTC(2026, 0, 1);
  const FakeDate = class extends Date {
    constructor(...a) { super(...(a.length ? a : [base + clock.now()])); }
    static now() { return base + clock.now(); }
  };
  const log = (level) => (...args) => logs.push({ level, args });
  Object.assign(win, {
    window: win, self: win, document: doc, location: createLocation(win, opts.url || 'http://127.0.0.1:7878/'), clock, routes: { ...(opts.routes || {}) },
    navigator: { language: opts.language || 'en-US', languages: [opts.language || 'en-US'], userAgent: 'test', clipboard },
    localStorage: createStorage(opts.storageThrows), sessionStorage: createStorage(opts.storageThrows),
    history: { state: null, pushState(s, t, u) { this.state = s; if (u) win.location.setUrl(u); }, replaceState(s, t, u) { this.state = s; if (u) win.location.setUrl(u); } },
    console: { error: log('error'), warn: log('warn'), log: log('log'), info: log('info'), debug: log('debug') },
    setTimeout: clock.setTimeout, clearTimeout: clock.clearTimeout, setInterval: clock.setInterval, clearInterval: clock.clearInterval,
    requestAnimationFrame: (fn) => clock.setTimeout(fn, 16), cancelAnimationFrame: clock.clearTimeout,
    matchMedia: () => ({ matches: false, addEventListener() {}, removeEventListener() {} }),
    Date: FakeDate, Event, CustomEvent: Event, Element, Text, URL, URLSearchParams, AbortController, AbortSignal, DOMException, structuredClone,
    fetchCalls: calls, eventSources: sources, logs, clipboard, EventSource: createEventSourceClass(sources),
  });
  Object.assign(win, opts.globals);
  win.fetch = createFetch(win.routes, calls);
  win.$ = (sel) => win.document.querySelector(sel);
  win.$$ = (sel) => win.document.querySelectorAll(sel);
  win.flush = flush;
  win.advance = async (ms) => { win.clock.advance(ms); await flush(); };
  vm.createContext(win);
  return win;
}

// runScript runs a file or source text in the page, as a classic script, so top level declarations are shared.
function runScript(win, file, source) {
  const abs = file ? path.resolve(file) : null;
  new vm.Script(source ?? fs.readFileSync(abs, 'utf8'), { filename: abs ? url.pathToFileURL(abs).href : 'inline.js' }).runInContext(win);
}

// openPage loads webDir/htmlPath into a new window and runs its scripts in order (absolute src is relative to webDir).
// Option url sets the address, for example http://127.0.0.1:7878/overlays/result/?demo=1.
function openPage(webDir, htmlPath, opts = {}) {
  const win = createWindow(opts);
  const file = path.join(webDir, htmlPath);
  parseInto(win.document, fs.readFileSync(file, 'utf8'));
  for (const s of win.document.scripts) {
    if (!s.src) runScript(win, null, s.text);
    else runScript(win, s.src.startsWith('/') ? path.join(webDir, s.src) : path.join(path.dirname(file), s.src));
  }
  return win;
}

module.exports = { createWindow, openPage, runScript, parseInto, createClock, Event, json, text, deferred, flush };
