// Plays a scripted session against a real qsc binary and asserts what it serves.
//
//   node scripts/smoke/drive.mjs --bin <qsc> (--layout <name> | --quaver-dir <dir>) [options]
//
// --layout builds a Steam tree under a temporary HOME (Linux only), so qsc has to find
// the game by itself. --quaver-dir forces the folder (the only mode used on Windows).
// Options: --exe exe|native|both (game executable files), --expect-keys supported|unsupported,
// --port N (default: a free port), --quick (doctor checks only, no server), --expect-version V,
// --windows-default-steam (CI only: builds the game in the default Windows Steam folder, no --quaver-dir).
import { spawn, spawnSync } from 'node:child_process';
import fs from 'node:fs';
import net from 'node:net';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { parseArgs } from 'node:util';
import * as fx from './fixture.mjs';

const { values: a } = parseArgs({ options: {
  bin: { type: 'string' },
  layout: { type: 'string' },
  'quaver-dir': { type: 'string' },
  exe: { type: 'string', default: 'both' },
  'expect-keys': { type: 'string', default: 'unsupported' },
  port: { type: 'string' },
  quick: { type: 'boolean', default: false },
  'expect-version': { type: 'string' },
  'windows-default-steam': { type: 'boolean', default: false },
} });
if (!a.bin) throw new Error('--bin is required');
if (!a.layout && !a['quaver-dir'] && !a['windows-default-steam']) throw new Error('give --layout or --quaver-dir');

const failures = [];
function check(name, ok, detail = '') {
  console.log(`${ok ? 'PASS' : 'FAIL'} ${name}${ok ? '' : ' :: ' + detail}`);
  if (!ok) failures.push(name);
}
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

async function until(fn, ms = 10000) {
  const end = Date.now() + ms;
  for (;;) {
    try {
      const v = await fn();
      if (v) return v;
    } catch { /* retry until the deadline */ }
    if (Date.now() > end) return null;
    await sleep(100);
  }
}

const freePort = () => new Promise((resolve, reject) => {
  const s = net.createServer();
  s.on('error', reject);
  s.listen(0, '127.0.0.1', () => {
    const { port } = s.address();
    s.close(() => resolve(port));
  });
});

const work = fs.mkdtempSync(path.join(os.tmpdir(), 'qsc-smoke-'));
const home = path.join(work, 'home');
fs.mkdirSync(home, { recursive: true });
const port = Number(a.port ?? await freePort());
if ([7878, 5001, 5003].includes(port)) throw new Error(`refusing reserved port ${port}`);
const defaultSteam = 'C:\\Program Files (x86)\\Steam';
let createdDefault = false;
if (a['windows-default-steam']) {
  if (process.platform !== 'win32' || !process.env.CI) throw new Error('--windows-default-steam is for Windows CI runners only');
  if (fs.existsSync(defaultSteam)) throw new Error('refusing to touch an existing Steam folder');
  createdDefault = true;
}
const quaver = a['windows-default-steam']
  ? fx.makeQuaverDir(path.join(defaultSteam, 'steamapps', 'common', 'Quaver'), { exe: a.exe })
  : a['quaver-dir']
    ? fx.makeQuaverDir(path.resolve(a['quaver-dir']), { exe: a.exe })
    : fx.makeSteamTree(home, a.layout, { exe: a.exe });
const forced = a['quaver-dir'] ? ['--quaver-dir', quaver] : [];
const cfg = path.join(work, 'config.json');
const env = {
  ...process.env,
  HOME: home,
  USERPROFILE: home,
  XDG_CONFIG_HOME: path.join(work, 'xdg'),
  APPDATA: path.join(work, 'appdata'),
};
const base = `http://127.0.0.1:${port}`;
console.log(`qsc=${a.bin} port=${port} home=${home} quaver=${quaver} exe=${a.exe}`);

const run = (...args) => spawnSync(a.bin, args, { env, encoding: 'utf8', timeout: 60000 });
const get = async (p) => {
  const r = await fetch(base + p, { signal: AbortSignal.timeout(5000) });
  return { status: r.status, text: await r.text() };
};
const getJSON = async (p) => JSON.parse((await get(p)).text);

function checkDoctor() {
  const ok = run('doctor', '--port', String(port), '--config', cfg, ...forced);
  console.log(ok.stdout + ok.stderr);
  check('doctor exit code 0 on a healthy tree', ok.status === 0, `exit ${ok.status}`);
  check('doctor finds the Quaver folder', /\[ OK \] Quaver folder/.test(ok.stdout), ok.stdout);
  const spellings = [quaver, fs.realpathSync.native(quaver)];
  if (a.layout === 'symlink') spellings.push(path.join(home, '.steam', 'steam', 'steamapps', 'common', 'Quaver'));
  check('doctor names the folder that was set up', spellings.some((p) => ok.stdout.includes(p)), `expected one of ${spellings.join(' | ')} in: ${ok.stdout}`);
  check('doctor finds the log and the database', /\[ OK \] Logs.runtime\.log/.test(ok.stdout) && /\[ OK \] quaver\.db/.test(ok.stdout), ok.stdout);

  const empty = path.join(work, 'empty-quaver');
  fs.mkdirSync(empty);
  const bad = run('doctor', '--port', String(port), '--config', cfg, '--quaver-dir', empty);
  console.log(bad.stdout + bad.stderr);
  check('doctor exit code 1 on a tree without Quaver', bad.status === 1, `exit ${bad.status}`);
  check('doctor reports the missing Quaver folder', /\[FAIL\] Quaver folder/.test(bad.stdout), bad.stdout);
  if (a.layout) {
    const bare = fs.mkdtempSync(path.join(work, 'bare-home-'));
    const none = spawnSync(a.bin, ['doctor', '--port', String(port), '--config', cfg], { env: { ...env, HOME: bare, USERPROFILE: bare }, encoding: 'utf8', timeout: 60000 });
    console.log(none.stdout + none.stderr);
    check('doctor exit code 1 when no Steam folder exists', none.status === 1, `exit ${none.status}`);
    check('doctor explains how to point at the game', /\[FAIL\] Quaver folder/.test(none.stdout) && /--quaver-dir/.test(none.stdout), none.stdout);
  }
  const ver = run('version');
  check('version prints the program name and a version', /quaver-stream-companion \S+/.test(ver.stdout), ver.stdout + ver.stderr);
  if (a['expect-version']) check('version contains the expected tag', ver.stdout.includes(a['expect-version']), ver.stdout);
}

const webDir = path.join(path.dirname(fileURLToPath(import.meta.url)), '..', '..', 'web');

function overlayPaths() {
  const dir = path.join(webDir, 'overlays');
  return fs.readdirSync(dir, { withFileTypes: true })
    .filter((e) => e.isDirectory() && fs.existsSync(path.join(dir, e.name, 'index.html')))
    .map((e) => `/overlays/${e.name}/`)
    .sort();
}

function catalogPaths() {
  const src = fs.readFileSync(path.join(webDir, 'dashboard', 'pages', 'overlay-catalog.js'), 'utf8');
  return [...src.matchAll(/path: '(\/overlays\/[a-z0-9-]+\/)'/g)].map((m) => m[1]).sort();
}

async function playSession(srv) {
  const up = await until(async () => (await get('/api/status')).status === 200);
  check('serve answers /api/status', !!up, srv.output());
  if (!up) throw new Error('server did not start');
  const st = await getJSON('/api/status');
  check('status reports a version', !!st.version, JSON.stringify(st));
  check('status checks are all ok', st.checks.every((c) => c.ok), JSON.stringify(st.checks));
  check('key sampling support matches the platform', (a['expect-keys'] === 'supported') === !!st.keys?.supported, JSON.stringify(st.keys));

  const stream = [];
  let buf = '';
  const ac = new AbortController();
  fetch(base + '/events', { signal: ac.signal }).then(async (r) => {
    for await (const chunk of r.body) {
      buf += Buffer.from(chunk).toString();
      for (let i = buf.indexOf('\n\n'); i >= 0; i = buf.indexOf('\n\n')) {
        stream.push(buf.slice(0, i));
        buf = buf.slice(i + 2);
      }
    }
  }).catch(() => {});

  const game = async () => (await getJSON('/state')).game;
  // The tailer follows only lines written after its first poll: repeat a harmless line until it shows.
  check('state follows the log: select', !!(await until(async () => {
    fx.appendLog(quaver, fx.LINES.select);
    return (await game()) === 'select';
  }, 20000)));

  const md5 = 'smoke0123456789abcdef0123456789ab';
  fx.addMap(quaver, { md5, artist: 'Smoke', title: 'Test', diff: 'Hard' });
  fx.appendLog(quaver, fx.LINES.gameplay);
  check('state follows the log: gameplay', !!(await until(async () => (await game()) === 'gameplay')));
  const id = fx.addScore(quaver, { md5 });
  fx.appendLog(quaver, fx.LINES.score(id), fx.LINES.replay, fx.LINES.results);
  const fin = await until(async () => {
    const s = await getJSON('/state');
    return s.game === 'results' && s.last_score?.score?.id === id ? s : null;
  });
  check('state: results with the score read from the database', !!fin, (await get('/state')).text);
  check('map title resolved from the database', fin?.map?.title === 'Test', JSON.stringify(fin?.map));
  await sleep(500);
  ac.abort();

  const events = stream.map((block) => {
    const line = block.split('\n').find((l) => l.startsWith('data:'));
    try { return JSON.parse(line.slice(5)); } catch { return null; }
  }).filter(Boolean);
  const seen = events.map((e) => (e.type === 'game.state' ? `game.state:${e.data?.state}` : e.type));
  console.log(`events seen: ${seen.join(', ')}`);
  const wanted = ['game.state:select', 'game.state:gameplay', 'map.started', 'game.state:results', 'score.saved'];
  let next = 0;
  for (const type of seen) if (type === wanted[next]) next++;
  check('/events carried select, gameplay, map.started, results, score.saved in order', next === wanted.length, seen.join(', '));
  const saved = events.find((e) => e.type === 'score.saved');
  check('/events score.saved carries the score id', JSON.stringify(saved?.data ?? {}).includes(`"id":${id}`), JSON.stringify(saved));

  const dirs = overlayPaths();
  const catalog = catalogPaths();
  check('the dashboard catalog and the overlay folders list the same overlays', dirs.join() === catalog.join(), `folders: ${dirs} catalog: ${catalog}`);
  const pages = ['/', '/api/overlays', '/api/themes', '/api/history', ...new Set([...dirs, ...catalog])];
  console.log(`checking ${pages.length} pages`);
  for (const p of pages) {
    const r = await get(p);
    check(`GET ${p} answers 200`, r.status === 200, String(r.status));
    if (!p.startsWith('/api/')) {
      for (const m of r.text.matchAll(/(?:src|href)="([^"#?:]+\.(?:js|css))"/g)) {
        const asset = new URL(m[1], base + p).pathname;
        check(`GET ${asset} (from ${p}) answers 200`, (await get(asset)).status === 200, asset);
      }
    }
  }
  const quit = await fetch(base + '/api/quit', { method: 'POST' });
  check('POST /api/quit without the required headers is refused', quit.status >= 400, String(quit.status));
}

function startServer() {
  const child = spawn(a.bin, ['serve', '--port', String(port), '--config', cfg, ...forced], { env, stdio: ['ignore', 'pipe', 'pipe'] });
  let out = '';
  child.stdout.on('data', (d) => (out += d));
  child.stderr.on('data', (d) => (out += d));
  const srv = { child, exited: null, output: () => out };
  child.on('exit', (code, signal) => (srv.exited = { code, signal }));
  return srv;
}

try {
  checkDoctor();
  if (!a.quick) {
    const srv = startServer();
    try {
      await playSession(srv);
      const stop = run('stop', '--port', String(port), '--config', cfg);
      console.log(stop.stdout + stop.stderr);
      check('qsc stop exit code 0', stop.status === 0, `exit ${stop.status}`);
      check('serve exited with code 0 after stop', !!(await until(() => srv.exited, 10000)) && srv.exited.code === 0, JSON.stringify(srv.exited));
      const closed = await until(() => new Promise((res) => {
        const s = net.connect(port, '127.0.0.1');
        s.on('connect', () => { s.destroy(); res(false); });
        s.on('error', () => res(true));
      }), 5000);
      check('the port is free after stop', !!closed);
    } finally {
      if (!srv.exited) srv.child.kill();
      if (failures.length) console.log('--- serve output ---\n' + srv.output());
    }
  }
} catch (e) {
  check('driver ran without crashing', false, String(e?.stack ?? e));
} finally {
  fs.rmSync(work, { recursive: true, force: true });
  if (createdDefault) fs.rmSync(defaultSteam, { recursive: true, force: true });
}
console.log(failures.length ? `\n${failures.length} FAILED: ${failures.join('; ')}` : '\nALL PASSED');
process.exit(failures.length ? 1 : 0);
