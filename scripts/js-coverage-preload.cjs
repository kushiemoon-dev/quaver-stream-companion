// Gives anonymous vm scripts a filename so V8 coverage attributes them to files (loaded by js-coverage.mjs).
const fs = require('fs'), path = require('path'), vm = require('vm'), url = require('url'), crypto = require('crypto');
const WEB = process.env.QSC_WEB, INLINE = process.env.QSC_INLINE;
const h = (s) => crypto.createHash('sha1').update(s).digest('hex');
const map = new Map();
(function walk(d) {
  for (const e of fs.readdirSync(d, { withFileTypes: true })) {
    const p = path.join(d, e.name);
    if (e.isDirectory()) { if (e.name !== 'testdata') walk(p); continue; }
    if (e.name.endsWith('.js')) map.set(h(fs.readFileSync(p, 'utf8')), url.pathToFileURL(p).href);
    else if (e.name.endsWith('.html')) {
      let i = 0; const src = fs.readFileSync(p, 'utf8');
      for (const m of src.matchAll(/<script(?![^>]*\bsrc=)[^>]*>([\s\S]*?)<\/script>/g)) {
        const f = path.join(INLINE, path.relative(WEB, p).split(/[^A-Za-z0-9._-]+/).join('__') + '.' + (i++) + '.js');
        fs.mkdirSync(INLINE, { recursive: true }); fs.writeFileSync(f, m[1]);
        map.set(h(m[1]), url.pathToFileURL(f).href);
      }
    }
  }
})(WEB);
const orig = vm.runInContext;
vm.runInContext = function (code, ctx, opts) {
  const u = typeof code === 'string' && map.get(h(code));
  if (u && (typeof opts !== 'object' || !opts?.filename)) return new vm.Script(code, { filename: u }).runInContext(ctx);
  return orig.call(this, code, ctx, opts);
};
