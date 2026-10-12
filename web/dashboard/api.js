(function () {
  async function request(method, path, body, signal) {
    const opts = { method: method, signal: signal, headers: { Accept: 'application/json' } };
    if (method !== 'GET') {
      opts.headers['Content-Type'] = 'application/json';
      opts.headers['X-Requested-With'] = 'qsc';
    }
    if (body !== undefined) opts.body = JSON.stringify(body);
    const res = await fetch(path, opts);
    let data = null;
    try {
      data = await res.json();
    } catch (e) {
      data = null;
    }
    if (!res.ok) {
      const err = new Error((data && data.error) || 'HTTP ' + res.status);
      err.status = res.status;
      err.field = (data && data.field) || '';
      throw err;
    }
    return data;
  }

  window.qscApi = {
    get: function (path, signal) { return request('GET', path, undefined, signal); },
    put: function (path, body, signal) { return request('PUT', path, body, signal); },
  };
})();
