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

  const DEMO_KEY = 'qsc.overlays.demo';

  // The remembered choice, or null when nothing is stored or the storage is blocked.
  function storedDemo() {
    try {
      const v = window.localStorage.getItem(DEMO_KEY);
      return v === '1' ? true : v === '0' ? false : null;
    } catch (e) {
      return null;
    }
  }

  function storeDemo(on) {
    try { window.localStorage.setItem(DEMO_KEY, on ? '1' : '0'); } catch (e) { /* the choice just is not remembered */ }
  }

  // Without a remembered choice the preview shows demo data unless Quaver is found and a map is known.
  function demoDefault(checks, live) {
    const folder = (Array.isArray(checks) ? checks : []).filter(function (c) { return c && c.id === 'quaver_dir'; })[0];
    return !(folder && folder.ok === true && live && live.map);
  }

  // The remembered choice, else the default from the Quaver check and the live state (a failed request counts as nothing known).
  async function initialDemo(call) {
    const stored = storedDemo();
    if (stored !== null) return stored;
    const got = await Promise.all([
      call(function (s) { return qsc.api.get('/api/status', s); }).catch(function () { return {}; }),
      call(function (s) { return qsc.api.get('/state', s); }).catch(function () { return null; }),
    ]);
    return demoDefault(got[0].checks, got[1]);
  }

  window.qscOverlayUtil = { pickerValue: pickerValue, makeCall: makeCall, describe: describe, storeDemo: storeDemo, initialDemo: initialDemo };
})();
