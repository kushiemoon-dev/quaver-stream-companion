(function () {
  const LANGUAGES = ['', 'en', 'fr', 'de'];
  const FIELDS = ['quaver_dir', 'port', 'language'];
  const PORT_RE = /^\d{1,5}$/;
  const LAYOUTS = ['auto', '4k', '7k'];
  const KEYS_FIELDS = { 'keys.layout': 'layout', 'keys.k4': 'k4', 'keys.k7': 'k7' };
  const LANES = { k4: 4, k7: 7 };
  const KEY_DEFAULTS = { enabled: false, layout: 'auto', k4: ['A', 'S', 'K', 'L'], k7: ['A', 'S', 'D', 'SPACE', 'J', 'K', 'L'] };
  // The names the backend accepts for a lane key.
  const KEY_NAMES = []
    .concat('ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789'.split(''))
    .concat(['SPACE', 'LEFT', 'UP', 'RIGHT', 'DOWN', 'SHIFT_L', 'SHIFT_R', 'CTRL_L', 'CTRL_R', 'ALT_L', 'ALT_R', ';', ',', '.', '/', "'"])
    .concat(Array.from({ length: 12 }, function (_, i) { return 'F' + (i + 1); }));

  function validatePort(raw) {
    const text = typeof raw === 'string' ? raw.trim() : '';
    if (!PORT_RE.test(text)) return { ok: false };
    const value = Number(text);
    return value >= 1 && value <= 65535 ? { ok: true, value: value } : { ok: false };
  }

  function isOverridden(overridden, name) {
    return Array.isArray(overridden) && overridden.indexOf(name) >= 0;
  }

  // PUT replaces the whole document: start from the current one and touch only this form's fields.
  function buildConfigBody(current, form, overridden) {
    const body = Object.assign({}, current);
    if (current.overlay) body.overlay = Object.assign({}, current.overlay);
    if (!isOverridden(overridden, 'quaver_dir')) body.quaver_dir = String(form.quaver_dir).trim();
    if (!isOverridden(overridden, 'port')) body.port = form.port;
    body.language = form.language;
    return body;
  }

  function fieldFor(apiField) {
    return FIELDS.indexOf(apiField) >= 0 ? apiField : '';
  }

  function validList(list, count) {
    return Array.isArray(list) && list.length === count && list.every(function (k) { return KEY_NAMES.indexOf(k) >= 0; });
  }

  // The keys section of a config as the form shows it; anything unusable falls back to the defaults.
  function keysFromConfig(keys) {
    const k = keys !== null && typeof keys === 'object' ? keys : {};
    return {
      enabled: k.enabled === true,
      layout: LAYOUTS.indexOf(k.layout) >= 0 ? k.layout : KEY_DEFAULTS.layout,
      k4: (validList(k.k4, LANES.k4) ? k.k4 : KEY_DEFAULTS.k4).slice(),
      k7: (validList(k.k7, LANES.k7) ? k.k7 : KEY_DEFAULTS.k7).slice(),
    };
  }

  // PUT replaces the whole document: only the keys section changes.
  function buildKeysBody(current, form) {
    const body = Object.assign({}, current);
    if (current.overlay) body.overlay = Object.assign({}, current.overlay);
    body.keys = Object.assign({}, current.keys, { enabled: form.enabled === true, layout: form.layout, k4: form.k4.slice(), k7: form.k7.slice() });
    return body;
  }

  function keysFieldFor(apiField) {
    return Object.prototype.hasOwnProperty.call(KEYS_FIELDS, apiField) ? KEYS_FIELDS[apiField] : '';
  }

  const api = {
    LANGUAGES: LANGUAGES, validatePort: validatePort, buildConfigBody: buildConfigBody, fieldFor: fieldFor, isOverridden: isOverridden,
    LAYOUTS: LAYOUTS, LANES: LANES, KEY_NAMES: KEY_NAMES, keysFromConfig: keysFromConfig, buildKeysBody: buildKeysBody, keysFieldFor: keysFieldFor,
  };
  if (typeof module !== 'undefined' && module.exports) module.exports = api;
  else window.qscSettingsModel = api;
})();
