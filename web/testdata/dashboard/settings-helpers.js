// Opens the Settings page on a fake config store that records every save.
'use strict';
const dom = require('../../testsupport/dom.js');
const H = require('./helpers.js');

const config = (over = {}) => ({ quaver_dir: H.QDIR, port: 7878, language: '', watch_quaver_process: true, overlay: { theme: 'neutral-light' }, keys: { enabled: false }, ...over });

// Options: config, effective, overridden, restart (a save asks for a restart), failFirst, firstGate (promise the page load waits for), routes.
// state is live: set putError {status, body}, putGate, getGate or getAfterLoad {status, body} to shape later answers.
async function open(opts = {}) {
  const state = {
    config: opts.config || config(), effective: opts.effective || { quaver_dir: H.QDIR, port: 7878 }, overridden: opts.overridden || [], puts: [],
    putError: null, putGate: null, getGate: null, getAfterLoad: null, ready: false, gets: 0, restart: !!opts.restart, keysStatus: { keys: { supported: true } },
  };
  const get = async () => {
    state.gets++;
    if (opts.failFirst && state.gets === 2) return dom.json({}, 500);
    if (opts.firstGate && state.gets === 2) await opts.firstGate;
    if (state.ready && state.getAfterLoad) return dom.json(state.getAfterLoad.body, state.getAfterLoad.status);
    if (state.ready && state.getGate) await state.getGate;
    return dom.json({
      config: state.config, effective: state.effective, overridden: state.overridden, restart_required: false,
      config_path: H.CFG, themes_dir: ['C:', 'cfg', 'themes'].join(String.fromCharCode(92)), overlays_dir: '', version: '1.2.3',
    });
  };
  const put = async (call) => {
    state.puts.push(JSON.parse(call.body));
    if (state.putGate) await state.putGate;
    if (state.putError) return dom.json(state.putError.body, state.putError.status);
    state.config = state.puts[state.puts.length - 1];
    return dom.json({ config: state.config, restart_required: state.restart });
  };
  const routes = {
    'GET /api/config': get, 'PUT /api/config': put, '/api/status': () => dom.json(state.keysStatus), ...(opts.routes || {}),
  };
  const win = await H.open({ hash: '#/settings', routes });
  state.ready = true;
  if (opts.failFirst) state.getAfterLoad = { status: 500, body: {} };
  return { win, state };
}

module.exports = { open, config, QDIR: H.QDIR };
