(function () {
  const qsc = window.qsc;
  const t = qsc.t;
  const h = qsc.h;
  const RELEASES_URL = 'https://github.com/kushiemoon-dev/quaver-stream-companion/releases';
  const COPIED_MS = 1500;

  function pathRow(id, labelKey, value, live) {
    const input = h('input', { id: id, type: 'text', class: 'url-input', readonly: true, value: value, 'aria-label': t(labelKey) });
    const button = h('button', { type: 'button', class: 'btn', 'aria-label': t('settings.copy_aria', { name: t(labelKey) }) }, t('settings.copy'));
    button.addEventListener('click', function () {
      window.qscCopy.copy(value).then(function () {
        button.textContent = t('settings.copied');
        live.textContent = '';
        requestAnimationFrame(function () { live.textContent = t('settings.copied_live', { name: t(labelKey) }); });
        setTimeout(function () { button.textContent = t('settings.copy'); }, COPIED_MS);
      }, function () {
        input.focus();
        input.select();
        live.textContent = t('settings.copy_manual');
      });
    });
    return h('div', { class: 'info-row' }, h('label', { class: 'muted', for: id }, t(labelKey)), h('div', { class: 'url-row' }, input, button));
  }

  // res is the GET /api/config answer.
  function build(res) {
    const live = h('p', { class: 'muted status-line', role: 'status', 'aria-live': 'polite' });
    const rows = [];
    [['set-path-config', 'settings.config_file', res.config_path],
      ['set-path-themes', 'settings.themes_dir', res.themes_dir],
      ['set-path-overlays', 'settings.overlays_dir', res.overlays_dir]].forEach(function (r) {
      if (typeof r[2] === 'string' && r[2]) rows.push(pathRow(r[0], r[1], r[2], live));
    });
    if (res.version) {
      rows.push(h('div', { class: 'info-row' }, h('span', { class: 'muted' }, t('settings.version')),
        h('span', null, res.version, ' ', h('a', { href: RELEASES_URL, target: '_blank', rel: 'noopener noreferrer' }, t('settings.releases')))));
    }
    return h('section', { class: 'card', 'aria-labelledby': 'set-info-title' },
      h('h2', { id: 'set-info-title', class: 'card-label' }, t('settings.about')),
      h('div', { class: 'info-list' }, rows), live);
  }

  window.qscSettingsInfo = { build: build };
})();
