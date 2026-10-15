(function () {
  const LIMITS = [25, 50, 100, 200];
  const DEFAULT_LIMIT = 50;
  const MAX_MODS = 3;
  const MAX_RANK = 999;
  const GRADE_CLASS = { X: 'gold', SS: 'gold', S: 'gold', A: 'green', B: 'blue', C: 'amber', D: 'amber', F: 'red' };
  const DATE_LANGS = ['en', 'fr', 'de'];
  const FILTERS = { pb: 'is_pb', fc: 'full_combo', failed: 'failed' };

  const COMMA_LANGS = ['fr', 'de'];
  const NBSP = String.fromCharCode(160);

  // Decimal comma for fr and de, a point otherwise; falls back to toFixed when Intl is missing.
  function formatDecimal(n, digits, lang) {
    const fixed = n.toFixed(digits);
    if (COMMA_LANGS.indexOf(lang) < 0) return fixed;
    try {
      return new Intl.NumberFormat(lang, { minimumFractionDigits: digits, maximumFractionDigits: digits }).format(n);
    } catch (e) {
      return fixed.replace('.', ',');
    }
  }

  function formatRate(rate, lang) {
    const n = Number(rate);
    const rounded = Number(n.toFixed(2));
    return !isFinite(n) || rounded <= 0 || rounded === 1 ? '' : String(rounded).replace('.', COMMA_LANGS.indexOf(lang) >= 0 ? ',' : '.') + 'x';
  }

  function modChips(mods) {
    const list = Array.isArray(mods) ? mods.map(String) : [];
    return { shown: list.slice(0, MAX_MODS), extra: Math.max(0, list.length - MAX_MODS), hidden: list.slice(MAX_MODS) };
  }

  const BADGE_MODS = { Paused: 'paused', Autoplay: 'autoplay' };

  // A mod shown as a pill is not listed again as a chip.
  function modsWithoutBadges(mods, flags) {
    const list = Array.isArray(mods) ? mods.map(String) : [];
    const on = flags && typeof flags === 'object' ? flags : {};
    return list.filter(function (name) {
      return !(Object.prototype.hasOwnProperty.call(BADGE_MODS, name) && on[BADGE_MODS[name]] === true);
    });
  }

  function gradeText(grade) {
    return typeof grade === 'string' && grade !== '' ? grade : '-';
  }

  function gradeClass(grade) {
    const key = typeof grade === 'string' ? grade.toUpperCase() : '';
    return Object.prototype.hasOwnProperty.call(GRADE_CLASS, key) ? GRADE_CLASS[key] : 'muted';
  }

  function formatStars(value, lang) {
    const n = Number(value);
    return formatDecimal(isFinite(n) ? n : 0, 1, lang);
  }

  function formatAccuracy(value, lang) {
    const n = Number(value);
    return formatDecimal(isFinite(n) ? n : 0, 2, lang) + (COMMA_LANGS.indexOf(lang) >= 0 ? NBSP + '%' : '%');
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

  // "#k" for a whole local rank of 1 or more, empty otherwise.
  function rankLabel(play) {
    const n = play && typeof play.local_rank === 'number' ? play.local_rank : 0;
    return Number.isInteger(n) && n >= 1 ? '#' + Math.min(n, MAX_RANK) : '';
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
    LIMITS: LIMITS, DEFAULT_LIMIT: DEFAULT_LIMIT, formatDecimal: formatDecimal, formatRate: formatRate, modChips: modChips, modsWithoutBadges: modsWithoutBadges, gradeText: gradeText,
    gradeClass: gradeClass, formatStars: formatStars, formatAccuracy: formatAccuracy, formatPlayedAt: formatPlayedAt,
    filterPlays: filterPlays, rankLabel: rankLabel, parseLimit: parseLimit, playsOf: playsOf,
  };
  if (typeof module !== 'undefined' && module.exports) module.exports = api;
  else window.qscHistoryModel = api;
})();
