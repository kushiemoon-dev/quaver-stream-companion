(function () {
  const STATES = ['initialization', 'menu', 'select', 'loading', 'gameplay', 'results', 'editor', 'importing', 'download', 'lobby', 'multiplayer', 'music', 'theatre', 'closed', 'unknown'];
  const LANGS = ['auto', 'en', 'fr', 'de'];
  const DEFAULTS = { autohide: 0, scale: 1, lang: 'auto', n: 20 };
  const LIMITS = { autohide: { min: 1, max: 3600 }, scale: { min: 0.5, max: 3 }, n: { min: 5, max: 50 } };
  const CUSTOM_ID = /^[a-z0-9-]+$/;
  const CUSTOM_LIMITS = { count: 100, name: 40, description: 160, width: { min: 100, max: 3840, def: 500 }, height: { min: 40, max: 2160, def: 200 } };

  // Recommended OBS browser source sizes; params lists the options each overlay honors; autohide is the delay used when none is given.
  const OVERLAYS = [
    { id: 'now-playing', path: '/overlays/now-playing/', width: 540, height: 140,
      nameKey: 'overlays.now_playing.name', descKey: 'overlays.now_playing.description', params: ['autohide', 'scale', 'lang', 'meta'] },
    { id: 'result', path: '/overlays/result/', width: 520, height: 230,
      nameKey: 'overlays.result.name', descKey: 'overlays.result.description', params: ['autohide', 'scale', 'lang'] },
    { id: 'history', path: '/overlays/history/', width: 520, height: 180,
      nameKey: 'overlays.history.name', descKey: 'overlays.history.description', params: ['scale', 'lang', 'n', 'best'] },
    { id: 'state', path: '/overlays/state/', width: 280, height: 64,
      nameKey: 'overlays.state.name', descKey: 'overlays.state.description', params: ['scale', 'lang', 'hide'] },
    { id: 'alert', path: '/overlays/alert/', width: 520, height: 120, autohide: 8,
      nameKey: 'overlays.alert.name', descKey: 'overlays.alert.description', params: ['autohide', 'scale', 'lang', 'sound'] },
    { id: 'keys', path: '/overlays/keys/', width: 480, height: 200,
      nameKey: 'overlays.keys.name', descKey: 'overlays.keys.description', params: ['scale', 'lang', 'counts', 'peak'] },
    { id: 'curve', path: '/overlays/curve/', width: 520, height: 180,
      nameKey: 'overlays.curve.name', descKey: 'overlays.curve.description', params: ['autohide', 'scale', 'lang'] },
  ];

  let custom = [];

  function text(v, max) {
    return typeof v === 'string' ? Array.from(v.replace(/[\u0000-\u001f\u007f-\u009f]/g, ' ').trim()).slice(0, max).join('') : '';
  }

  function dimension(v, lim) {
    return typeof v === 'number' && isFinite(v) ? Math.min(lim.max, Math.max(lim.min, Math.round(v))) : lim.def;
  }

  // Custom overlays come from the server; anything malformed or sharing a built-in id is dropped.
  function setCustom(list) {
    const lim = CUSTOM_LIMITS;
    const seen = {};
    custom = (Array.isArray(list) ? list : []).filter(function (o) {
      const ok = o && typeof o.id === 'string' && CUSTOM_ID.test(o.id) && !seen[o.id] && !OVERLAYS.some(function (b) { return b.id === o.id; });
      if (ok) seen[o.id] = true;
      return ok;
    }).slice(0, lim.count).map(function (o) {
      return { id: o.id, path: '/overlays/' + o.id + '/', custom: true, params: [],
        name: text(o.name, lim.name) || o.id, description: text(o.description, lim.description),
        width: dimension(o.width, lim.width), height: dimension(o.height, lim.height) };
    });
  }

  function customList() {
    return custom;
  }

  function byId(id) {
    return OVERLAYS.concat(custom).find(function (o) { return o.id === id; }) || OVERLAYS[0];
  }

  function sizeText(overlay) {
    return overlay.width + ' x ' + overlay.height;
  }

  const api = { STATES: STATES, LANGS: LANGS, DEFAULTS: DEFAULTS, LIMITS: LIMITS, OVERLAYS: OVERLAYS, byId: byId, sizeText: sizeText, setCustom: setCustom, customList: customList };
  if (typeof module !== 'undefined' && module.exports) module.exports = api;
  else window.qscOverlayCatalog = api;
})();
