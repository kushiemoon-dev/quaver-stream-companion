// Builds synthetic Quaver installs and Steam folder trees for the smoke tests.
// Uses only Node built-ins; node:sqlite needs Node 22.13 or newer.
// The tables mirror internal/testutil/quaverdir.go.
import { DatabaseSync } from 'node:sqlite';
import fs from 'node:fs';
import path from 'node:path';

const MAP_TABLE = 'CREATE TABLE Map (Id integer primary key autoincrement not null, Md5Checksum varchar, Artist varchar, Title varchar, DifficultyName varchar, LastTimePlayed integer, Mode integer, Difficulty10X float, Difficulty09X float, Difficulty115X float, HasScratchKey integer)';
const SCORE_TABLE = 'CREATE TABLE Score (Id integer primary key autoincrement not null, MapMd5 varchar, TotalScore integer, Grade integer, Accuracy float, MaxCombo integer, CountMarv integer, CountPerf integer, CountGreat integer, CountGood integer, CountOkay integer, CountMiss integer, Mods integer, DateTime varchar, PerformanceRating float, JudgementBreakdown varchar)';

export const LINES = {
  select: '[1:00:00] - RUNTIME - IMPORTANT: Screen has been switched to type: `Select`',
  gameplay: '[1:05:29] - RUNTIME - IMPORTANT: Screen has been switched to type: `Gameplay`',
  results: '[1:06:07] - RUNTIME - IMPORTANT: Screen has been switched to type: `Results`',
  score: (id) => `[1:06:06] - RUNTIME - IMPORTANT: Successfully saved score to database with id: ${id}`,
  replay: String.raw`[1:06:06] - RUNTIME - IMPORTANT: Successfully saved replay to path: Z:\home\deck\.local\share\Steam\steamapps\common\Quaver\Data\r\1.qr`,
};

const EXECUTABLES = { exe: ['Quaver.exe'], native: ['Quaver'], both: ['Quaver', 'Quaver.exe'] };

/** Creates the game executable(s), Logs/runtime.log and quaver.db in dir. exe is one of exe, native, both. */
export function makeQuaverDir(dir, { exe = 'both' } = {}) {
  const names = EXECUTABLES[exe];
  if (!names) throw new Error(`unknown exe variant ${exe}; use ${Object.keys(EXECUTABLES).join(', ')}`);
  fs.mkdirSync(path.join(dir, 'Logs'), { recursive: true });
  for (const name of names) fs.writeFileSync(path.join(dir, name), '');
  fs.writeFileSync(logPath(dir), '');
  const db = new DatabaseSync(path.join(dir, 'quaver.db'));
  db.exec(MAP_TABLE);
  db.exec(SCORE_TABLE);
  db.close();
  return dir;
}

export function logPath(dir) {
  return path.join(dir, 'Logs', 'runtime.log');
}

export function appendLog(dir, ...lines) {
  fs.appendFileSync(logPath(dir), lines.map((l) => l + '\n').join(''));
}

function withDb(dir, fn) {
  const db = new DatabaseSync(path.join(dir, 'quaver.db'));
  db.exec('PRAGMA busy_timeout=10000');
  try {
    return fn(db);
  } finally {
    db.close();
  }
}

export function addMap(dir, m) {
  withDb(dir, (db) => db.prepare('INSERT INTO Map (Md5Checksum, Artist, Title, DifficultyName, LastTimePlayed, Mode, Difficulty10X, HasScratchKey) VALUES (?,?,?,?,?,?,?,0)')
    .run(m.md5, m.artist, m.title, m.diff, m.lastPlayedMs ?? Date.now(), m.mode ?? 1, m.difficulty ?? 12.5));
}

/** Inserts a score row and returns its id. */
export function addScore(dir, s) {
  return withDb(dir, (db) => Number(db.prepare("INSERT INTO Score (MapMd5, TotalScore, Grade, Accuracy, MaxCombo, CountMarv, CountPerf, CountGreat, CountGood, CountOkay, CountMiss, Mods, DateTime, PerformanceRating, JudgementBreakdown) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,'2026-10-12 13:06:06',?,'')")
    .run(s.md5, s.total ?? 950000, s.grade ?? 1, s.acc ?? 97.5, s.combo ?? 500, s.marv ?? 400, s.perf ?? 80, s.great ?? 15, s.good ?? 3, s.okay ?? 1, s.miss ?? 1, s.mods ?? 0, s.rating ?? 20.1).lastInsertRowid));
}

const ROOTS = {
  'dot-steam': (home) => path.join(home, '.steam', 'steam'),
  'local-share': (home) => path.join(home, '.local', 'share', 'Steam'),
  flatpak: (home) => path.join(home, '.var', 'app', 'com.valvesoftware.Steam', '.local', 'share', 'Steam'),
  snap: (home) => path.join(home, 'snap', 'steam', 'common', '.local', 'share', 'Steam'),
};

export const LAYOUTS = [...Object.keys(ROOTS), 'symlink', 'library'];

const gameDir = (root) => path.join(root, 'steamapps', 'common', 'Quaver');

/** Builds the Steam layout under home and returns the Quaver folder. Linux layouts only. */
export function makeSteamTree(home, layout, opts) {
  if (layout === 'symlink') {
    const real = path.join(home, 'real-steam');
    const dir = makeQuaverDir(gameDir(real), opts);
    fs.mkdirSync(path.join(home, '.steam'), { recursive: true });
    fs.symlinkSync(real, path.join(home, '.steam', 'steam'), 'dir');
    return dir;
  }
  if (layout === 'library') {
    const root = ROOTS['local-share'](home);
    const lib = path.join(home, 'games', 'SteamLibrary');
    fs.mkdirSync(path.join(root, 'steamapps'), { recursive: true });
    fs.writeFileSync(path.join(root, 'steamapps', 'libraryfolders.vdf'),
      `"libraryfolders"\n{\n\t"0"\n\t{\n\t\t"path"\t\t"${root}"\n\t}\n\t"1"\n\t{\n\t\t"path"\t\t"${lib}"\n\t}\n}\n`);
    return makeQuaverDir(gameDir(lib), opts);
  }
  if (!ROOTS[layout]) throw new Error(`unknown layout ${layout}; use ${LAYOUTS.join(', ')}`);
  return makeQuaverDir(gameDir(ROOTS[layout](home)), opts);
}
