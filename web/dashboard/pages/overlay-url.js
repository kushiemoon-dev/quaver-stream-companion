(function () {
  const THEME_RE = /^[a-z0-9-]+$/;
  const ACCENT_RE = /^([0-9a-f]{3}|[0-9a-f]{4}|[0-9a-f]{6}|[0-9a-f]{8})$/;
  const cat = typeof module !== 'undefined' && module.exports ? require('./overlay-catalog.js') : window.qscOverlayCatalog;

  function normalizeAccent(raw) {
    return String(raw).trim().replace(/^#/, '').toLowerCase();
  }

  function isValidAccent(hex) {
    return ACCENT_RE.test(hex);
  }

  function isValidTheme(name) {
    return typeof name === 'string' && THEME_RE.test(name);
  }

  function number(raw) {
    const s = typeof raw === 'string' || typeof raw === 'number' ? String(raw).trim() : '';
    return s === '' ? NaN : Number(s);
  }

  function clamp(n, key) {
    return Math.min(cat.LIMITS[key].max, Math.max(cat.LIMITS[key].min, n));
  }

  // Only the options this overlay honors and that differ from its defaults, clamped to what the overlay accepts.
  function overlayOptions(m, overlay) {
    const out = {};
    const has = function (k) { return overlay.params.indexOf(k) >= 0; };
    const autohide = Math.floor(number(m.autohide));
    if (has('autohide') && isFinite(autohide) && autohide > 0 && clamp(autohide, 'autohide') !== overlay.autohide) out.autohide = String(clamp(autohide, 'autohide'));
    const scale = number(m.scale);
    if (has('scale') && scale > 0 && isFinite(scale)) {
      const v = Math.round(clamp(scale, 'scale') * 100) / 100;
      if (v !== cat.DEFAULTS.scale) out.scale = String(v);
    }
    if (has('lang') && cat.LANGS.indexOf(m.lang) > 0) out.lang = m.lang;
    const n = Math.floor(number(m.n));
    if (has('n') && isFinite(n) && clamp(n, 'n') !== cat.DEFAULTS.n) out.n = String(clamp(n, 'n'));
    ['best', 'sound'].forEach(function (k) { if (has(k) && m[k] === true) out[k] = '1'; });
    const hide = cat.STATES.filter(function (s) { return Array.isArray(m.hide) && m.hide.indexOf(s) >= 0; });
    if (has('hide') && hide.length > 0) out.hide = hide.join(',');
    return out;
  }

  // Demo links are relative and always carry the look; copied links only carry what differs from the saved default.
  function buildOverlayUrl(m, demo, origin) {
    const params = new URLSearchParams();
    const overlay = cat.byId(m.overlay);
    if (isValidTheme(m.theme) && (demo || m.theme !== m.defaultTheme)) params.set('theme', m.theme);
    const accent = normalizeAccent(m.accent || '');
    if (isValidAccent(accent) && (demo || accent !== normalizeAccent(m.defaultAccent || ''))) params.set('accent', accent);
    const options = overlayOptions(m, overlay);
    Object.keys(options).forEach(function (k) { params.set(k, options[k]); });
    if (demo && !overlay.custom) params.set('demo', '1');
    const query = params.toString();
    return (demo ? '' : origin) + overlay.path + (query ? '?' + query : '');
  }

  const api = { normalizeAccent: normalizeAccent, isValidAccent: isValidAccent, isValidTheme: isValidTheme, overlayOptions: overlayOptions, buildOverlayUrl: buildOverlayUrl };
  if (typeof module !== 'undefined' && module.exports) module.exports = api;
  else window.qscOverlayUrl = api;
})();
