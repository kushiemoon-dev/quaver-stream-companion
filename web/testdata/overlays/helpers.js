// Opens a built-in overlay page in the simulated browser, with the config request answered.
'use strict';
const path = require('node:path');
const dom = require('../../testsupport/dom.js');

const WEB = path.resolve(__dirname, '..', '..');

function open(name, query, opts = {}) {
  const routes = { '/api/config': dom.json({ config: { overlay: {} } }), ...(opts.routes || {}) };
  return dom.openPage(WEB, 'overlays/' + name + '/index.html', {
    ...opts, routes, url: 'http://127.0.0.1:7878/overlays/' + name + '/?' + (query || ''),
  });
}

// stream returns the latest event stream the page opened, after marking it open.
function stream(win) {
  const es = win.eventSources[win.eventSources.length - 1];
  es.open();
  return es;
}

const visible = (el) => !el.classList.contains('hidden');

module.exports = { open, stream, visible, json: dom.json, text: dom.text, deferred: dom.deferred };
