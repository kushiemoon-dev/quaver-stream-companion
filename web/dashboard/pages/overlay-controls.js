(function () {
  const qsc = window.qsc;
  const t = qsc.t;
  const h = qsc.h;
  const cat = window.qscOverlayCatalog;
  const custom = window.qscOverlayCustom;
  const LANG_LABEL = { auto: 'settings.lang.auto', en: 'settings.lang.en', fr: 'settings.lang.fr', de: 'settings.lang.de' };

  // One radio card per overlay (built-in first, then custom): name, one-line description and recommended size.
  function picker(selected, onSelect) {
    const cards = cat.OVERLAYS.concat(cat.customList()).map(function (o) {
      return h('label', { class: 'theme-card' },
        h('input', { type: 'radio', class: 'theme-input', name: 'ov-overlay', value: o.id, checked: o.id === selected,
          onchange: function (ev) { if (ev.target.checked) onSelect(o.id); } }),
        h('span', { class: 'theme-body' },
          h('span', { class: 'theme-name' }, custom.name(o)),
          h('span', { class: 'muted overlay-desc' }, custom.desc(o)),
          h('span', { class: 'theme-tags' },
            o.custom && h('span', { class: 'pill pill-amber' }, t('overlays.custom')),
            h('span', { class: 'pill pill-muted' }, cat.sizeText(o)))));
    });
    return h('fieldset', { class: 'theme-picker' }, h('legend', null, t('overlays.choose')),
      h('div', { class: 'theme-grid overlay-grid' }, cards));
  }

  function numberField(id, labelKey, hintKey, attrs, onValue) {
    const input = h('input', Object.assign({ id: id, type: 'number', inputmode: 'decimal', 'aria-describedby': id + '-hint',
      oninput: function (ev) { onValue(ev.target.value); } }, attrs));
    return h('div', { class: 'field' }, h('label', { for: id }, t(labelKey)), input,
      h('p', { id: id + '-hint', class: 'muted hint' }, t(hintKey)));
  }

  function langGroup(model, onChange) {
    const radios = cat.LANGS.map(function (code) {
      return h('label', { class: 'seg' },
        h('input', { type: 'radio', class: 'seg-input', name: 'ov-lang', value: code, checked: code === model.lang,
          onchange: function (ev) { if (ev.target.checked) onChange({ lang: code }); } }),
        h('span', { class: 'seg-label' }, t(LANG_LABEL[code])));
    });
    return h('fieldset', { class: 'seg-group field' }, h('legend', null, t('overlays.lang')), h('div', { class: 'seg-row' }, radios),
      h('p', { class: 'muted hint' }, t('overlays.lang_hint')));
  }

  function hideGroup(model, onChange) {
    const boxes = cat.STATES.map(function (s) {
      return h('label', { class: 'check' },
        h('input', { type: 'checkbox', value: s, checked: model.hide.indexOf(s) >= 0, onchange: function (ev) {
          const rest = model.hide.filter(function (x) { return x !== s; });
          onChange({ hide: ev.target.checked ? rest.concat(s) : rest });
        } }),
        h('span', null, t('game.' + s)));
    });
    return h('fieldset', { class: 'seg-group field' }, h('legend', null, t('overlays.hide')), h('div', { class: 'check-list' }, boxes),
      h('p', { class: 'muted hint' }, t('overlays.hide_hint')));
  }

  // Option controls keyed by param name; sync(overlay) shows only the ones that overlay supports.
  function options(model, onChange) {
    const lim = cat.LIMITS;
    const parts = {
      autohide: numberField('ov-autohide', 'overlays.autohide', 'overlays.autohide_hint',
        { min: String(lim.autohide.min), max: String(lim.autohide.max), step: '1', value: model.autohide },
        function (v) { onChange({ autohide: v }); }),
      scale: numberField('ov-scale', 'overlays.scale', 'overlays.scale_hint',
        { min: String(lim.scale.min), max: String(lim.scale.max), step: '0.1', placeholder: String(cat.DEFAULTS.scale), value: model.scale },
        function (v) { onChange({ scale: v }); }),
      lang: langGroup(model, onChange),
      n: numberField('ov-count', 'overlays.count', 'overlays.count_hint',
        { min: String(lim.n.min), max: String(lim.n.max), step: '1', placeholder: String(cat.DEFAULTS.n), value: model.n },
        function (v) { onChange({ n: v }); }),
      hide: hideGroup(model, onChange),
    };
    const root = h('div', { class: 'overlay-options' }, h('h3', { class: 'card-label' }, t('overlays.options')),
      Object.keys(parts).map(function (k) { return parts[k]; }));
    function sync(overlay) {
      Object.keys(parts).forEach(function (k) { parts[k].hidden = overlay.params.indexOf(k) < 0; });
    }
    return { el: root, sync: sync };
  }

  window.qscOverlayControls = { picker: picker, options: options };
})();
