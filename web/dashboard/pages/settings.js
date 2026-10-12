(function () {
  const qsc = window.qsc;
  const t = qsc.t;
  const h = qsc.h;
  const m = window.qscSettingsModel;
  const TIMEOUT_MS = 8000;
  const FIELDS = ['quaver_dir', 'port', 'language'];
  const LANG_ATTR = { en: 'en', fr: 'fr', de: 'de' };

  let mountId = 0;
  let lifetime = null;
  let ui = null;
  let current = null;
  let overridden = [];
  let savedNotice = false;

  function call(fn) {
    const life = lifetime;
    const own = new AbortController();
    const stop = function () { own.abort(); };
    life.signal.addEventListener('abort', stop);
    const timer = setTimeout(function () { own.abort(); }, TIMEOUT_MS);
    return fn(own.signal)
      .catch(function (e) {
        if (e && e.name === 'AbortError') {
          const err = new Error(t('settings.timeout'));
          err.timedOut = !life.signal.aborted;
          throw err;
        }
        throw e;
      })
      .finally(function () {
        clearTimeout(timer);
        life.signal.removeEventListener('abort', stop);
      });
  }

  function showStatus(kind, text) {
    ui.status.className = 'save-status ' + kind;
    ui.status.textContent = '';
    if (!text) return;
    requestAnimationFrame(function () { if (ui) ui.status.textContent = text; });
  }

  function setFieldError(name, text) {
    const f = ui.fields[name];
    f.error.textContent = text;
    if (text) f.control.setAttribute('aria-invalid', 'true');
    else f.control.removeAttribute('aria-invalid');
  }

  function clearErrors() {
    FIELDS.forEach(function (name) { setFieldError(name, ''); });
    ui.general.textContent = '';
  }

  function showBanner(restart) {
    const appeared = restart && ui.banner.hidden;
    ui.banner.hidden = !restart;
    return appeared;
  }

  function describedBy(name) {
    const f = ui.fields[name];
    const locked = m.isOverridden(overridden, name);
    return [f.help, locked ? f.noteId : null, f.errorId].filter(Boolean).join(' ');
  }

  function fillForm(res) {
    current = res.config;
    overridden = Array.isArray(res.overridden) ? res.overridden : [];
    const effective = res.effective || {};
    const f = ui.fields;
    f.quaver_dir.control.value = m.isOverridden(overridden, 'quaver_dir') ? String(effective.quaver_dir || '') : current.quaver_dir || '';
    f.port.control.value = String(m.isOverridden(overridden, 'port') ? effective.port : current.port);
    f.language.control.value = m.LANGUAGES.indexOf(current.language) >= 0 ? current.language : '';
    FIELDS.forEach(function (name) {
      const locked = m.isOverridden(overridden, name);
      f[name].control.disabled = locked;
      f[name].note.hidden = !locked;
      f[name].control.setAttribute('aria-describedby', describedBy(name));
    });
    const dir = String(effective.quaver_dir || '');
    ui.dirHelp.replaceChildren(dir ? t('settings.dir_effective') + ' ' : t('settings.dir_none'));
    if (dir) ui.dirHelp.appendChild(h('code', null, qsc.pathNodes(dir)));
    showBanner(res.restart_required === true);
  }

  function readForm() {
    const f = ui.fields;
    const port = m.validatePort(f.port.control.value);
    return {
      port: port,
      values: { quaver_dir: f.quaver_dir.control.value, port: port.value, language: f.language.control.value },
    };
  }

  function showApiError(e) {
    const name = m.fieldFor(e.field);
    if (name) {
      setFieldError(name, e.message);
      ui.fields[name].control.focus();
      showStatus('bad', t('settings.save_failed_short'));
      return;
    }
    ui.general.textContent = e.field ? e.field + ': ' + e.message : e.message;
    showStatus('bad', t('settings.save_failed_short'));
  }

  async function onSubmit(ev) {
    ev.preventDefault();
    if (ui.save.disabled) return;
    clearErrors();
    ui.reload.hidden = true;
    const form = readForm();
    if (!m.isOverridden(overridden, 'port') && !form.port.ok) {
      setFieldError('port', t('settings.port_invalid'));
      ui.fields.port.control.focus();
      showStatus('bad', t('settings.save_failed_short'));
      return;
    }
    const id = mountId;
    const before = current.language;
    let putting = false;
    ui.save.disabled = true;
    ui.reload.disabled = true;
    ui.save.setAttribute('aria-busy', 'true');
    showStatus('muted', t('settings.saving'));
    try {
      const fresh = await call(function (s) { return qsc.api.get('/api/config', s); });
      const body = m.buildConfigBody(fresh.config, form.values, overridden);
      putting = true;
      const res = await call(function (s) { return qsc.api.put('/api/config', body, s); });
      if (id !== mountId) return;
      current = res.config;
      const appeared = showBanner(res.restart_required === true);
      if (res.config.language !== before) {
        await window.qscI18n.load(res.config.language);
        savedNotice = id === mountId;
        qsc.relabel();
        qsc.rerender();
        return;
      }
      showStatus('ok', appeared ? t('settings.saved') + ' ' + t('settings.restart_title') + ' ' + t('settings.restart_body') : t('settings.saved'));
    } catch (e) {
      if (id !== mountId) return;
      if (putting && e.timedOut) {
        showStatus('bad', t('settings.save_unknown'));
        ui.reload.hidden = false;
      } else {
        showApiError(e);
      }
    }
    if (id !== mountId) return;
    ui.save.disabled = false;
    ui.reload.disabled = false;
    ui.save.removeAttribute('aria-busy');
    const active = document.activeElement;
    if (!active || active === document.body || active === ui.save) ui.save.focus();
  }

  async function onReload() {
    const id = mountId;
    ui.reload.disabled = true;
    ui.save.disabled = true;
    let ok = false;
    try {
      const res = await call(function (s) { return qsc.api.get('/api/config', s); });
      if (id !== mountId) return;
      clearErrors();
      fillForm(res);
      ui.reload.hidden = true;
      showStatus('muted', t('settings.reloaded'));
      ok = true;
    } catch (e) {
      if (id !== mountId) return;
      showStatus('bad', t('settings.load_failed', { message: e.message }));
    }
    ui.reload.disabled = false;
    ui.save.disabled = false;
    (ok ? ui.save : ui.reload).focus();
  }

  function field(name, id, labelKey, control, help) {
    const error = h('p', { id: id + '-error', class: 'field-error', role: 'alert' });
    const note = h('p', { id: id + '-note', class: 'muted hint', hidden: true }, t('settings.overridden'));
    const helpId = help ? help.getAttribute('id') : null;
    control.setAttribute('aria-describedby', [helpId, id + '-error'].filter(Boolean).join(' '));
    ui.fields[name] = { control: control, error: error, note: note, help: helpId, noteId: id + '-note', errorId: id + '-error' };
    return h('div', { class: 'field' }, h('label', { for: id }, t(labelKey)), control, help, note, error);
  }

  function buildForm() {
    ui.fields = {};
    ui.dirHelp = h('p', { id: 'set-dir-help', class: 'muted hint' });
    ui.general = h('p', { class: 'field-error', role: 'alert' });
    ui.banner = h('div', { class: 'banner', role: 'status', hidden: true },
      h('strong', null, t('settings.restart_title')), ' ', t('settings.restart_body'));
    ui.status = h('p', { class: 'save-status', role: 'status', 'aria-live': 'polite' });
    ui.save = h('button', { type: 'submit', class: 'btn btn-primary' }, t('settings.save'));
    ui.reload = h('button', { type: 'button', class: 'btn', hidden: true, onclick: onReload }, t('settings.reload'));
    const dir = h('input', { id: 'set-dir', type: 'text', autocomplete: 'off', spellcheck: 'false' });
    const port = h('input', { id: 'set-port', type: 'text', inputmode: 'numeric', autocomplete: 'off' });
    const lang = h('select', { id: 'set-lang' }, m.LANGUAGES.map(function (code) {
      return h('option', { value: code, lang: LANG_ATTR[code] }, t('settings.lang.' + (code || 'auto')));
    }));
    return h('form', { class: 'card settings-form', onsubmit: onSubmit },
      ui.banner,
      field('quaver_dir', 'set-dir', 'settings.dir', dir, ui.dirHelp),
      field('port', 'set-port', 'settings.port', port, h('p', { id: 'set-port-help', class: 'muted hint' }, t('settings.port_help'))),
      field('language', 'set-lang', 'settings.language', lang, h('p', { id: 'set-lang-help', class: 'muted hint' }, t('settings.language_help'))),
      ui.general,
      h('div', { class: 'save-row' }, ui.save, ui.reload, ui.status));
  }

  function buildInfo() {
    ui.info = h('dl', { class: 'info-list' });
    return h('section', { class: 'card', 'aria-labelledby': 'set-info-title' },
      h('h2', { id: 'set-info-title', class: 'card-label' }, t('settings.about')), ui.info);
  }

  function renderInfo(status) {
    const rows = [];
    if (status.config_path) rows.push([t('settings.config_file'), h('code', null, qsc.pathNodes(status.config_path))]);
    if (status.version) rows.push([t('settings.version'), status.version]);
    ui.info.replaceChildren.apply(ui.info, rows.map(function (r) {
      return h('div', { class: 'info-row' }, h('dt', { class: 'muted' }, r[0]), h('dd', null, r[1]));
    }));
  }

  async function loadInfo(id) {
    try {
      const status = await call(function (s) { return qsc.api.get('/api/status', s); });
      if (id === mountId) renderInfo(status || {});
    } catch (e) {
      if (id === mountId) ui.info.replaceChildren(h('p', { class: 'muted' }, t('settings.info_unavailable')));
    }
  }

  function showFailure(root, e) {
    root.replaceChildren(h('section', { class: 'card empty' },
      h('p', { class: 'save-status bad', role: 'alert' }, t('settings.load_failed', { message: e.message })),
      h('button', { type: 'button', class: 'btn', onclick: function () { load(root, true); } }, t('settings.retry'))));
    const retry = root.querySelector('button');
    if (retry) retry.focus();
  }

  async function load(root, refocus) {
    const id = mountId;
    const notice = savedNotice;
    savedNotice = false;
    root.replaceChildren(h('p', { class: 'muted', role: 'status' }, t('settings.loading')));
    try {
      const res = await call(function (s) { return qsc.api.get('/api/config', s); });
      if (id !== mountId) return;
      root.replaceChildren(buildForm(), window.qscSettingsKeys.build({ call: call, alive: function () { return id === mountId; } }, res.config.keys), buildInfo());
      fillForm(res);
      if (refocus) ui.fields.quaver_dir.control.focus();
      if (notice) {
        showStatus('ok', t('settings.saved'));
        ui.save.focus();
      }
      loadInfo(id);
    } catch (e) {
      if (id !== mountId) return;
      showFailure(root, e);
    }
  }

  function mount(root) {
    mountId++;
    lifetime = new AbortController();
    ui = null;
    current = null;
    ui = {};
    root.appendChild(h('header', { class: 'page-head' },
      h('div', null, h('h1', null, t('nav.settings')), h('p', { class: 'subtitle' }, t('settings.subtitle')))));
    const body = h('div', { class: 'settings-body' });
    root.appendChild(body);
    load(body, false);
  }

  function unmount() {
    mountId++;
    if (lifetime) lifetime.abort();
    lifetime = null;
    ui = null;
    current = null;
  }

  qsc.registerPage('settings', { title: 'nav.settings', mount: mount, unmount: unmount });
})();
