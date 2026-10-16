(function () {
  const LANGS = ['en', 'fr', 'de'];
  const GRADE_CLASS = { X: 'gold', SS: 'gold', S: 'gold', A: 'green', B: 'blue', C: 'amber', D: 'amber', F: 'red' };
  const STATE_TONE = { initialization: 'amber', menu: 'blue', select: 'blue', loading: 'amber', gameplay: 'green', results: 'gold', editor: 'blue', importing: 'amber', download: 'amber', lobby: 'blue', multiplayer: 'blue', music: 'blue', theatre: 'blue', closed: 'muted', unknown: 'muted' };
  const JUDGMENTS = ['marv', 'perf', 'great', 'good', 'okay', 'miss'];
  const BADGES = [['is_pb', 'pb'], ['is_local_best', 'best'], ['full_combo', 'fc'], ['failed', 'failed'], ['paused', 'paused'], ['autoplay', 'autoplay']];
  const MAX_SHOWN_MODS = 6;
  const MAX_MOD_LEN = 12;
  const MAX_TEXT = 200;
  const MAX_RATE = 10;
  const MAX_STARS = 999;
  const MAX_BPM = 99999;
  const MAX_LENGTH_MS = 86400000;
  const MAX_NOTES = 99999;
  const MAX_COUNT = 50;
  const MAX_RANK = 999;
  const MAX_CHIPS = 8;
  const MIN_COUNT = 5;
  const MAX_JUDGMENT = 999999;
  const MAX_RATING = 9999;
  const MAX_KPS = 999;
  const MAX_KEYS = 10;
  const MAX_LANES = MAX_KEYS + 1;
  const NBSP = String.fromCharCode(160);
  const STAR = String.fromCharCode(9733);
  const SEP = ' ' + String.fromCharCode(183) + ' ';
  const PAD = { l: 26, r: 14, t: 14, b: 10 };

  const has = function (obj, key) { return Object.prototype.hasOwnProperty.call(obj, key); };
  const num = function (v) { return typeof v === 'number' || (typeof v === 'string' && v.trim() !== '') ? Number(v) : NaN; };
  const clamp = function (n, lo, hi) { return Math.min(hi, Math.max(lo, n)); };
  const obj = function (v) { return v !== null && typeof v === 'object' ? v : {}; };
  const text = function (v, max) { return typeof v === 'string' ? v.slice(0, max || MAX_TEXT) : ''; };

  function gradeClass(grade) {
    const key = typeof grade === 'string' ? grade.toUpperCase() : '';
    return has(GRADE_CLASS, key) ? GRADE_CLASS[key] : 'muted';
  }

  function gradeText(grade) {
    return typeof grade === 'string' && grade !== '' ? grade.slice(0, 3) : '-';
  }

  function accuracyValue(v) {
    const n = num(v);
    return isFinite(n) ? clamp(n, 0, 100) : NaN;
  }

  const COMMA_LANGS = ['fr', 'de'];

  // Decimal comma for fr and de, a point otherwise; falls back to toFixed when Intl is missing.
  function decimal(n, digits, lang) {
    const fixed = n.toFixed(digits);
    if (COMMA_LANGS.indexOf(lang) < 0) return fixed;
    try {
      return new Intl.NumberFormat(lang, { minimumFractionDigits: digits, maximumFractionDigits: digits }).format(n);
    } catch (e) {
      return fixed.replace('.', ',');
    }
  }

  function formatAccuracy(v, lang) {
    const n = accuracyValue(v);
    return decimal(isFinite(n) ? n : 0, 2, lang) + (COMMA_LANGS.indexOf(lang) >= 0 ? NBSP + '%' : '%');
  }

  function formatRate(rate, lang) {
    const n = num(rate);
    const rounded = Number(n.toFixed(2));
    return !isFinite(n) || rounded <= 0 || rounded > MAX_RATE || rounded === 1 ? '' : String(rounded).replace('.', COMMA_LANGS.indexOf(lang) >= 0 ? ',' : '.') + 'x';
  }

  function formatStars(v, lang) {
    const n = num(v);
    return isFinite(n) && n >= 0 && n <= MAX_STARS ? decimal(n, 2, lang) : '';
  }

  // Every field is optional: a map without a difficulty still gets a line.
  function nowPlayingView(map, lang) {
    const m = obj(map);
    const stars = formatStars(m.difficulty, lang);
    const k = Math.floor(num(m.keys));
    return {
      title: text(m.title),
      artist: text(m.artist),
      line: [text(m.difficulty_name), stars ? stars + STAR : '', k > 0 ? k + 'K' : ''].filter(Boolean).join(SEP),
    };
  }

  function positive(v, max) {
    const n = num(v);
    return isFinite(n) && n > 0 && n <= max ? n : 0;
  }

  // One line of map details: mapper, BPM, length and note counts. Values that are 0 or missing are left out.
  // BPM and length are stored at 1.0x; a known play rate scales them. labels: by, notes, ln, mines.
  function metaLine(map, rate, lang, labels) {
    const m = obj(map);
    const l = obj(labels);
    const r = positive(rate, MAX_RATE) || 1;
    const bpm = positive(m.bpm, MAX_BPM);
    const secs = Math.round(positive(m.length_ms, MAX_LENGTH_MS) / r / 1000);
    const count = function (v, label) {
      const n = Math.floor(positive(v, MAX_NOTES));
      return n > 0 ? [n, text(l[label], 20)].filter(Boolean).join(NBSP) : '';
    };
    const mapper = text(m.creator, 60).trim();
    return [
      mapper ? [text(l.by, 20), mapper].filter(Boolean).join(' ') : '',
      bpm ? decimal(bpm * r, 0, lang) + NBSP + 'BPM' : '',
      secs > 0 ? Math.floor(secs / 60) + ':' + String(secs % 60).padStart(2, '0') : '',
      count(m.regular_notes, 'notes'), count(m.long_notes, 'ln'), count(m.mines, 'mines'),
    ].filter(Boolean).join(SEP);
  }

  // room is how many chips are left for mods, the "+N" chip included; without it six mods are shown.
  function modList(mods, room) {
    if (!Array.isArray(mods)) return { shown: [], extra: 0 };
    let count = mods.length;
    if (room !== undefined && count > room) count = Math.max(0, room - 1);
    count = Math.min(count, MAX_SHOWN_MODS);
    const shown = mods.slice(0, count).map(function (m) { return String(m).slice(0, MAX_MOD_LEN); });
    return { shown: shown, extra: room !== undefined && room < 1 ? 0 : mods.length - count };
  }

  function judgments(j) {
    const src = obj(j);
    return JUDGMENTS.map(function (key) {
      const n = num(src[key]);
      return { key: key, value: isFinite(n) ? clamp(Math.floor(n), 0, MAX_JUDGMENT) : 0 };
    });
  }

  function parseCount(raw) {
    const n = Math.floor(num(raw));
    return isFinite(n) ? clamp(n, MIN_COUNT, MAX_COUNT) : 20;
  }

  function parseScale(raw) {
    const n = num(raw);
    return isFinite(n) && n > 0 ? clamp(n, 0.5, 3) : 1;
  }

  // fallback applies only when the parameter is absent or empty.
  function parseAutohide(raw, fallback) {
    if ((raw === null || raw === undefined || raw === '') && fallback !== undefined) return fallback;
    const n = Math.floor(num(raw));
    return isFinite(n) && n > 0 ? Math.min(n, 3600) : 0;
  }

  function parseLang(raw, browser) {
    if (LANGS.indexOf(raw) >= 0) return raw;
    const b = String(browser || '').toLowerCase();
    if (b.indexOf('fr') === 0) return 'fr';
    if (b.indexOf('de') === 0) return 'de';
    return 'en';
  }

  function parseHide(raw) {
    if (typeof raw !== 'string') return [];
    const out = [];
    raw.split(',').forEach(function (s) {
      const k = s.trim().toLowerCase();
      if (has(STATE_TONE, k) && out.indexOf(k) < 0) out.push(k);
    });
    return out;
  }

  function knownState(state) {
    return typeof state === 'string' && has(STATE_TONE, state) ? state : 'unknown';
  }

  function isHiddenState(state, hide) {
    return hide.indexOf(knownState(state)) >= 0;
  }

  function stateKey(state) { return 'state.' + knownState(state); }
  function stateTone(state) { return STATE_TONE[knownState(state)]; }

  // "#k" for a whole rank of 1 or more, empty otherwise.
  function rankLabel(v) {
    return typeof v === 'number' && Number.isInteger(v) && v >= 1 ? '#' + Math.min(v, MAX_RANK) : '';
  }

  // The result card has room for MAX_CHIPS chips: badges and the rank come first, then the rate, then the mods.
  function resultView(result, lang) {
    if (result === null || typeof result !== 'object') return null;
    const score = obj(result.score);
    const map = obj(result.map);
    const badges = BADGES.filter(function (b) { return result[b[0]] === true; }).map(function (b) { return b[1]; });
    const rank = rankLabel(result.local_rank);
    const rate = formatRate(score.rate, lang);
    const room = MAX_CHIPS - badges.length - (rank ? 1 : 0) - (rate ? 1 : 0);
    return {
      grade: gradeText(score.grade),
      gradeClass: gradeClass(score.grade),
      accuracy: formatAccuracy(score.accuracy, lang),
      combo: isFinite(num(score.max_combo)) ? clamp(Math.floor(num(score.max_combo)), 0, MAX_JUDGMENT) : 0,
      judgments: judgments(score.judgments),
      title: text(map.title),
      artist: text(map.artist),
      difficultyName: text(map.difficulty_name),
      stars: formatStars(map.difficulty, lang),
      rate: rate,
      mods: modList(withoutBadgeMods(score.mods, result), room),
      rank: rank,
      badges: badges,
    };
  }

  const BADGE_MODS = { Paused: 'paused', Autoplay: 'autoplay' };

  // A mod shown as a badge is not listed again as a chip.
  function withoutBadgeMods(mods, result) {
    if (!Array.isArray(mods)) return mods;
    return mods.filter(function (m) { return !(has(BADGE_MODS, m) && result[BADGE_MODS[m]] === true); });
  }

  // fresh(result) is false only for a score whose id was the last one seen, so a reconnect never replays it.
  function createIdGate() {
    let last = null;
    return {
      fresh: function (result) {
        if (result === null || typeof result !== 'object') return false;
        const id = obj(result.score).id;
        if (!Number.isInteger(id)) return true;
        if (id === last) return false;
        last = id;
        return true;
      },
    };
  }

  const ALERT_KINDS = [['is_pb', 'pb'], ['is_local_best', 'best'], ['full_combo', 'fc']];
  const ALERT_BLOCKERS = ['failed', 'paused', 'autoplay'];

  // A saved score earns an alert when it is a personal best, the local best or a full combo, and was not failed, paused or autoplayed.
  // The local best replaces the personal best, which it already says.
  function alertView(result, lang) {
    if (result === null || typeof result !== 'object') return null;
    if (ALERT_BLOCKERS.some(function (k) { return result[k] === true; })) return null;
    const kinds = ALERT_KINDS.filter(function (k) { return result[k[0]] === true; }).map(function (k) { return k[1]; })
      .filter(function (k, _, all) { return !(k === 'pb' && all.indexOf('best') >= 0); });
    if (kinds.length === 0) return null;
    return { kinds: kinds, title: text(obj(result.map).title), accuracy: formatAccuracy(obj(result.score).accuracy, lang) };
  }

  // Input is newest first (the API order); the curve runs oldest left to newest right.
  function curve(plays, width, height) {
    const list = (Array.isArray(plays) ? plays : []).slice(0, MAX_COUNT).reverse()
      .map(function (p) { return { accuracy: accuracyValue(obj(p).accuracy), grade: obj(p).grade }; })
      .filter(function (p) { return isFinite(p.accuracy); });
    if (list.length === 0) return null;
    const lowest = Math.min.apply(null, list.map(function (p) { return p.accuracy; }));
    const yMin = Math.min(75, Math.floor(lowest / 5) * 5);
    const yMax = 100;
    const innerW = width - PAD.l - PAD.r;
    const innerH = height - PAD.t - PAD.b;
    const yOf = function (acc) { return PAD.t + ((yMax - acc) / (yMax - yMin)) * innerH; };
    const avg = list.reduce(function (s, p) { return s + p.accuracy; }, 0) / list.length;
    return {
      yMin: yMin,
      yTop: PAD.t,
      yBottom: height - PAD.b,
      yMax: yMax,
      avg: avg,
      avgY: yOf(avg),
      left: PAD.l,
      right: width - PAD.r,
      points: list.map(function (p, i) {
        return {
          x: list.length === 1 ? PAD.l + innerW / 2 : PAD.l + (i * innerW) / (list.length - 1),
          y: yOf(p.accuracy),
          accuracy: p.accuracy,
          grade: p.grade,
          last: i === list.length - 1,
        };
      }),
    };
  }

  const LABEL_H = 12;
  const DOT_R = 6;
  const LAST_W = 48;
  const BEST_W = 84;
  const box = function (x, y, anchor, w) {
    const x0 = anchor === 'end' ? x - w : x;
    return { x0: x0, x1: x0 + w, y0: y - LABEL_H + 2, y1: y + 3 };
  };
  const overlaps = function (a, b) { return a.x0 < b.x1 && b.x0 < a.x1 && a.y0 < b.y1 && b.y0 < a.y1; };

  // Where the accuracy label of the newest play goes: beside its dot, on the inner side near the right edge.
  function lastLabel(c, lang) {
    if (!c || c.points.length === 0) return null;
    const p = c.points[c.points.length - 1];
    const nearRight = p.x > c.right + PAD.r - 50;
    return { x: p.x + (nearRight ? -10 : 10), y: Math.max(12, p.y - 8), anchor: nearRight ? 'end' : 'start', text: formatAccuracy(p.accuracy, lang) };
  }

  // The line at the best accuracy of the window, from a curve() result. Its label is right aligned at
  // the plot edge, above or below the line, whichever collides less with the dots (a third spot sits just above the plot) and the newest label.
  function bestLine(c, lang) {
    if (!c || c.points.length === 0) return null;
    const top = c.points.reduce(function (a, p) { return p.accuracy > a.accuracy ? p : a; });
    const taken = c.points.map(function (p) { return { x0: p.x - DOT_R, x1: p.x + DOT_R, y0: p.y - DOT_R, y1: p.y + DOT_R }; });
    const last = lastLabel(c, lang);
    taken.push(box(last.x, last.y, last.anchor, LAST_W));
    const candidates = [top.y - 4, top.y + 13, c.yTop - 3].filter(function (y) { return y >= c.yTop - 3 && y <= c.yBottom - 2; });
    let label = null;
    let fewest = Infinity;
    candidates.forEach(function (y) {
      const hits = taken.filter(function (b) { return overlaps(b, box(c.right, y, 'end', BEST_W)); }).length;
      if (hits < fewest) {
        fewest = hits;
        label = { x: c.right, y: y, anchor: 'end' };
      }
    });
    return { y: top.y, x1: c.left, x2: c.right, text: formatAccuracy(top.accuracy, lang), label: label };
  }

  const MAX_POINTS = 200;
  const MAX_MISSES = 500;
  const MAX_TOTAL = 100000;
  const WARMUP = 0.1;

  // The first tenth of the points is left out of the axis range so the first judgments do not flatten the curve.
  function timelineView(tl, width, height, lang) {
    const src = obj(tl);
    const points = (Array.isArray(src.points) ? src.points : []).slice(0, MAX_POINTS).map(accuracyValue).filter(isFinite);
    if (points.length === 0) return null;
    const t = Math.floor(num(src.total));
    const total = isFinite(t) ? clamp(t, points.length, MAX_TOTAL) : points.length;
    const innerW = width - PAD.l - PAD.r;
    const innerH = height - PAD.t - PAD.b;
    const lowest = Math.min.apply(null, points.slice(Math.floor(points.length * WARMUP)));
    const yMin = Math.min(90, Math.floor(lowest / 5) * 5);
    const yOf = function (acc) { return PAD.t + ((100 - clamp(acc, yMin, 100)) / (100 - yMin)) * innerH; };
    const misses = (Array.isArray(src.misses) ? src.misses : []).slice(0, MAX_MISSES).map(num)
      .filter(function (i) { return Number.isInteger(i) && i >= 0 && i < total; });
    return {
      yMin: yMin,
      yMax: 100,
      yTop: PAD.t,
      yBottom: height - PAD.b,
      left: PAD.l,
      right: width - PAD.r,
      points: points.map(function (acc, i) { return { x: PAD.l + (innerW * (i + 1)) / points.length, y: yOf(acc) }; }),
      misses: misses.map(function (i) { return PAD.l + (innerW * (i + 0.5)) / total; }),
      final: formatAccuracy(points[points.length - 1], lang),
    };
  }

  function keysCounter(v, max) {
    const n = Math.floor(num(v));
    return isFinite(n) ? clamp(n, 0, max) : 0;
  }

  // Lane state from a keys.frame; null when the frame cannot be drawn.
  function keysView(frame) {
    const f = obj(frame);
    if (!Number.isInteger(f.lanes) || f.lanes < 1 || f.lanes > MAX_LANES) return null;
    const lane = function (list, make) {
      const src = Array.isArray(list) ? list : [];
      return Array.from({ length: f.lanes }, function (_, i) { return make(src[i]); });
    };
    return {
      lanes: f.lanes,
      scratch: f.scratch === true && f.lanes > 1,
      down: lane(f.down, function (v) { return v === true; }),
      presses: lane(f.presses, function (v) { return keysCounter(v, MAX_JUDGMENT); }),
      kps: keysCounter(f.kps, MAX_KPS),
      peak: keysCounter(f.peak, MAX_KPS),
    };
  }

  // True for a frame of a map whose key mode is not supported: it has no lane at all.
  function keysUnknown(frame) {
    const f = obj(frame);
    return f.lanes === 0 && f.layout === 'unknown';
  }

  // A frame for demo=1: layout=NK or lanes=N (1 to 10) picks the key lanes, scratch=1 adds a scratch lane first.
  function demoKeys(get) {
    const count = function (v, re) { const n = re.test(v || '') ? parseInt(v, 10) : 0; return n >= 1 && n <= MAX_KEYS ? n : 0; };
    const keys = count(get('layout'), /^[0-9]{1,2}[kK]$/) || count(get('lanes'), /^[0-9]{1,2}$/) || 4;
    const scratch = get('scratch') === '1';
    const lanes = keys + (scratch ? 1 : 0);
    const at = function (make) { return Array.from({ length: lanes }, function (_, i) { return make(i); }); };
    return {
      layout: keys + 'k', lanes: lanes, scratch: scratch,
      down: at(function (i) { return i % 2 === 0; }),
      presses: at(function (i) { return 128 - ((i * 17) % 60); }),
      kps: 9, peak: 14,
    };
  }

  // The reason to explain why no lane data will come: 'unsupported', 'off' or '' when it should work.
  function keysHint(status) {
    const s = obj(status);
    if (s.supported === false) return 'unsupported';
    if (s.enabled === false) return 'off';
    return '';
  }

  // seconds <= 0 disables it. The clock is injectable for tests.
  function createAutohide(seconds, clock, onHide) {
    let timer = null;
    const stop = function () {
      if (timer !== null) clock.clearTimeout(timer);
      timer = null;
    };
    return {
      touch: function () {
        stop();
        if (seconds > 0) timer = clock.setTimeout(function () { timer = null; onHide(); }, seconds * 1000);
      },
      stop: stop,
    };
  }

  const MAX_ELAPSED_S = 99 * 3600 + 3599;
  const NONE = '-';

  function formatElapsed(ms) {
    const n = num(ms);
    const s = isFinite(n) ? clamp(Math.floor(n / 1000), 0, MAX_ELAPSED_S) : 0;
    const pad = function (v) { return String(v).padStart(2, '0'); };
    const h = Math.floor(s / 3600);
    return (h > 0 ? h + ':' + pad(Math.floor((s % 3600) / 60)) : String(Math.floor(s / 60))) + ':' + pad(s % 60);
  }

  // The figures of a session object as display strings; averages and ratings show "-" until there is a pass.
  function sessionView(summary, now, lang) {
    const s = obj(summary);
    const count = function (v) { const n = num(v); return isFinite(n) ? clamp(Math.floor(n), 0, MAX_JUDGMENT) : 0; };
    const passes = count(s.passes);
    const started = Date.parse(text(s.started_at, 40));
    const rating = num(s.best_rating);
    return {
      elapsed: formatElapsed(isFinite(started) ? num(now) - started : 0),
      plays: String(count(s.plays)),
      outcome: passes + ' / ' + count(s.fails),
      average: passes > 0 ? formatAccuracy(s.avg_accuracy, lang) : NONE,
      best: passes > 0 ? formatAccuracy(s.best_accuracy, lang) : NONE,
      streak: String(count(s.streak)),
      bestStreak: String(count(s.best_streak)),
      fullCombos: String(count(s.full_combos)),
      rating: passes > 0 && isFinite(rating) ? decimal(clamp(rating, 0, MAX_RATING), 2, lang) : NONE,
    };
  }

  const RECONNECT_FIRST_MS = 5000;
  const RECONNECT_MAX_MS = 30000;
  const EVENT_SOURCE_CLOSED = 2;

  // createConnection opens an event stream with open(url) and, when the browser gives up on it
  // (a 503 when the server is full ends an EventSource for good), opens a new one after a delay
  // that doubles from 5 s up to 30 s and starts over once a stream is open again.
  // The returned handle forwards onopen and close() to whichever stream is current.
  function createConnection(open, url, handlers, timers) {
    let es = null;
    let timer = null;
    let delay = RECONNECT_FIRST_MS;
    let stopped = false;
    const handle = { onopen: null, close: function () {
      stopped = true;
      if (timer !== null) timers.clearTimeout(timer);
      if (es) es.close();
    } };
    function connect() {
      timer = null;
      const cur = open(url);
      es = cur;
      Object.keys(handlers).forEach(function (name) {
        cur.addEventListener(name, function (ev) {
          let body;
          try { body = JSON.parse(ev.data); } catch (e) { return; }
          const data = body !== null && typeof body === 'object' ? body.data : null;
          if (data !== null && typeof data === 'object') handlers[name](data);
        });
      });
      cur.onopen = function () {
        delay = RECONNECT_FIRST_MS;
        if (typeof handle.onopen === 'function') handle.onopen();
      };
      cur.onerror = function () {
        if (stopped || cur.readyState !== EVENT_SOURCE_CLOSED || timer !== null) return;
        cur.close();
        timer = timers.setTimeout(connect, delay);
        delay = Math.min(delay * 2, RECONNECT_MAX_MS);
      };
    }
    connect();
    return handle;
  }

  const api = {
    createConnection: createConnection, createIdGate: createIdGate,
    gradeClass: gradeClass, gradeText: gradeText, formatAccuracy: formatAccuracy, formatRate: formatRate,
    formatStars: formatStars, nowPlayingView: nowPlayingView, metaLine: metaLine, modList: modList, judgments: judgments, parseCount: parseCount,
    parseScale: parseScale, parseAutohide: parseAutohide, parseLang: parseLang, parseHide: parseHide,
    isHiddenState: isHiddenState, rankLabel: rankLabel, stateKey: stateKey, stateTone: stateTone, resultView: resultView,
    curve: curve, createAutohide: createAutohide, alertView: alertView, bestLine: bestLine,
    lastLabel: lastLabel, sessionView: sessionView, formatElapsed: formatElapsed, keysView: keysView, demoKeys: demoKeys, keysUnknown: keysUnknown, keysHint: keysHint, timelineView: timelineView,
  };
  if (typeof module !== 'undefined' && module.exports) module.exports = api;
  else window.qscModel = api;
})();
