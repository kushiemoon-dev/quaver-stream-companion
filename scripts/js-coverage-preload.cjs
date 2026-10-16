// Gives anonymous vm scripts a filename so V8 coverage attributes them to files (loaded by js-coverage.mjs).
const fs = require('fs'), path = require('path'), vm = require('vm'), url = require('url'), crypto = require('crypto');
const WEB = process.env.QSC_WEB;
const h = (s) => crypto.createHash('sha1').update(s).digest('hex');
const map = new Map();
(function walk(d) {
  for (const e of fs.readdirSync(d, { withFileTypes: true })) {
    const p = path.join(d, e.name);
    if (e.isDirectory()) { if (e.name !== 'testdata') walk(p); continue; }
    if (e.name.endsWith('.js')) map.set(h(fs.readFileSync(p, 'utf8')), url.pathToFileURL(p).href);
  }
})(WEB);
const orig = vm.runInContext;
vm.runInContext = function (code, ctx, opts) {
  const u = typeof code === 'string' && map.get(h(code));
  if (u && (typeof opts !== 'object' || !opts?.filename)) return new vm.Script(code, { filename: u }).runInContext(ctx);
  return orig.call(this, code, ctx, opts);
};
