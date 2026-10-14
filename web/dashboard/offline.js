(function () {
  const BEAT_MS = 5000;
  const FRESH_MS = 4500;
  const BEAT_TIMEOUT_MS = 4000;
  const listeners = [];
  let offline = false;
  let lastAt = 0;
  let timer = null;

  function paint() {
    const el = document.getElementById('offline');
    if (el) el.textContent = offline ? window.qscI18n.t('offline.banner') : '';
  }

  // Any answer from the companion, even an error status, counts as online; only a failed request counts as offline.
  function report(ok) {
    lastAt = Date.now();
    if (offline === !ok) return;
    offline = !ok;
    paint();
    listeners.slice().forEach(function (fn) { fn(offline); });
  }

  function onChange(fn) {
    listeners.push(fn);
    return function () {
      const i = listeners.indexOf(fn);
      if (i >= 0) listeners.splice(i, 1);
    };
  }

  // Pages that poll report on their own; this only covers the quiet ones.
  function beat() {
    if (document.hidden || Date.now() - lastAt < FRESH_MS) return;
    const ctl = new AbortController();
    const timeout = setTimeout(function () { ctl.abort(); }, BEAT_TIMEOUT_MS);
    window.qscApi.get('/api/status', ctl.signal).catch(function (e) {
      if (e && e.name === 'AbortError') report(false);
    }).finally(function () { clearTimeout(timeout); });
  }

  function start() {
    paint();
    if (timer === null) timer = setInterval(beat, BEAT_MS);
  }

  window.qscOffline = { report: report, onChange: onChange, isOffline: function () { return offline; }, repaint: paint, start: start };
})();
