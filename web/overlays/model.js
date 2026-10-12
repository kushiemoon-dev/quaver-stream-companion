(function () {
  const LANGS = ['en', 'fr', 'de'];
  const GRADE_CLASS = { X: 'gold', SS: 'gold', S: 'gold', A: 'green', B: 'blue', C: 'amber', D: 'amber', F: 'red' };
  const STATE_TONE = { initialization: 'amber', menu: 'blue', select: 'blue', loading: 'amber', gameplay: 'green', results: 'gold', closed: 'muted', unknown: 'muted' };
  const JUDGMENTS = ['marv', 'perf', 'great', 'good', 'okay', 'miss'];
  const BADGES = [['is_pb', 'pb'], ['full_combo', 'fc'], ['failed', 'failed'], ['paused', 'paused'], ['autoplay', 'autoplay']];
  const MAX_SHOWN_MODS = 6;
  const MAX_MOD_LEN = 12;
  const MAX_TEXT = 200;
  const MAX_RATE = 10;
  const MAX_STARS = 999;
  const MAX_COUNT = 50;
  const MIN_COUNT = 5;
  const MAX_JUDGMENT = 999999;
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

  function formatAccuracy(v) {
    const n = accuracyValue(v);
    return (isFinite(n) ? n : 0).toFixed(2) + '%';
  }

  function formatRate(rate) {
    const n = num(rate);
    const rounded = Number(n.toFixed(2));
    return !isFinite(n) || rounded <= 0 || rounded > MAX_RATE || rounded === 1 ? '' : String(rounded) + 'x';
  }

  function formatStars(v) {
    const n = num(v);
    return isFinite(n) && n >= 0 && n <= MAX_STARS ? n.toFixed(2) : '';
  }

  function modList(mods) {
    if (!Array.isArray(mods)) return { shown: [], extra: 0 };
    const shown = mods.slice(0, MAX_SHOWN_MODS).map(function (m) { return String(m).slice(0, MAX_MOD_LEN); });
    return { shown: shown, extra: mods.length - shown.length };
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

  function resultView(result) {
    if (result === null || typeof result !== 'object') return null;
    const score = obj(result.score);
    const map = obj(result.map);
    return {
      grade: gradeText(score.grade),
      gradeClass: gradeClass(score.grade),
      accuracy: formatAccuracy(score.accuracy),
      combo: isFinite(num(score.max_combo)) ? clamp(Math.floor(num(score.max_combo)), 0, MAX_JUDGMENT) : 0,
      judgments: judgments(score.judgments),
      title: text(map.title),
      artist: text(map.artist),
      difficultyName: text(map.difficulty_name),
      stars: formatStars(map.difficulty),
      rate: formatRate(score.rate),
      mods: modList(score.mods),
      badges: BADGES.filter(function (b) { return result[b[0]] === true; }).map(function (b) { return b[1]; }),
    };
  }

  const ALERT_KINDS = [['is_pb', 'pb'], ['full_combo', 'fc']];
  const ALERT_BLOCKERS = ['failed', 'paused', 'autoplay'];

  // A saved score earns an alert when it is a personal best or a full combo, and was not failed, paused or autoplayed.
  function alertView(result) {
    if (result === null || typeof result !== 'object') return null;
    if (ALERT_BLOCKERS.some(function (k) { return result[k] === true; })) return null;
    const kinds = ALERT_KINDS.filter(function (k) { return result[k[0]] === true; }).map(function (k) { return k[1]; });
    if (kinds.length === 0) return null;
    return { kinds: kinds, title: text(obj(result.map).title), accuracy: formatAccuracy(obj(result.score).accuracy) };
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

  // The dashed line at the best accuracy of the window, from a curve() result.
  function bestLine(c) {
    if (!c || c.points.length === 0) return null;
    const top = c.points.reduce(function (a, p) { return p.accuracy > a.accuracy ? p : a; });
    return { y: top.y, x1: c.left, x2: c.right, text: formatAccuracy(top.accuracy) };
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

  const api = {
    gradeClass: gradeClass, gradeText: gradeText, formatAccuracy: formatAccuracy, formatRate: formatRate,
    formatStars: formatStars, modList: modList, judgments: judgments, parseCount: parseCount,
    parseScale: parseScale, parseAutohide: parseAutohide, parseLang: parseLang, parseHide: parseHide,
    isHiddenState: isHiddenState, stateKey: stateKey, stateTone: stateTone, resultView: resultView,
    curve: curve, createAutohide: createAutohide, alertView: alertView, bestLine: bestLine,
  };
  if (typeof module !== 'undefined' && module.exports) module.exports = api;
  else window.qscModel = api;
})();
