package web

import "testing"

const settingsPageHarness = `
const fs = require('fs');
const vm = require('vm');
const path = require('path');
const assert = require('assert');

function node(tag) {
  const attrs = {};
  return { tag, attrs, props: {}, children: [], hidden: false, disabled: false, checked: false, value: '', textContent: '', className: '',
    setAttribute(k, v) { attrs[k] = String(v); }, getAttribute(k) { return k in attrs ? attrs[k] : null; }, removeAttribute(k) { delete attrs[k]; },
    appendChild(c) { this.children.push(c); return c; }, replaceChildren(...c) { this.children = c.flat(); }, focus() {}, querySelector() { return null; } };
}
function h(tag, props, ...kids) {
  const n = node(tag);
  for (const [k, v] of Object.entries(props || {})) {
    if (k === 'class') n.className = v; else if (k.startsWith('on')) n.props[k] = v; else { n[k] = v; n.attrs[k] = String(v); }
  }
  n.children = kids.flat(Infinity).filter((c) => c !== null && c !== undefined && c !== false);
  return n;
}
function find(n, pred) {
  if (!n || typeof n !== 'object') return null;
  if (pred(n)) return n;
  for (const c of n.children || []) { const r = find(c, pred); if (r) return r; }
  return null;
}

async function mountWith(config) {
  const puts = [];
  const stub = { build: () => node('section') };
  let page;
  const qsc = {
    t: (k) => k, h, pathNodes: () => [], relabel() {}, rerender() {},
    registerPage: (name, p) => { page = p; },
    api: { get: async () => ({ config, effective: { quaver_dir: '', port: 7878 }, overridden: [] }), put: async (u, body) => { puts.push(body); return { config: body }; } },
  };
  const win = { qsc, qscSettingsKeys: stub, qscSettingsInfo: stub, qscSettingsQuit: stub, qscI18n: { load: async () => {} } };
  const ctx = vm.createContext({ window: win, document: { activeElement: null, body: {} }, AbortController, setTimeout, clearTimeout, requestAnimationFrame: (f) => f(), module: undefined });
  const dir = path.dirname(process.argv[2]);
  vm.runInContext(fs.readFileSync(path.join(dir, 'settings-model.js'), 'utf8'), ctx);
  vm.runInContext(fs.readFileSync(process.argv[2], 'utf8'), ctx);
  const root = node('div');
  page.mount(root);
  for (let i = 0; i < 20; i++) await new Promise((r) => setImmediate(r));
  const box = find(root, (n) => n.attrs && n.attrs.id === 'set-watch');
  const form = find(root, (n) => n.tag === 'form');
  return { box, form, puts };
}
`

func TestSettingsPageCheckboxFollowsTheConfig(t *testing.T) {
	runNodeModel(t, "dashboard/pages/settings.js", settingsPageHarness+`
(async () => {
  const base = { quaver_dir: '', port: 7878, language: 'en', future_setting: { a: 1 } };
  for (const [config, want] of [[{ ...base, watch_quaver_process: false }, false], [{ ...base, watch_quaver_process: true }, true], [base, true]]) {
    const { box } = await mountWith(config);
    assert.ok(box, 'checkbox rendered');
    assert.strictEqual(box.attrs.type, 'checkbox');
    assert.strictEqual(box.checked, want, JSON.stringify(config));
  }
  const { box, form, puts } = await mountWith({ ...base, watch_quaver_process: true });
  box.checked = false;
  await form.props.onsubmit({ preventDefault() {} });
  assert.strictEqual(puts.length, 1);
  assert.strictEqual(puts[0].watch_quaver_process, false);
  assert.deepStrictEqual(puts[0].future_setting, { a: 1 });
  console.log('ok');
})().catch((e) => { console.error(e); process.exit(1); });
`)
}
