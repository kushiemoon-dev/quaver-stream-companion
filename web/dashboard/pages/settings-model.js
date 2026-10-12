(function () {
  const LANGUAGES = ['', 'en', 'fr', 'de'];
  const FIELDS = ['quaver_dir', 'port', 'language'];
  const PORT_RE = /^\d{1,5}$/;

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

  const api = { LANGUAGES: LANGUAGES, validatePort: validatePort, buildConfigBody: buildConfigBody, fieldFor: fieldFor, isOverridden: isOverridden };
  if (typeof module !== 'undefined' && module.exports) module.exports = api;
  else window.qscSettingsModel = api;
})();
