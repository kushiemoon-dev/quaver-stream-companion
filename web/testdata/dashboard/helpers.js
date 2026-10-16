// Opens the real dashboard in the simulated browser, with the translation files served from disk.
'use strict';
const fs = require('node:fs');
const path = require('node:path');
const dom = require('../../testsupport/dom.js');

const WEB = path.resolve(__dirname, '..', '..');
const catalog = (lang) => JSON.parse(fs.readFileSync(path.join(WEB, 'dashboard', 'i18n', lang + '.json'), 'utf8'));
const en = catalog('en');

const BS = String.fromCharCode(92);
const QDIR = ['C:', 'Games', 'Quaver'].join(BS);
const CFG = ['C:', 'cfg', 'qsc.json'].join(BS);
const CHECKS = [
  { id: 'quaver_dir', ok: true, detail: QDIR },
  { id: 'log', ok: true, detail: 'readable' },
  { id: 'database', ok: true, detail: 'readable' },
  { id: 'port', ok: true, detail: 'in use by this qsc' },
];
const status = (over = {}) => ({ version: '1.2.3', config_path: CFG, checks: CHECKS, ...over });

function routes(extra = {}) {
  const out = {
    '/api/config': () => dom.json({ config: { language: '' } }),
    '/api/status': () => dom.json(status()),
  };
  for (const lang of ['en', 'fr', 'de']) out['i18n/' + lang + '.json'] = () => dom.json(catalog(lang));
  return { ...out, ...extra };
}

// open loads the dashboard and lets its start-up requests finish. Options: hash, language (browser), routes (merged over the defaults).
async function open(opts = {}) {
  const win = dom.openPage(WEB, 'dashboard/index.html', {
    language: opts.language, storageThrows: opts.storageThrows, url: 'http://127.0.0.1:7878/' + (opts.hash || ''), routes: routes(opts.routes),
  });
  win.dispatchEvent(new dom.Event('DOMContentLoaded', { bubbles: false }));
  await win.flush();
  return win;
}

const stream = (win) => win.eventSources[win.eventSources.length - 1];
const t = (key, params) => en[key].replace(/\{(\w+)\}/g, (m, k) => (k in (params || {}) ? String(params[k]) : m));

module.exports = { QDIR, CFG, open, routes, stream, status, catalog, en, t, CHECKS, WEB, json: dom.json, text: dom.text, deferred: dom.deferred, flush: dom.flush };
