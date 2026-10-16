(function () {
  const qsc = window.qsc;
  const t = qsc.t;
  const h = qsc.h;
  const url = window.qscOverlayUrl;
  const tm = window.qscThemeModel;
  const cat = window.qscOverlayCatalog;
  const controls = window.qscOverlayControls;
  const custom = window.qscOverlayCustom;
  const util = window.qscOverlayUtil;
  const pickerValue = util.pickerValue;
  const describe = util.describe;
  const FALLBACK_THEME = 'neutral-light';
  const PREVIEW_DELAY_MS = 250;
  const COPIED_MS = 2000;

  let mountId = 0;
  let lifetime = null;
  let ui = null;
  let model = null;
  let previewTimer = null;
  let copiedTimer = null;
  const call = util.makeCall(function () { return lifetime; });

  function field(id, labelKey, control, extra) {
    return h('div', { class: 'field' }, h('label', { for: id }, t(labelKey)), control, extra);
  }

  function refreshPreview() {
    clearTimeout(previewTimer);
    if (ui) custom.frameTitle(ui.frame, cat.byId(model.overlay), model.demo);
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

  function update(patch) {
    model = Object.assign({}, model, patch);
    refreshUrl();
    refreshPreview();
  }

  function selectOverlay(id) {
    const overlay = cat.byId(id);
    const name = custom.name(overlay);
    model = Object.assign({}, model, { overlay: overlay.id });
    ui.name.textContent = name;
    ui.desc.textContent = custom.desc(overlay);
    ui.size.textContent = cat.sizeText(overlay);
    ui.sizeStatus.textContent = '';
    ui.frame.className = custom.frameClass(overlay);
    custom.frameHeight(ui.frame, overlay);
    ui.copy.setAttribute('aria-label', t('overlays.copy_aria', { name: name }));
    ui.options.sync(overlay);
    refreshUrl();
    refreshPreview();
  }

  function onCopySize() {
    const text = cat.sizeText(cat.byId(model.overlay));
    const done = function (msg) { if (ui) ui.sizeStatus.textContent = msg; };
    if (!navigator.clipboard || !navigator.clipboard.writeText) {
      done(t('overlays.size_manual', { size: text }));
      return;
    }
    navigator.clipboard.writeText(text).then(function () { done(t('overlays.size_copied')); },
      function () { done(t('overlays.size_manual', { size: text })); });
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

  function swatch(color) {
    const el = h('span', { class: color ? 'theme-swatch' : 'theme-swatch is-empty' });
    if (color) el.style.setProperty('--sw', color);
    return el;
  }

  function onTheme(ev) {
    if (!ev.target.checked) return;
    model = Object.assign({}, model, { theme: ev.target.value });
    clearSaved();
    refreshUrl();
    refreshPreview();
  }

  function themeCard(card) {
    const tags = [];
    if (card.isDefault) tags.push(h('span', { class: 'pill pill-ok' }, t('overlays.theme_default')));
    if (card.custom) tags.push(h('span', { class: 'pill pill-muted' }, t('overlays.theme_custom')));
    return h('label', { class: 'theme-card' },
      h('input', { type: 'radio', class: 'theme-input', name: 'ov-theme', value: card.id, checked: card.selected, onchange: onTheme }),
      h('span', { class: 'theme-body' },
        h('span', { class: 'theme-swatches', 'aria-hidden': 'true' },
          swatch(card.colors.bg), swatch(card.colors.fg), swatch(card.colors.accent)),
        h('span', { class: 'theme-name' }, card.name),
        tags.length > 0 && h('span', { class: 'theme-tags' }, tags)));
  }

  function refreshThemeOptions() {
    const cards = tm.buildCards(model.themes, model.theme, model.defaultTheme);
    ui.grid.replaceChildren.apply(ui.grid, cards.map(themeCard));
  }

  function buildCard() {
    ui = {
      url: h('input', { id: 'ov-url', class: 'url-input', type: 'text', readonly: true, spellcheck: 'false' }),
      copy: h('button', { type: 'button', class: 'btn', onclick: onCopy }, t('overlays.copy')),
      copyStatus: h('p', { class: 'muted status-line', role: 'status', 'aria-live': 'polite' }),
      grid: h('div', { class: 'theme-grid' }),
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
      name: h('h2', { id: 'ov-name', class: 'overlay-name' }),
      desc: h('p', { class: 'muted' }),
      size: h('strong', { id: 'ov-size' }),
      copySize: h('button', { type: 'button', class: 'btn', 'aria-label': t('overlays.copy_size_aria'), 'aria-describedby': 'ov-size', onclick: onCopySize }, t('overlays.copy')),
      sizeStatus: h('p', { class: 'muted status-line', role: 'status', 'aria-live': 'polite' }),
      options: controls.options(model, update),
      demo: controls.demoSwitch(model.demo, function (on) { update({ demo: on }); }),
      frame: h('iframe', { class: 'preview-frame', title: '', loading: 'lazy',
        sandbox: 'allow-scripts' }),
    };
    refreshThemeOptions();
    ui.hex.value = model.accent;
    ui.picker.value = pickerValue(model.accent);
    refreshUrl();
    ui.picked = controls.picker(model.overlay, selectOverlay);
    selectOverlay(model.overlay);
    clearTimeout(previewTimer);
    ui.frame.setAttribute('src', url.buildOverlayUrl(model, true, location.origin));

    return h('section', { class: 'card overlay-card', 'aria-labelledby': 'ov-name' },
      ui.picked, custom.dirHelp(model.overlaysDir), ui.name, ui.desc,
      h('div', { class: 'field' }, h('label', { for: 'ov-url' }, t('overlays.url')),
        h('div', { class: 'url-row' }, ui.url, ui.copy), ui.copyStatus),
      h('div', { class: 'overlay-body' },
        h('div', { class: 'controls' },
          ui.options.el,
          h('h3', { class: 'card-label' }, t('overlays.appearance')),
          h('fieldset', { class: 'theme-picker' }, h('legend', null, t('overlays.theme')), ui.grid),
          model.themesDir && h('p', { class: 'muted hint' }, t('overlays.themes_dir_help'), ' ', h('code', null, qsc.pathNodes(model.themesDir))),
          h('div', { class: 'accent-row' },
            field('ov-accent', 'overlays.accent', ui.picker),
            field('ov-hex', 'overlays.accent_hex', ui.hex)),
          h('p', { id: 'ov-hex-hint', class: 'muted hint' }, t('overlays.accent_hint')),
          ui.hexError,
          h('div', { class: 'save-row' }, ui.save, ui.reload, ui.saveStatus)),
        h('div', { class: 'preview' },
          h('div', { class: 'size-row' }, h('span', null, t('overlays.size')), ui.size, ui.copySize), ui.sizeStatus,
          h('h3', { class: 'card-label' }, t('overlays.preview')),
          ui.demo,
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
        call(function (s) { return qsc.api.get('/api/overlays', s); }).catch(function () { return {}; }),
        util.initialDemo(call),
      ]);
      if (id !== mountId) return;
      cat.setCustom((res[2] || {}).custom);
      const overlay = (res[1].config && res[1].config.overlay) || {};
      const theme = overlay.theme || FALLBACK_THEME;
      const accent = url.normalizeAccent(overlay.accent || '');
      model = { overlay: cat.OVERLAYS[0].id, autohide: '', scale: '', lang: 'auto', n: '', best: false, sound: false, meta: false, demo: res[3], counts: true, peak: true, hide: [], themes: Array.isArray(res[0]) ? res[0] : [], themesDir: res[1].themes_dir || '', overlaysDir: typeof (res[2] || {}).dir === 'string' ? res[2].dir : '', theme: theme, accent: accent, defaultTheme: theme, defaultAccent: accent };
      root.replaceChildren(buildCard());
      const target = ui.grid.querySelector('.theme-input:checked') || ui.grid.querySelector('.theme-input');
      if (refocus && target) target.focus();
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
