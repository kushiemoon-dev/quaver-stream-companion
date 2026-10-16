// A sequential test runner for the simulated browser scripts: register with test(name, fn), then await run().
'use strict';
const tests = [];
const test = (name, fn) => { tests.push({ name, fn }); };

async function run() {
  let failed = 0;
  for (const t of tests) {
    try { await t.fn(); } catch (e) { failed++; console.log('FAIL ' + t.name + '\n' + (e && e.stack ? e.stack : e)); }
  }
  console.log((tests.length - failed) + '/' + tests.length + ' passed');
  if (failed) process.exitCode = 1;
}

module.exports = { test, run };
