const params = qscParams;
const SVG_NS = ['http:', '', 'www.w3.org', '2000', 'svg'].join('/');
const WIDTH = 480;
const HEIGHT = 150;
const FETCH_TIMEOUT_MS = 4000;
const DEBOUNCE_MS = 600;
const card = document.getElementById('card');
document.title = qscT('history.title');
let loaded = false;
let inflight = null;
let debounce = null;

function svg(tag, attrs) {
  const el = document.createElementNS(SVG_NS, tag);
  Object.keys(attrs || {}).forEach(function (k) { el.setAttribute(k, String(attrs[k])); });
  return el;
}

function svgText(attrs, text) {
  const el = svg('text', attrs);
  el.textContent = text;
  return el;
}

function draw(plays) {
  const c = qscModel.curve(plays, WIDTH, HEIGHT);
  if (!c) {
    card.classList.add('hidden');
    return;
  }
  const root = svg('svg', { class: 'curve', viewBox: '0 0 ' + WIDTH + ' ' + HEIGHT, role: 'img' });
  root.setAttribute('aria-label', qscT('history.label'));
  root.appendChild(svg('title')).textContent = qscT('history.label');
  const yTop = c.yTop;
  const yBottom = c.yBottom;
  root.appendChild(svg('line', { class: 'axis', x1: c.left, x2: c.right, y1: yTop, y2: yTop }));
  root.appendChild(svg('line', { class: 'axis', x1: c.left, x2: c.right, y1: yBottom, y2: yBottom }));
  root.appendChild(svgText({ class: 'tick', x: c.left - 4, y: yTop + 4, 'text-anchor': 'end' }, String(c.yMax)));
  root.appendChild(svgText({ class: 'tick', x: c.left - 4, y: yBottom + 4, 'text-anchor': 'end' }, String(c.yMin)));
  root.appendChild(svg('line', { class: 'avg', x1: c.left, x2: c.right, y1: c.avgY, y2: c.avgY }));
  const best = params.best ? qscModel.bestLine(c, params.lang) : null;
  if (best) {
    root.appendChild(svg('line', { class: 'best', x1: best.x1, x2: best.x2, y1: best.y, y2: best.y }));
    root.appendChild(svgText({ class: 'best-label', x: best.label.x, y: best.label.y.toFixed(1), 'text-anchor': best.label.anchor }, qscT('history.best') + ' ' + best.text));
  }
  root.appendChild(svg('polyline', { class: 'line-path', points: c.points.map(function (p) { return p.x.toFixed(1) + ',' + p.y.toFixed(1); }).join(' ') }));
  c.points.forEach(function (p) {
    root.appendChild(svg('circle', { class: 'dot-' + qscModel.gradeClass(p.grade), cx: p.x.toFixed(1), cy: p.y.toFixed(1), r: p.last ? 6 : 3.5 }));
    if (p.last) {
      const l = qscModel.lastLabel(c, params.lang);
      root.appendChild(svgText({ x: l.x.toFixed(1), y: l.y.toFixed(1), 'text-anchor': l.anchor, 'font-weight': 700 }, l.text));
    }
  });
  card.replaceChildren(root);
  card.classList.remove('hidden');
}

function load() {
  if (inflight) inflight.abort();
  const ctl = new AbortController();
  inflight = ctl;
  const timer = setTimeout(function () { ctl.abort(); }, FETCH_TIMEOUT_MS);
  fetch('/api/history?limit=' + params.n, { signal: ctl.signal, credentials: 'same-origin' })
    .then(function (res) {
      if (!res.ok) throw new Error('status ' + res.status);
      return res.json();
    })
    .then(function (plays) { if (inflight === ctl) { loaded = true; draw(plays); } })
    .catch(function () {})
    .then(function () {
      clearTimeout(timer);
      if (inflight === ctl) inflight = null;
    });
}

function schedule() {
  clearTimeout(debounce);
  debounce = setTimeout(load, DEBOUNCE_MS);
}

if (params.demo) {
  const demo = [];
  const grades = ['S', 'A', 'S', 'SS', 'B', 'A', 'S', 'A', 'S', 'S'];
  for (let i = 0; i < 10; i++) demo.push({ accuracy: 93 + ((i * 37) % 70) / 10, grade: grades[i] });
  draw(demo);
} else {
  load();
  const es = qscConnect({ 'score.saved': schedule });
  es.onopen = function () { if (!loaded) load(); };
}
