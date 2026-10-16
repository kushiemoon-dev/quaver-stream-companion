const params = qscParams;
const STATUS_TIMEOUT_MS = 4000;
const card = document.getElementById('card');
document.title = qscT('keys.title');
let lanesEl = null;
let built = 0;
let builtScratch = false;
let countEls = [];
let kpsEl = null;
let peakEl = null;

function stat(label) {
  const num = qscEl('b', '', '0');
  const el = qscEl('span');
  el.append(qscEl('span', '', label), num);
  return { el: el, num: num };
}

function build(lanes, scratch) {
  const bars = qscEl('div', 'keys-lanes');
  bars.dataset.dense = lanes >= 9 ? '1' : '0';
  lanesEl = [];
  countEls = [];
  for (let i = 0; i < lanes; i++) {
    const lane = qscEl('div', 'keys-lane');
    lane.setAttribute('role', 'img');
    const isScratch = scratch && i === 0;
    if (isScratch) lane.classList.add('scratch');
    lane.setAttribute('aria-label', isScratch ? qscT('keys.scratch') : qscT('keys.lane') + ' ' + (scratch ? i : i + 1));
    const count = qscEl('div', 'keys-count', '0');
    if (!params.counts) count.hidden = true;
    lane.appendChild(qscEl('div', 'keys-bar'));
    lane.appendChild(count);
    bars.appendChild(lane);
    lanesEl.push(lane);
    countEls.push(count);
  }
  const stats = qscEl('div', 'keys-stats');
  const kps = stat(qscT('keys.kps'));
  kpsEl = kps.num;
  stats.appendChild(kps.el);
  peakEl = null;
  if (params.peak) {
    const peak = stat(qscT('keys.peak'));
    peakEl = peak.num;
    stats.appendChild(peak.el);
  }
  const root = qscEl('div', 'keys');
  root.append(bars, stats);
  card.replaceChildren(root);
  built = lanes;
  builtScratch = scratch;
}

function draw(frame) {
  if (qscModel.keysUnknown(frame)) {
    showHint('unknown_mode');
    return;
  }
  const v = qscModel.keysView(frame);
  if (!v) return;
  if (built !== v.lanes || builtScratch !== v.scratch) build(v.lanes, v.scratch);
  v.down.forEach(function (d, i) { lanesEl[i].classList.toggle('down', d); });
  v.presses.forEach(function (n, i) { countEls[i].textContent = String(n); });
  kpsEl.textContent = String(v.kps);
  if (peakEl) peakEl.textContent = String(v.peak);
  card.classList.remove('hidden');
}

function showHint(hint) {
  built = 0;
  card.replaceChildren(qscEl('div', 'sub', qscT('keys.' + hint)));
  card.classList.remove('hidden');
}

function checkStatus() {
  const ctl = new AbortController();
  const timer = setTimeout(function () { ctl.abort(); }, STATUS_TIMEOUT_MS);
  fetch('/api/status', { signal: ctl.signal, credentials: 'same-origin' })
    .then(function (res) {
      if (!res.ok) throw new Error('status ' + res.status);
      return res.json();
    })
    .then(function (body) {
      const hint = qscModel.keysHint(body && body.keys);
      if (hint) showHint(hint);
      else if (built === 0) card.classList.add('hidden');
    })
    .catch(function () {})
    .then(function () { clearTimeout(timer); });
}

if (params.demo) {
  draw(qscModel.demoKeys(function (k) { return new URLSearchParams(location.search).get(k); }));
} else {
  const es = qscConnect({
    snapshot: function (snap) { if (snap && snap.keys) draw(snap.keys); },
    'keys.frame': draw,
  }, { keys: true });
  es.onopen = checkStatus;
}
