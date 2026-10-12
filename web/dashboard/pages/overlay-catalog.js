(function () {
  const STATES = ['initialization', 'menu', 'select', 'loading', 'gameplay', 'results', 'closed', 'unknown'];
  const LANGS = ['auto', 'en', 'fr', 'de'];
  const DEFAULTS = { autohide: 0, scale: 1, lang: 'auto', n: 20 };
  const LIMITS = { autohide: { min: 1, max: 3600 }, scale: { min: 0.5, max: 3 }, n: { min: 5, max: 50 } };

  // Recommended OBS browser source sizes; params lists the options each overlay honors.
  const OVERLAYS = [
    { id: 'now-playing', path: '/overlays/now-playing/', width: 540, height: 140,
      nameKey: 'overlays.now_playing.name', descKey: 'overlays.now_playing.description', params: ['autohide', 'scale', 'lang'] },
    { id: 'result', path: '/overlays/result/', width: 520, height: 230,
      nameKey: 'overlays.result.name', descKey: 'overlays.result.description', params: ['autohide', 'scale', 'lang'] },
    { id: 'history', path: '/overlays/history/', width: 520, height: 180,
      nameKey: 'overlays.history.name', descKey: 'overlays.history.description', params: ['scale', 'lang', 'n'] },
    { id: 'state', path: '/overlays/state/', width: 280, height: 64,
      nameKey: 'overlays.state.name', descKey: 'overlays.state.description', params: ['scale', 'lang', 'hide'] },
  ];

  function byId(id) {
    return OVERLAYS.find(function (o) { return o.id === id; }) || OVERLAYS[0];
  }

  function sizeText(overlay) {
    return overlay.width + ' x ' + overlay.height;
  }

  const api = { STATES: STATES, LANGS: LANGS, DEFAULTS: DEFAULTS, LIMITS: LIMITS, OVERLAYS: OVERLAYS, byId: byId, sizeText: sizeText };
  if (typeof module !== 'undefined' && module.exports) module.exports = api;
  else window.qscOverlayCatalog = api;
})();
