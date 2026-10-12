(function () {
  const THEME_RE = /^[a-z0-9-]+$/;
  const ACCENT_RE = /^([0-9a-f]{3}|[0-9a-f]{4}|[0-9a-f]{6}|[0-9a-f]{8})$/;
  const OVERLAY_PATH = '/overlays/now-playing/';

  function normalizeAccent(raw) {
    return String(raw).trim().replace(/^#/, '').toLowerCase();
  }

  function isValidAccent(hex) {
    return ACCENT_RE.test(hex);
  }

  function isValidTheme(name) {
    return typeof name === 'string' && THEME_RE.test(name);
  }

  // Demo links are relative and always carry the look; copied links only carry what differs from the saved default.
  function buildOverlayUrl(m, demo, origin) {
    const params = new URLSearchParams();
    if (isValidTheme(m.theme) && (demo || m.theme !== m.defaultTheme)) params.set('theme', m.theme);
    const accent = normalizeAccent(m.accent || '');
    if (isValidAccent(accent) && (demo || accent !== normalizeAccent(m.defaultAccent || ''))) params.set('accent', accent);
    if (demo) params.set('demo', '1');
    const query = params.toString();
    return (demo ? '' : origin) + OVERLAY_PATH + (query ? '?' + query : '');
  }

  const api = { normalizeAccent: normalizeAccent, isValidAccent: isValidAccent, isValidTheme: isValidTheme, buildOverlayUrl: buildOverlayUrl };
  if (typeof module !== 'undefined' && module.exports) module.exports = api;
  else window.qscOverlayUrl = api;
})();
