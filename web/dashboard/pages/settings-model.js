(function () {
  const LANGUAGES = ['', 'en', 'fr', 'de'];
  const FIELDS = ['quaver_dir', 'port', 'language'];
  const PORT_RE = /^\d{1,5}$/;
  const MAX_KEYS = 10;
  const COUNTS = Array.from({ length: MAX_KEYS }, function (_, i) { return i + 1; });
  const LISTS = COUNTS.map(function (n) { return 'k' + n; });
  const LAYOUTS = ['auto'].concat(COUNTS.map(function (n) { return n + 'k'; }));
  const KEYS_FIELDS = { 'keys.layout': 'layout', 'keys.scratch': 'scratch' };
  const LANES = {};
  LISTS.forEach(function (name, i) { LANES[name] = i + 1; KEYS_FIELDS['keys.' + name] = name; });
  // Quaver's own default bindings, like the backend.
  const LIST_DEFAULTS = {
    k1: ['SPACE'], k2: ['A', 'L'], k3: ['A', 'SPACE', 'L'], k4: ['A', 'S', 'K', 'L'], k5: ['A', 'S', 'SPACE', 'K', 'L'],
    k6: ['A', 'S', 'D', 'J', 'K', 'L'], k7: ['A', 'S', 'D', 'SPACE', 'J', 'K', 'L'], k8: ['A', 'S', 'D', 'F', 'H', 'J', 'K', 'L'],
    k9: ['A', 'S', 'D', 'F', 'SPACE', 'H', 'J', 'K', 'L'], k10: ['A', 'S', 'D', 'F', 'V', 'B', 'H', 'J', 'K', 'L'],
  };
  const SCRATCH_DEFAULT = 'SHIFT_L';
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
    if (typeof form.watch_quaver_process === 'boolean') body.watch_quaver_process = form.watch_quaver_process;
    return body;
  }

  // A config without the key (a v0.1.0 file) means the check is on.
  function watchFromConfig(config) {
    return !config || config.watch_quaver_process !== false;
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
    const out = {
      enabled: k.enabled === true,
      layout: LAYOUTS.indexOf(k.layout) >= 0 ? k.layout : 'auto',
      scratch: KEY_NAMES.indexOf(k.scratch) >= 0 ? k.scratch : SCRATCH_DEFAULT,
    };
    LISTS.forEach(function (name) { out[name] = (validList(k[name], LANES[name]) ? k[name] : LIST_DEFAULTS[name]).slice(); });
    return out;
  }

  // PUT replaces the whole document: only the keys section changes.
  function buildKeysBody(current, form) {
    const body = Object.assign({}, current);
    if (current.overlay) body.overlay = Object.assign({}, current.overlay);
    const next = Object.assign({}, current.keys, { enabled: form.enabled === true, layout: form.layout, scratch: form.scratch });
    // A scratch key the user never chose stays absent: the backend then uses its default without checking it against the lane lists.
    if (form.scratch === SCRATCH_DEFAULT && !(current.keys && current.keys.scratch)) delete next.scratch;
    LISTS.forEach(function (name) { next[name] = form[name].slice(); });
    body.keys = next;
    return body;
  }

  function keysFieldFor(apiField) {
    return Object.prototype.hasOwnProperty.call(KEYS_FIELDS, apiField) ? KEYS_FIELDS[apiField] : '';
  }

  const api = {
    LANGUAGES: LANGUAGES, watchFromConfig: watchFromConfig, validatePort: validatePort, buildConfigBody: buildConfigBody, fieldFor: fieldFor, isOverridden: isOverridden,
    LAYOUTS: LAYOUTS, LISTS: LISTS, LANES: LANES, KEY_NAMES: KEY_NAMES, keysFromConfig: keysFromConfig, buildKeysBody: buildKeysBody, keysFieldFor: keysFieldFor,
  };
  if (typeof module !== 'undefined' && module.exports) module.exports = api;
  else window.qscSettingsModel = api;
})();
