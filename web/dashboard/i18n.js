(function () {
  const SUPPORTED = ['en', 'fr', 'de'];
  let messages = {};
  let current = 'en';

  function pick(configured, browser) {
    if (SUPPORTED.indexOf(configured) >= 0) return configured;
    const b = String(browser || '').toLowerCase();
    if (b.indexOf('fr') === 0) return 'fr';
    if (b.indexOf('de') === 0) return 'de';
    return 'en';
  }

  async function fetchCatalog(lang) {
    try {
      const res = await fetch('i18n/' + lang + '.json');
      if (!res.ok) return null;
      return await res.json();
    } catch (e) {
      return null;
    }
  }

  async function load(configured) {
    const lang = pick(configured, navigator.language);
    const base = await fetchCatalog('en');
    const over = lang === 'en' ? base : await fetchCatalog(lang);
    current = lang;
    messages = Object.assign({}, base, over);
    if (over) document.documentElement.lang = lang;
    return lang;
  }

  function t(key, params) {
    const raw = Object.prototype.hasOwnProperty.call(messages, key) ? messages[key] : key;
    if (!params) return raw;
    return raw.replace(/\{(\w+)\}/g, function (m, name) {
      return Object.prototype.hasOwnProperty.call(params, name) ? String(params[name]) : m;
    });
  }

  window.qscI18n = { load: load, t: t, pick: pick, lang: function () { return current; } };
})();
