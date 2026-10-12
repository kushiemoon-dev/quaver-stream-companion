(function () {
  const FALLBACK_THEME = 'neutral-light';
  const THEME_RE = /^[a-z0-9-]+$/;
  const ACCENT_RE = /^[0-9a-fA-F]{3,8}$/;
  const FETCH_TIMEOUT_MS = 1200;
  const REVEAL_TIMEOUT_MS = 1500;
  const root = document.documentElement;
  const params = new URLSearchParams(location.search);

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
    link.onerror = reveal;
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
    if (!(urlTheme && urlAccent)) defaults = fetchDefaults();
  } catch (e) {
    reveal();
  }
  defaults.then(function (overlay) {
    const savedTheme = typeof overlay.theme === 'string' && THEME_RE.test(overlay.theme) ? overlay.theme : '';
    const savedAccent = typeof overlay.accent === 'string' && ACCENT_RE.test(overlay.accent) ? overlay.accent : '';
    applyLook(urlTheme || savedTheme || FALLBACK_THEME, urlAccent || savedAccent);
  });

  window.qscConnect = function (handlers) {
    const es = new EventSource('/events');
    Object.keys(handlers).forEach(function (name) {
      es.addEventListener(name, function (ev) {
        handlers[name](JSON.parse(ev.data).data);
      });
    });
    return es;
  };
})();
