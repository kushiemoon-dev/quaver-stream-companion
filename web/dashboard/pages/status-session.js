(function () {
  const qsc = window.qsc;
  const TIMEOUT_MS = 8000;

  // alive() tells whether the Status page is still mounted.
  function build(alive) {
    const t = qsc.t;
    const h = qsc.h;
    let busy = false;
    const status = h('p', { class: 'muted status-line session-status', role: 'status', 'aria-live': 'polite' });
    const button = h('button', { type: 'button', class: 'btn', onclick: reset }, t('status.session_reset'));
    const card = h('section', { class: 'card', 'aria-labelledby': 'status-session-title' },
      h('h2', { id: 'status-session-title', class: 'card-label' }, t('status.session_title')),
      h('p', { class: 'hint' }, t('status.session_hint')),
      h('div', { class: 'save-row' }, button),
      status);

    // The button stays enabled (aria-disabled only) so keyboard focus is never lost.
    async function reset() {
      if (busy) return;
      busy = true;
      button.setAttribute('aria-disabled', 'true');
      status.textContent = '';
      const ctl = new AbortController();
      const timer = setTimeout(function () { ctl.abort(); }, TIMEOUT_MS);
      let message;
      try {
        await qsc.api.post('/api/session/reset', {}, ctl.signal);
        message = t('status.session_done');
      } catch (e) {
        message = t('status.session_failed', { message: e && e.name === 'AbortError' ? t('settings.timeout') : e.message });
      } finally {
        clearTimeout(timer);
        busy = false;
      }
      if (!alive()) return;
      button.removeAttribute('aria-disabled');
      status.textContent = message;
    }

    return card;
  }

  window.qscStatusSession = { build: build };
})();
