#!/usr/bin/env node
// JavaScript coverage of web/: runs the Go web tests with V8 coverage, then prints a per-file table and a total.
// Usage (repo root): node scripts/js-coverage.mjs [--lines N] [--branches N] [--functions N] [--html]
// Thresholds may also come from QSC_JS_COV_LINES, QSC_JS_COV_BRANCHES, QSC_JS_COV_FUNCTIONS. Default: report only.
// Needs go, node 20 or later, and network access for npx on the first run.
import { spawnSync } from 'node:child_process';
import { mkdtempSync, mkdirSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const C8 = 'c8@10.1.3';
const root = process.cwd();
const preload = join(dirname(fileURLToPath(import.meta.url)), 'js-coverage-preload.cjs');
const args = process.argv.slice(2);

function threshold(name) {
  const i = args.indexOf('--' + name);
  const raw = i >= 0 ? args[i + 1] : process.env['QSC_JS_COV_' + name.toUpperCase()];
  if (raw === undefined || raw === '') return null;
  const n = Number(raw);
  if (!Number.isFinite(n) || n < 0 || n > 100) {
    console.error(`js-coverage: --${name} must be a number from 0 to 100, got "${raw}"`);
    process.exit(2);
  }
  return n;
}

const limits = ['lines', 'branches', 'functions'].map((n) => [n, threshold(n)]).filter(([, v]) => v !== null);
const tmp = mkdtempSync(join(tmpdir(), 'qsc-cov-'));
const v8 = join(tmp, 'v8');
const inline = join(tmp, 'inline');
mkdirSync(v8);

const run = (cmd, cmdArgs, env, capture) =>
  spawnSync(cmd, cmdArgs, { cwd: root, env, encoding: 'utf8', stdio: capture ? 'pipe' : 'inherit', shell: process.platform === 'win32' });

function main() {
  const testEnv = {
    ...process.env,
    QSC_WEB: join(root, 'web'),
    QSC_INLINE: inline,
    NODE_V8_COVERAGE: v8,
    NODE_OPTIONS: `${process.env.NODE_OPTIONS ?? ''} --require ${JSON.stringify(preload)}`.trim(),
  };
  const tests = run('go', ['test', './web', '-count=1'], testEnv, false);
  if (tests.status !== 0) return tests.status ?? 1;

  const report = ['report', '--temp-directory', v8, '--reports-dir', join(root, 'coverage-js'),
    '--src', 'web', '--src', inline, '--all', '--allowExternal',
    '--exclude', '**/testdata/**', '--exclude', '**/scripts/**', '--reporter=text', '--reporter=json-summary'];
  if (args.includes('--html')) report.push('--reporter=html');
  if (limits.length) report.push('--check-coverage', ...limits.flatMap(([n, v]) => [`--${n}`, String(v)]));

  const plain = { ...process.env }; // keep the reporter itself out of the coverage
  const networkError = /ENOTFOUND|ECONNRESET|ETIMEDOUT|EAI_AGAIN|ECONNREFUSED|fetch failed|network/i;
  let res = run('npx', ['--yes', C8, ...report], plain, true);
  if (res.status !== 0 && networkError.test(`${res.stderr}`)) {
    console.error('js-coverage: npx failed on the network, retrying once');
    res = run('npx', ['--yes', C8, ...report], plain, true);
  }
  process.stdout.write(res.stdout ?? '');
  process.stderr.write(res.stderr ?? '');
  return res.status ?? 1;
}

let code = 1;
try {
  code = main();
} finally {
  rmSync(tmp, { recursive: true, force: true });
}
process.exit(code);
