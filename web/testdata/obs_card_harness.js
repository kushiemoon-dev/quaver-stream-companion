const fs = require('fs');
const vm = require('vm');
const assert = require('assert');

const text = (s) => ({ nodeType: 3, text: String(s) });
function el(tag) {
  const e = {
    nodeType: 1, tag, kids: [],
    setAttribute() {}, addEventListener() {},
    appendChild(c) { this.kids.push(c); return c; },
    // like the DOM: arguments that are not nodes are turned into text
    replaceChildren(...c) { this.kids = c.map((x) => (x && x.nodeType ? x : text(x))); },
    get textContent() { return this.kids.map((k) => (k.nodeType === 3 ? k.text : k.textContent)).join(''); },
  };
  return e;
}
function h(tag, attrs, ...children) {
  const e = el(tag);
  const add = (c) => {
    if (c === null || c === undefined || c === false) return;
    if (Array.isArray(c)) c.forEach(add);
    else e.appendChild(c.nodeType ? c : text(c));
  };
  children.forEach(add);
  return e;
}

const dir = 'C:\Users\me\AppData\Roaming\quaver-stream-companion\overlays';
let page;
const win = {
  qscOverlayCatalog: { OVERLAYS: [], sizeText: () => '' },
  qsc: {
    t: (k) => k, h,
    pathNodes: (d) => [text(d)],
    api: { get: async () => ({ dir }) },
    registerPage: (id, p) => { page = p; },
  },
};
const ctx = vm.createContext({ window: win, document: { createElement: el }, AbortController, setTimeout, clearTimeout, navigator: {}, location: { origin: 'http://127.0.0.1:1' } });
vm.runInContext(fs.readFileSync(process.argv[2], 'utf8'), ctx);
const root = el('div');
page.mount(root);
setTimeout(() => {
  const card = root.kids[root.kids.length - 1];
  const out = card.textContent;
  assert.ok(out.includes('obs.custom.folder ' + dir), out);
  assert.ok(!out.includes('[object'), out);
  page.unmount();
  console.log('ok');
}, 50);
