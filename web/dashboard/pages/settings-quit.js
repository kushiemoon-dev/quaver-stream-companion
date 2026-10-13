(function () {
  const qsc = window.qsc;

  // ctx.call(fn) runs a request with a timeout and the page lifetime, ctx.alive() tells whether the page is still mounted.
  function build(ctx) {
    const t = qsc.t;
    const h = qsc.h;
    let quitting = false;

    const status = h('p', { class: 'save-status', role: 'status', 'aria-live': 'polite' });
    const error = h('p', { class: 'field-error', role: 'alert' });
    const open = h('button', { type: 'button', class: 'btn', onclick: ask }, t('settings.quit.button'));
    const cancel = h('button', { type: 'button', class: 'btn', onclick: dismiss }, t('settings.quit.cancel'));
    const now = h('button', { type: 'button', class: 'btn btn-primary', onclick: confirmQuit }, t('settings.quit.now'));
    const confirmBox = h('div', { id: 'set-quit-confirm', class: 'save-row', role: 'group', 'aria-labelledby': 'set-quit-question', hidden: true, onkeydown: onKey },
      h('p', { id: 'set-quit-question' }, t('settings.quit.confirm')), cancel, now);
    const title = h('h2', { id: 'set-quit-title', class: 'card-label', tabindex: '-1' }, t('settings.quit.title'));
    const help = h('p', { class: 'hint' }, t('settings.quit.help'));
    const row = h('div', { class: 'save-row' }, open);
    const card = h('section', { class: 'card', 'aria-labelledby': 'set-quit-title' },
      title,
      help,
      row,
      confirmBox, error, status);

    function ask() {
      error.textContent = '';
      confirmBox.hidden = false;
      open.hidden = true;
      cancel.focus();
    }

    function dismiss() {
      confirmBox.hidden = true;
      open.hidden = false;
      error.textContent = '';
      open.focus();
    }

    function stopped() {
      help.hidden = true;
      row.hidden = true;
      confirmBox.hidden = true;
      error.textContent = '';
      status.className = 'save-status ok';
      status.textContent = t('settings.quit.stopped');
      title.focus();
    }

    function onKey(ev) {
      if (ev.key === 'Escape' && !quitting) dismiss();
    }

    async function confirmQuit() {
      if (quitting) return;
      quitting = true;
      cancel.disabled = true;
      now.disabled = true;
      now.setAttribute('aria-busy', 'true');
      error.textContent = '';
      status.textContent = t('settings.quit.stopping');
      try {
        await ctx.call(function (s) { return qsc.api.post('/api/quit', {}, s); });
        if (ctx.alive()) stopped();
        return;
      } catch (e) {
        if (!ctx.alive()) return;
        // The server closing the connection while it stops is the expected outcome, not a failure.
        if (e.status === undefined && e.timedOut === undefined) {
          stopped();
          return;
        }
        status.textContent = '';
        error.textContent = t('settings.quit.failed', { message: e.message });
      }
      quitting = false;
      cancel.disabled = false;
      now.disabled = false;
      now.removeAttribute('aria-busy');
      now.focus();
    }

    return card;
  }

  window.qscSettingsQuit = { build: build };
})();
