(function () {
  const qsc = window.qsc;
  const t = qsc.t;
  const url = window.qscOverlayUrl;
  const PICKER_START = '#a855f7';
  const TIMEOUT_MS = 8000;

  function pickerValue(hex) {
    if (!url.isValidAccent(hex)) return PICKER_START;
    const rgb = hex.length <= 4
      ? hex.slice(0, 3).split('').map(function (c) { return c + c; }).join('')
      : hex.slice(0, 6);
    return '#' + rgb;
  }

  // Wraps a request with the page lifetime and a timeout; getLifetime returns the controller of the mounted page.
  function makeCall(getLifetime) {
    return function (fn) {
      const life = getLifetime();
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
    };
  }

  function describe(e) {
    return e.field ? e.field + ': ' + e.message : e.message;
  }

  window.qscOverlayUtil = { pickerValue: pickerValue, makeCall: makeCall, describe: describe };
})();
