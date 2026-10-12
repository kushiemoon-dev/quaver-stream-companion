(function () {
  const qsc = window.qsc;
  const m = window.qscSettingsModel;
  const LIST_LAYOUT = { k4: '4K', k7: '7K' };

  // ctx.call(fn) runs a request with a timeout and the page lifetime, ctx.alive() tells whether the page is still mounted.
  function build(ctx, keysConfig) {
    const t = qsc.t;
    const h = qsc.h;
    const values = m.keysFromConfig(keysConfig);
    const fields = {};

    const enabled = h('input', { id: 'set-keys-enabled', type: 'checkbox', checked: values.enabled, 'aria-describedby': 'set-keys-enabled-hint' });
    const layoutSelect = h('select', { id: 'set-keys-layout', 'aria-describedby': 'set-keys-layout-help set-keys-layout-error' },
      m.LAYOUTS.map(function (l) { return h('option', { value: l }, t('settings.keys.layout.' + l)); }));
    layoutSelect.value = values.layout;

    const support = h('p', { class: 'muted hint', role: 'status', 'aria-live': 'polite' });
    const banner = h('div', { class: 'banner', role: 'status', hidden: true },
      h('strong', null, t('settings.restart_title')), ' ', t('settings.restart_body'));
    const general = h('p', { class: 'field-error', role: 'alert' });
    const status = h('p', { class: 'save-status', role: 'status', 'aria-live': 'polite' });
    const save = h('button', { type: 'submit', class: 'btn btn-primary' }, t('settings.keys.save'));

    fields.layout = { controls: [layoutSelect], error: h('p', { id: 'set-keys-layout-error', class: 'field-error', role: 'alert' }) };

    function list(name) {
      const label = LIST_LAYOUT[name];
      const selects = values[name].map(function (key, i) {
        const select = h('select', { 'aria-label': t('settings.keys.lane', { layout: label, n: i + 1 }), 'aria-describedby': 'set-keys-' + name + '-error' },
          m.KEY_NAMES.map(function (k) { return h('option', { value: k }, k); }));
        select.value = key;
        return select;
      });
      fields[name] = { controls: selects, error: h('p', { id: 'set-keys-' + name + '-error', class: 'field-error', role: 'alert' }) };
      return h('fieldset', { class: 'seg-group field' }, h('legend', null, t('settings.keys.' + name)),
        h('div', { class: 'key-grid' }, selects), fields[name].error);
    }

    function setError(name, text) {
      const f = fields[name];
      f.error.textContent = text;
      f.controls.forEach(function (c) {
        if (text) c.setAttribute('aria-invalid', 'true');
        else c.removeAttribute('aria-invalid');
      });
    }

    function clearErrors() {
      Object.keys(fields).forEach(function (name) { setError(name, ''); });
      general.textContent = '';
    }

    function showStatus(kind, text) {
      status.className = 'save-status ' + kind;
      status.textContent = text;
    }

    function readForm() {
      return {
        enabled: enabled.checked,
        layout: layoutSelect.value,
        k4: fields.k4.controls.map(function (s) { return s.value; }),
        k7: fields.k7.controls.map(function (s) { return s.value; }),
      };
    }

    function showError(e) {
      const name = m.keysFieldFor(e.field);
      if (name) {
        setError(name, e.message);
        fields[name].controls[0].focus();
      } else {
        general.textContent = e.field ? e.field + ': ' + e.message : e.message;
      }
      showStatus('bad', t('settings.save_failed_short'));
    }

    async function onSubmit(ev) {
      ev.preventDefault();
      if (save.disabled) return;
      clearErrors();
      save.disabled = true;
      save.setAttribute('aria-busy', 'true');
      showStatus('muted', t('settings.saving'));
      try {
        const fresh = await ctx.call(function (s) { return qsc.api.get('/api/config', s); });
        const body = m.buildKeysBody(fresh.config, readForm());
        const res = await ctx.call(function (s) { return qsc.api.put('/api/config', body, s); });
        if (!ctx.alive()) return;
        banner.hidden = res.restart_required !== true;
        showStatus('ok', t('settings.keys.saved'));
      } catch (e) {
        if (!ctx.alive()) return;
        showError(e);
      }
      if (!ctx.alive()) return;
      save.disabled = false;
      save.removeAttribute('aria-busy');
    }

    ctx.call(function (s) { return qsc.api.get('/api/status', s); }).then(function (res) {
      if (!ctx.alive()) return;
      const ok = res && res.keys && res.keys.supported === true;
      support.textContent = t(ok ? 'settings.keys.status.ok' : 'settings.keys.status.unsupported');
    }, function () {
      if (ctx.alive()) support.textContent = t('settings.keys.status.unknown');
    });

    return h('form', { class: 'card settings-form', 'aria-labelledby': 'set-keys-title', onsubmit: onSubmit },
      h('h2', { id: 'set-keys-title', class: 'card-label' }, t('settings.keys.title')),
      h('p', { class: 'hint' }, t('settings.keys.disclosure')),
      support,
      banner,
      h('div', { class: 'field' },
        h('label', { class: 'check' }, enabled, h('span', null, t('settings.keys.enabled'))),
        h('p', { id: 'set-keys-enabled-hint', class: 'muted hint' }, t('settings.keys.enabled_hint'))),
      h('div', { class: 'field' }, h('label', { for: 'set-keys-layout' }, t('settings.keys.layout')), layoutSelect,
        h('p', { id: 'set-keys-layout-help', class: 'muted hint' }, t('settings.keys.layout_help')), fields.layout.error),
      list('k4'),
      list('k7'),
      general,
      h('div', { class: 'save-row' }, save, status));
  }

  window.qscSettingsKeys = { build: build };
})();
