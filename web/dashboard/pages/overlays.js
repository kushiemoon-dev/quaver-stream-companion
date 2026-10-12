(function () {
  const qsc = window.qsc;
  const t = qsc.t;
  const h = qsc.h;
  const url = window.qscOverlayUrl;
  const FALLBACK_THEME = 'neutral-light';
  const PICKER_START = '#a855f7';
  const TIMEOUT_MS = 8000;
  const PREVIEW_DELAY_MS = 250;
  const COPIED_MS = 2000;

  let mountId = 0;
  let lifetime = null;
  let ui = null;
  let model = null;
  let previewTimer = null;
  let copiedTimer = null;

  function pickerValue(hex) {
    if (!url.isValidAccent(hex)) return PICKER_START;
    const rgb = hex.length <= 4
      ? hex.slice(0, 3).split('').map(function (c) { return c + c; }).join('')
      : hex.slice(0, 6);
    return '#' + rgb;
  }

  function call(fn) {
    const life = lifetime;
    const own = new AbortController();
    const stop = function () { own.abort(); };
    life.signal.addEventListener('abort', stop);
    const timer = setTimeout(function () { own.abort(); }, TIMEOUT_MS);
    return fn(own.signal)
      .catch(function (e) {
        if (e && e.name === 'AbortError') {
          const err = new Error(t('overlays.timeout'));
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

  function describe(e) {
    return e.field ? e.field + ': ' + e.message : e.message;
  }

  function field(id, labelKey, control, extra) {
    return h('div', { class: 'field' }, h('label', { for: id }, t(labelKey)), control, extra);
  }

  function refreshPreview() {
    clearTimeout(previewTimer);
    previewTimer = setTimeout(function () {
      if (ui) ui.frame.setAttribute('src', url.buildOverlayUrl(model, true, location.origin));
    }, PREVIEW_DELAY_MS);
  }

  function clearSaved() {
    if (ui.saveStatus.classList.contains('ok')) showSave('muted', '');
  }

  function refreshUrl() {
    ui.url.value = url.buildOverlayUrl(model, false, location.origin);
    ui.copyStatus.textContent = '';
  }

  function setAccent(hex, fromPicker) {
    const valid = hex === '' || url.isValidAccent(hex);
    ui.hex.setAttribute('aria-invalid', valid ? 'false' : 'true');
    ui.hexError.textContent = valid ? '' : t('overlays.accent_invalid');
    if (fromPicker) ui.hex.value = hex;
    if (!valid) return;
    ui.picker.value = pickerValue(hex);
    model = Object.assign({}, model, { accent: hex });
    clearSaved();
    refreshUrl();
    refreshPreview();
  }

  function onCopy() {
    const manual = function () {
      ui.url.focus();
      ui.url.select();
      ui.copyStatus.textContent = t('overlays.copy_manual');
    };
    if (!navigator.clipboard || !navigator.clipboard.writeText) {
      manual();
      return;
    }
    navigator.clipboard.writeText(ui.url.value).then(function () {
      if (!ui) return;
      ui.copy.textContent = t('overlays.copied');
      ui.copyStatus.textContent = '';
      requestAnimationFrame(function () {
        if (ui) ui.copyStatus.textContent = t('overlays.copied_live');
      });
      clearTimeout(copiedTimer);
      copiedTimer = setTimeout(function () {
        if (ui) ui.copy.textContent = t('overlays.copy');
      }, COPIED_MS);
    }, function () { if (ui) manual(); });
  }

  function showSave(kind, text) {
    ui.saveStatus.className = 'save-status ' + kind;
    ui.saveStatus.textContent = text;
  }

  async function onSave() {
    if (ui.hex.getAttribute('aria-invalid') === 'true') {
      ui.hex.focus();
      return;
    }
    const id = mountId;
    const saved = { theme: model.theme, accent: model.accent };
    let putting = false;
    ui.reload.hidden = true;
    ui.save.disabled = true;
    ui.save.setAttribute('aria-busy', 'true');
    showSave('muted', t('overlays.saving'));
    try {
      const current = await call(function (s) { return qsc.api.get('/api/config', s); });
      const next = Object.assign({}, current.config, {
        overlay: Object.assign({}, current.config.overlay, saved),
      });
      putting = true;
      await call(function (s) { return qsc.api.put('/api/config', next, s); });
      if (id !== mountId) return;
      model = Object.assign({}, model, { defaultTheme: saved.theme, defaultAccent: saved.accent });
      refreshUrl();
      refreshThemeOptions();
      showSave('ok', t('overlays.saved'));
    } catch (e) {
      if (id !== mountId) return;
      if (putting && e.timedOut) {
        showSave('bad', t('overlays.save_unknown'));
        ui.reload.hidden = false;
      } else {
        showSave('bad', t('overlays.save_failed', { message: describe(e) }));
      }
    }
    if (id !== mountId) return;
    ui.save.disabled = false;
    ui.save.removeAttribute('aria-busy');
  }

  async function onReload() {
    const id = mountId;
    ui.reload.disabled = true;
    try {
      const current = await call(function (s) { return qsc.api.get('/api/config', s); });
      if (id !== mountId) return;
      const overlay = (current.config && current.config.overlay) || {};
      model = Object.assign({}, model, {
        defaultTheme: overlay.theme || FALLBACK_THEME,
        defaultAccent: url.normalizeAccent(overlay.accent || ''),
      });
      refreshUrl();
      refreshThemeOptions();
      ui.reload.hidden = true;
      showSave('muted', t('overlays.reloaded'));
    } catch (e) {
      if (id !== mountId) return;
      showSave('bad', t('overlays.save_failed', { message: describe(e) }));
    }
    if (id === mountId) ui.reload.disabled = false;
  }

  function refreshThemeOptions() {
    ui.theme.replaceChildren.apply(ui.theme, model.themes.map(function (name) {
      const label = name === model.defaultTheme ? t('overlays.theme_default', { name: name }) : name;
      return h('option', { value: name, selected: name === model.theme }, label);
    }));
    ui.theme.value = model.theme;
  }

  function buildCard() {
    ui = {
      url: h('input', { id: 'ov-url', class: 'url-input', type: 'text', readonly: true, spellcheck: 'false' }),
      copy: h('button', { type: 'button', class: 'btn', 'aria-label': t('overlays.copy_aria'), onclick: onCopy }, t('overlays.copy')),
      copyStatus: h('p', { class: 'muted status-line', role: 'status', 'aria-live': 'polite' }),
      theme: h('select', { id: 'ov-theme', onchange: function (ev) {
        model = Object.assign({}, model, { theme: ev.target.value });
        clearSaved();
        refreshUrl();
        refreshPreview();
      } }),
      picker: h('input', { id: 'ov-accent', type: 'color', class: 'picker', oninput: function (ev) {
        setAccent(url.normalizeAccent(ev.target.value), true);
      } }),
      hex: h('input', { id: 'ov-hex', type: 'text', class: 'hex-input', autocomplete: 'off', spellcheck: 'false',
        maxlength: '12', 'aria-describedby': 'ov-hex-hint ov-hex-error', oninput: function (ev) {
          setAccent(url.normalizeAccent(ev.target.value), false);
        } }),
      hexError: h('p', { id: 'ov-hex-error', class: 'field-error', role: 'alert' }),
      save: h('button', { type: 'button', class: 'btn btn-primary', onclick: onSave }, t('overlays.save')),
      reload: h('button', { type: 'button', class: 'btn', hidden: true, onclick: onReload }, t('overlays.reload')),
      saveStatus: h('p', { class: 'save-status', role: 'status', 'aria-live': 'polite' }),
      frame: h('iframe', { class: 'preview-frame', title: t('overlays.preview_title'), loading: 'lazy',
        sandbox: 'allow-scripts' }),
    };
    refreshThemeOptions();
    ui.hex.value = model.accent;
    ui.picker.value = pickerValue(model.accent);
    refreshUrl();
    ui.frame.setAttribute('src', url.buildOverlayUrl(model, true, location.origin));

    return h('section', { class: 'card overlay-card', 'aria-labelledby': 'ov-name' },
      h('h2', { id: 'ov-name', class: 'overlay-name' }, t('overlays.now_playing.name')),
      h('p', { class: 'muted' }, t('overlays.now_playing.description')),
      h('div', { class: 'field' }, h('label', { for: 'ov-url' }, t('overlays.url')),
        h('div', { class: 'url-row' }, ui.url, ui.copy), ui.copyStatus),
      h('div', { class: 'overlay-body' },
        h('div', { class: 'controls' },
          h('h3', { class: 'card-label' }, t('overlays.appearance')),
          field('ov-theme', 'overlays.theme', ui.theme),
          h('div', { class: 'accent-row' },
            field('ov-accent', 'overlays.accent', ui.picker),
            field('ov-hex', 'overlays.accent_hex', ui.hex)),
          h('p', { id: 'ov-hex-hint', class: 'muted hint' }, t('overlays.accent_hint')),
          ui.hexError,
          h('div', { class: 'save-row' }, ui.save, ui.reload, ui.saveStatus)),
        h('div', { class: 'preview' },
          h('h3', { class: 'card-label' }, t('overlays.preview')),
          h('div', { class: 'checker' }, ui.frame))));
  }

  function showFailure(root, e) {
    root.replaceChildren(h('section', { class: 'card empty' },
      h('p', { class: 'save-status bad', role: 'alert' }, t('overlays.load_failed', { message: e.message })),
      h('button', { type: 'button', class: 'btn', onclick: function () { load(root, true); } }, t('overlays.retry'))));
  }

  async function load(root, refocus) {
    const id = mountId;
    root.replaceChildren(h('p', { class: 'muted', role: 'status' }, t('overlays.loading')));
    try {
      const res = await Promise.all([
        call(function (s) { return qsc.api.get('/api/themes', s); }),
        call(function (s) { return qsc.api.get('/api/config', s); }),
      ]);
      if (id !== mountId) return;
      const overlay = (res[1].config && res[1].config.overlay) || {};
      const theme = overlay.theme || FALLBACK_THEME;
      const accent = url.normalizeAccent(overlay.accent || '');
      const themes = Array.isArray(res[0]) ? res[0].slice() : [];
      if (themes.indexOf(theme) < 0) themes.unshift(theme);
      model = { themes: themes, theme: theme, accent: accent, defaultTheme: theme, defaultAccent: accent };
      root.replaceChildren(buildCard());
      if (refocus) ui.theme.focus();
    } catch (e) {
      if (id !== mountId) return;
      showFailure(root, e);
    }
  }

  function mount(root) {
    mountId++;
    lifetime = new AbortController();
    ui = null;
    model = null;
    root.appendChild(h('header', { class: 'page-head' },
      h('div', null, h('h1', null, t('nav.overlays')), h('p', { class: 'subtitle' }, t('overlays.subtitle')))));
    const body = h('div', { class: 'overlays-body' });
    root.appendChild(body);
    load(body);
  }

  function unmount() {
    mountId++;
    if (lifetime) lifetime.abort();
    lifetime = null;
    clearTimeout(previewTimer);
    clearTimeout(copiedTimer);
    ui = null;
    model = null;
  }

  qsc.registerPage('overlays', { title: 'nav.overlays', mount: mount, unmount: unmount });
})();
