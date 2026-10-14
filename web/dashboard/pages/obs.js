(function () {
  const qsc = window.qsc;
  const t = qsc.t;
  const h = qsc.h;
  const cat = window.qscOverlayCatalog;
  const CHECK_TIMEOUT_MS = 8000;
  const COPIED_MS = 2000;
  const GUIDE_URL = 'https://github.com/kushiemoon-dev/quaver-stream-companion/blob/main/docs/OVERLAYS.md';

  let ui = null;
  let mountId = 0;
  let copiedTimer = null;

  const STEP5 = ['obs.step5.item1', 'obs.step5.item2', 'obs.step5.item3', 'obs.step5.item4'];
  const TROUBLES = [
    ['obs.trouble1.title', 'obs.trouble1.body'],
    ['obs.trouble2.title', 'obs.trouble2.body'],
    ['obs.trouble3.title', 'obs.trouble3.body'],
    ['obs.trouble4.title', 'obs.trouble4.body'],
  ];

  function step(titleKey, body) {
    return h('li', { class: 'obs-step' }, h('h2', null, t(titleKey)), body);
  }

  function copyLink(input, button) {
    const manual = function () {
      input.focus();
      input.select();
    };
    if (!navigator.clipboard || !navigator.clipboard.writeText) {
      manual();
      return;
    }
    navigator.clipboard.writeText(input.value).then(function () {
      if (!ui) return;
      button.textContent = t('overlays.copied');
      ui.copyStatus.textContent = t('overlays.copied_live');
      clearTimeout(copiedTimer);
      copiedTimer = setTimeout(function () {
        button.textContent = t('overlays.copy');
        if (ui) ui.copyStatus.textContent = '';
      }, COPIED_MS);
    }, manual);
  }

  function row(overlay) {
    const name = t(overlay.nameKey);
    const input = h('input', { type: 'text', class: 'url-input', readonly: true, value: location.origin + overlay.path,
      'aria-label': t('overlays.url') + ': ' + name });
    const button = h('button', { type: 'button', class: 'btn', 'aria-label': t('overlays.copy_aria', { name: name }) }, t('overlays.copy'));
    button.addEventListener('click', function () { copyLink(input, button); });
    return h('tr', null,
      h('th', { scope: 'row' }, name),
      h('td', { class: 'obs-size' }, cat.sizeText(overlay)),
      h('td', null, h('div', { class: 'url-row' }, input, button)));
  }

  function sizeTable() {
    return h('div', { class: 'table-wrap', role: 'region', tabindex: '0', 'aria-label': t('obs.table_label') }, h('table', { class: 'obs-table' },
      h('thead', null, h('tr', null,
        h('th', { scope: 'col' }, t('obs.col.overlay')),
        h('th', { scope: 'col' }, t('obs.col.size')),
        h('th', { scope: 'col' }, t('obs.col.link')))),
      h('tbody', null, cat.OVERLAYS.map(row))));
  }

  async function runCheck() {
    const id = ++mountId;
    ui.check.disabled = true;
    ui.result.replaceChildren(t('obs.check_running'));
    const ctl = new AbortController();
    const timeout = setTimeout(function () { ctl.abort(); }, CHECK_TIMEOUT_MS);
    let checks = null;
    try {
      const s = await qsc.api.get('/api/status', ctl.signal);
      checks = Array.isArray(s.checks) ? s.checks : [];
    } catch (e) {
      checks = null;
    } finally {
      clearTimeout(timeout);
    }
    if (id !== mountId || !ui) return;
    ui.check.disabled = false;
    if (!checks) {
      ui.result.replaceChildren(h('span', { class: 'obs-bad' }, t('obs.check_failed')));
      return;
    }
    const allOk = checks.length > 0 && checks.every(function (c) { return c.ok; });
    ui.result.replaceChildren(
      h('ul', { class: 'obs-checks' }, checks.map(function (c) {
        return h('li', { class: c.ok ? 'obs-ok' : 'obs-bad' }, t('check.' + c.id), ': ', t(c.ok ? 'status.ok' : 'status.problem'));
      })),
      h('p', null, t(allOk ? 'obs.check_ok' : 'obs.check_bad'), allOk ? null : [' ', h('a', { href: '#/status' }, t('obs.status_link'))]));
  }

  async function loadFolder() {
    const id = mountId;
    const ctl = new AbortController();
    const timeout = setTimeout(function () { ctl.abort(); }, CHECK_TIMEOUT_MS);
    let dir = '';
    try {
      const body = await qsc.api.get('/api/overlays', ctl.signal);
      dir = body && typeof body.dir === 'string' ? body.dir : '';
    } catch (e) {
      dir = '';
    } finally {
      clearTimeout(timeout);
    }
    if (id !== mountId || !ui) return;
    if (dir) ui.folder.replaceChildren(t('obs.custom.folder'), ' ', h('code', null, qsc.pathNodes(dir)));
    else ui.folder.replaceChildren(t('obs.custom.folder_unavailable'));
  }

  function customCard() {
    return h('section', { class: 'card obs-card', 'aria-label': t('obs.custom.title') },
      h('h2', null, t('obs.custom.title')),
      h('p', { class: 'muted' }, t('obs.custom.body')),
      ui.folder,
      h('p', null, h('a', { class: 'btn', href: GUIDE_URL, target: '_blank', rel: 'noopener noreferrer' }, t('obs.custom.link'))));
  }

  function troubleshooting() {
    return TROUBLES.map(function (k) {
      return h('div', { class: 'obs-trouble' }, h('h3', null, t(k[0])), h('p', { class: 'muted' }, t(k[1])));
    });
  }

  function mount(root) {
    mountId++;
    ui = {
      copyStatus: h('p', { class: 'muted status-line', role: 'status', 'aria-live': 'polite' }),
      check: h('button', { type: 'button', class: 'btn', onclick: runCheck }, t('obs.check')),
      result: h('div', { class: 'obs-result', role: 'status', 'aria-live': 'polite' }),
      folder: h('p', { class: 'muted', role: 'status', 'aria-live': 'polite' }),
    };
    root.appendChild(h('header', { class: 'page-head' },
      h('div', null, h('h1', null, t('nav.obs')), h('p', { class: 'subtitle' }, t('obs.subtitle')))));
    root.appendChild(h('section', { class: 'card obs-card', 'aria-label': t('obs.steps') },
      h('ol', { class: 'obs-steps' },
        step('obs.step1.title', h('p', { class: 'muted' }, t('obs.step1.body'))),
        step('obs.step2.title', h('p', { class: 'muted' }, t('obs.step2.body'))),
        step('obs.step3.title', [h('p', { class: 'muted' }, t('obs.step3.body')),
          h('p', null, h('a', { class: 'btn', href: '#/overlays' }, t('obs.step3.button')))]),
        step('obs.step4.title', [h('p', { class: 'muted' }, t('obs.step4.body')), sizeTable(), ui.copyStatus]),
        step('obs.step5.title', h('ul', { class: 'muted' }, STEP5.map(function (k) { return h('li', null, t(k)); }))),
        step('obs.step6.title', h('p', { class: 'muted' }, t('obs.step6.body'))),
        step('obs.step7.title', [troubleshooting(), h('p', null, ui.check), ui.result]))));
    root.appendChild(customCard());
    loadFolder();
  }

  function unmount() {
    mountId++;
    clearTimeout(copiedTimer);
    ui = null;
  }

  qsc.registerPage('obs', { title: 'nav.obs', mount: mount, unmount: unmount });
})();
