(function () {
  const MESSAGE_KEYS = {
    'must be between 1 and 65535': 'error.port_range',
    'must be a folder on a local drive, not a network path': 'error.dir_unc',
    'must be a folder containing Quaver or Quaver.exe': 'error.dir_not_quaver',
    'must be one of en, fr, de': 'error.language',
    'unknown theme': 'error.theme',
    'must be 3, 4, 6 or 8 hexadecimal digits without #': 'error.accent',
    'Quaver is writing the database, try again': 'error.busy',
    'no judgment timeline for this score': 'error.no_timeline',
    'request body too large': 'error.too_large',
    'must not list the same key twice': 'error.keys_dup',
    'must differ from the lane keys of the layout in use': 'error.keys_scratch_clash',
  };
  const MESSAGE_PATTERNS = [
    [/^must list exactly (\d+) keys$/, 'error.keys_count'],
    [/^entry (\d+) is not a supported key name$/, 'error.keys_entry'],
  ];

  function say(key, params) {
    return window.qscI18n.t(key, params);
  }

  // The companion answers in English: known messages get a translation, the others a generic one.
  function describe(status, raw) {
    if (typeof raw === 'string') {
      if (Object.prototype.hasOwnProperty.call(MESSAGE_KEYS, raw)) return say(MESSAGE_KEYS[raw]);
      for (let i = 0; i < MESSAGE_PATTERNS.length; i++) {
        const m = MESSAGE_PATTERNS[i][0].exec(raw);
        if (m) return say(MESSAGE_PATTERNS[i][1], { n: m[1] });
      }
    }
    return say('error.server', { status: status });
  }

  function report(ok) {
    if (window.qscOffline) window.qscOffline.report(ok);
  }

  async function request(method, path, body, signal) {
    const opts = { method: method, signal: signal, headers: { Accept: 'application/json' } };
    if (method !== 'GET') {
      opts.headers['Content-Type'] = 'application/json';
      opts.headers['X-Requested-With'] = 'qsc';
    }
    if (body !== undefined) opts.body = JSON.stringify(body);
    let res;
    try {
      res = await fetch(path, opts);
    } catch (e) {
      if (e && e.name === 'AbortError') throw e;
      report(false);
      const err = new Error(say('error.network'));
      err.network = true;
      throw err;
    }
    report(true);
    let data = null;
    try {
      data = await res.json();
    } catch (e) {
      data = null;
    }
    if (!res.ok) {
      const err = new Error(describe(res.status, data && data.error));
      err.status = res.status;
      err.field = (data && data.field) || '';
      throw err;
    }
    return data;
  }

  window.qscApi = {
    get: function (path, signal) { return request('GET', path, undefined, signal); },
    put: function (path, body, signal) { return request('PUT', path, body, signal); },
    post: function (path, body, signal) { return request('POST', path, body, signal); },
  };
})();
