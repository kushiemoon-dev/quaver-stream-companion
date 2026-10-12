(function () {
  const LIMITS = [25, 50, 100, 200];
  const DEFAULT_LIMIT = 50;
  const MAX_MODS = 3;
  const GRADE_CLASS = { X: 'gold', SS: 'gold', S: 'gold', A: 'green', B: 'blue', C: 'amber', D: 'amber', F: 'red' };
  const DATE_LANGS = ['en', 'fr', 'de'];
  const FILTERS = { pb: 'is_pb', fc: 'full_combo', failed: 'failed' };

  function formatRate(rate) {
    const n = Number(rate);
    const rounded = Number(n.toFixed(2));
    return !isFinite(n) || rounded <= 0 || rounded === 1 ? '' : String(rounded) + 'x';
  }

  function modChips(mods) {
    const list = Array.isArray(mods) ? mods.map(String) : [];
    return { shown: list.slice(0, MAX_MODS), extra: Math.max(0, list.length - MAX_MODS), hidden: list.slice(MAX_MODS) };
  }

  function gradeText(grade) {
    return typeof grade === 'string' && grade !== '' ? grade : '-';
  }

  function gradeClass(grade) {
    const key = typeof grade === 'string' ? grade.toUpperCase() : '';
    return Object.prototype.hasOwnProperty.call(GRADE_CLASS, key) ? GRADE_CLASS[key] : 'muted';
  }

  function formatStars(value) {
    const n = Number(value);
    return (isFinite(n) ? n : 0).toFixed(1);
  }

  function formatAccuracy(value) {
    const n = Number(value);
    return (isFinite(n) ? n : 0).toFixed(2) + '%';
  }

  function formatPlayedAt(raw, lang) {
    if (typeof raw !== 'string' || raw === '') return '';
    const date = new Date(/^\d{4}-\d{2}-\d{2} /.test(raw) ? raw.replace(' ', 'T') : raw);
    if (isNaN(date.getTime())) return raw;
    try {
      return new Intl.DateTimeFormat(DATE_LANGS.indexOf(lang) >= 0 ? lang : 'en', { dateStyle: 'medium', timeStyle: 'short' }).format(date);
    } catch (e) {
      return raw;
    }
  }

  function filterPlays(plays, filter) {
    const list = Array.isArray(plays) ? plays : [];
    const flag = Object.prototype.hasOwnProperty.call(FILTERS, filter) ? FILTERS[filter] : '';
    return flag ? list.filter(function (p) { return p[flag] === true; }) : list.slice();
  }

  function parseLimit(raw) {
    const n = Number(raw);
    return LIMITS.indexOf(n) >= 0 ? n : DEFAULT_LIMIT;
  }

  function playsOf(data) {
    return Array.isArray(data) ? data.filter(function (p) { return p && typeof p === 'object'; }) : [];
  }

  const api = {
    LIMITS: LIMITS, DEFAULT_LIMIT: DEFAULT_LIMIT, formatRate: formatRate, modChips: modChips, gradeText: gradeText,
    gradeClass: gradeClass, formatStars: formatStars, formatAccuracy: formatAccuracy, formatPlayedAt: formatPlayedAt,
    filterPlays: filterPlays, parseLimit: parseLimit, playsOf: playsOf,
  };
  if (typeof module !== 'undefined' && module.exports) module.exports = api;
  else window.qscHistoryModel = api;
})();
