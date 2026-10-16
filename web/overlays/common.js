(function () {
  const FALLBACK_THEME = 'neutral-light';
  const THEME_RE = /^[a-z0-9-]+$/;
  const ACCENT_RE = /^[0-9a-fA-F]{3,8}$/;
  const FETCH_TIMEOUT_MS = 1200;
  const REVEAL_TIMEOUT_MS = 1500;
  const root = document.documentElement;
  const params = new URLSearchParams(location.search);

  const isDemo = params.get('demo') === '1';
  const urlTheme = THEME_RE.test(params.get('theme') || '') ? params.get('theme') : '';
  const urlAccent = ACCENT_RE.test(params.get('accent') || '') ? params.get('accent') : '';

  root.style.visibility = 'hidden';
  let revealed = false;
  function reveal() {
    if (revealed) return;
    revealed = true;
    root.style.visibility = '';
  }
  setTimeout(reveal, REVEAL_TIMEOUT_MS);

  function applyLook(theme, accent) {
    const link = document.createElement('link');
    link.rel = 'stylesheet';
    link.href = '/themes/' + theme + '.css';
    link.onload = reveal;
    link.onerror = function () {
      link.remove();
      if (theme !== FALLBACK_THEME) applyLook(FALLBACK_THEME, accent);
      else reveal();
    };
    document.head.appendChild(link);
    if (accent) root.style.setProperty('--accent', '#' + accent);
  }

  function fetchDefaults() {
    if (location.protocol !== 'http:' && location.protocol !== 'https:') return Promise.resolve({});
    const ctl = new AbortController();
    const timer = setTimeout(function () { ctl.abort(); }, FETCH_TIMEOUT_MS);
    return fetch('/api/config', { signal: ctl.signal, credentials: 'same-origin' })
      .then(function (res) {
        if (!res.ok) throw new Error('status ' + res.status);
        return res.json();
      })
      .then(function (body) { return (body && body.config && body.config.overlay) || {}; })
      .catch(function () { return {}; })
      .then(function (overlay) {
        clearTimeout(timer);
        return overlay;
      });
  }

  let defaults = Promise.resolve({});
  try {
    if (!isDemo && !(urlTheme && urlAccent)) defaults = fetchDefaults();
  } catch (e) {
    reveal();
  }
  defaults.then(function (overlay) {
    const savedTheme = typeof overlay.theme === 'string' && THEME_RE.test(overlay.theme) ? overlay.theme : '';
    const savedAccent = typeof overlay.accent === 'string' && ACCENT_RE.test(overlay.accent) ? overlay.accent : '';
    applyLook(urlTheme || savedTheme || FALLBACK_THEME, urlAccent || savedAccent);
  });

  const model = window.qscModel;
  const lang = model.parseLang(params.get('lang'), navigator.language);
  root.lang = lang;
  const qscParams = {
    demo: isDemo,
    lang: lang,
    scale: model.parseScale(params.get('scale')),
    autohide: model.parseAutohide(params.get('autohide')),
    hide: model.parseHide(params.get('hide')),
    n: model.parseCount(params.get('n')),
    best: params.get('best') === '1',
    sound: params.get('sound') === '1',
    counts: params.get('counts') !== '0',
    peak: params.get('peak') !== '0',
    meta: params.get('meta') === '1',
  };
  root.style.setProperty('--scale', String(qscParams.scale));
  window.qscParams = qscParams;

  window.qscT = function (key) {
    const catalogs = window.qscCatalogs || {};
    const own = function (c) { return c && Object.prototype.hasOwnProperty.call(c, key) ? c[key] : ''; };
    return own(catalogs[lang]) || own(catalogs.en) || key;
  };

  // fallback is the delay used when the page has no autohide parameter.
  window.qscAutohide = function (onHide, fallback) {
    const seconds = fallback === undefined ? qscParams.autohide : model.parseAutohide(params.get('autohide'), fallback);
    return model.createAutohide(seconds, {
      setTimeout: function (fn, ms) { return setTimeout(fn, ms); },
      clearTimeout: function (id) { clearTimeout(id); },
    }, onHide);
  };

  window.qscEl = function (tag, cls, text) {
    const el = document.createElement(tag);
    if (cls) el.className = cls;
    if (text !== undefined) el.textContent = text;
    return el;
  };

  // options.keys asks the server for lane key frames, which are not sent otherwise.
  window.qscConnect = function (handlers, options) {
    return model.createConnection(function (url) { return new EventSource(url); }, options && options.keys ? '/events?keys=1' : '/events', handlers, {
      setTimeout: function (fn, ms) { return setTimeout(fn, ms); },
      clearTimeout: function (id) { clearTimeout(id); },
    });
  };
})();
