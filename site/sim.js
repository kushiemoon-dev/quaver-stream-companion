// Made-up plays for the demo: each play gives the same events qsc sends, with the result and session objects it would build.
(function () {
  const MAPS = [
    { md5: 'demo0', title: 'Night Bus Home', artist: 'Velvet Static', difficulty_name: 'Hard', difficulty: 18.4, keys: 4, creator: 'demo mapper', bpm: 172, length_ms: 131000, regular_notes: 1104, long_notes: 212, mines: 0 },
    { md5: 'demo1', title: 'Glass Orchard', artist: 'Mira Fold', difficulty_name: 'Insane', difficulty: 24.9, keys: 4, creator: 'demo mapper', bpm: 190, length_ms: 118000, regular_notes: 1388, long_notes: 96, mines: 0 },
    { md5: 'demo2', title: 'Paper Satellites', artist: 'Low Tide Club', difficulty_name: 'Normal', difficulty: 9.7, keys: 4, creator: 'demo mapper', bpm: 128, length_ms: 154000, regular_notes: 702, long_notes: 330, mines: 0 },
    { md5: 'demo3', title: 'Tin Roof Rain', artist: 'Juno Park', difficulty_name: 'Hard', difficulty: 16.2, keys: 4, creator: 'demo mapper', bpm: 150, length_ms: 140000, regular_notes: 988, long_notes: 140, mines: 0 },
  ];
  // [map, accuracy, failed, rate, mods]: a run with passes, a fail, new bests and a full combo.
  const SCRIPT = [
    [0, 96.84, false, 1, []], [1, 92.1, false, 1, []], [0, 98.37, false, 1, []], [2, 71.5, true, 1.1, ['Mirror']],
    [1, 99.12, false, 1, []], [3, 88.4, false, 1.2, []], [0, 97.2, false, 1, []], [3, 99.64, false, 1.2, []],
  ];

  function grade(acc, failed) {
    if (failed) return 'F';
    const steps = [[100, 'X'], [99, 'SS'], [95, 'S'], [90, 'A'], [80, 'B'], [70, 'C']];
    const hit = steps.find(function (s) { return acc >= s[0]; });
    return hit ? hit[1] : 'D';
  }

  // judgments spreads the notes of a map so the counts match the accuracy roughly.
  function judgments(map, acc, failed) {
    const total = map.regular_notes + map.long_notes;
    const loss = (100 - acc) / 100;
    const miss = failed ? Math.round(total * 0.08) : acc > 99 ? 0 : Math.round(total * loss * 0.12);
    const great = Math.round(total * loss * 0.9);
    const perf = Math.round(total * (0.12 + loss));
    const good = Math.round(great / 6), okay = Math.round(great / 12);
    return { marv: Math.max(0, total - miss - great - perf - good - okay), perf: perf, great: great, good: good, okay: okay, miss: miss };
  }

  function createSession() {
    const boards = {};
    let n = 0, s = null;
    function reset() {
      s = { started_at: new Date().toISOString(), plays: 0, passes: 0, fails: 0, abandoned: 0, avg_accuracy: 0, best_accuracy: 0, best_rating: 0, full_combos: 0, streak: 0, best_streak: 0 };
      return s;
    }
    reset();

    // next returns the map and the result of the next play, and updates the session.
    function next() {
      const row = SCRIPT[n % SCRIPT.length];
      const jitter = n < SCRIPT.length ? 0 : Math.round((Math.random() - 0.7) * 120) / 100;
      n++;
      const map = MAPS[row[0]], failed = row[2], acc = Math.min(100, Math.max(60, row[1] + jitter));
      const j = judgments(map, acc, failed);
      const board = boards[map.md5] || (boards[map.md5] = []);
      const passes = board.filter(function (p) { return !p.failed; });
      const prevBest = passes.reduce(function (m, p) { return Math.max(m, p.acc); }, 0);
      const sameRate = passes.filter(function (p) { return p.rate === row[3]; }).reduce(function (m, p) { return Math.max(m, p.acc); }, 0);
      const play = { acc: acc, failed: failed, rate: row[3] };
      board.push(play);
      board.sort(function (a, b) { return (a.failed - b.failed) || (b.acc - a.acc); });
      const rating = failed ? 0 : Math.round(map.difficulty * row[3] * Math.pow(acc / 98, 6) * 100) / 100;
      const result = {
        score: { grade: grade(acc, failed), accuracy: acc, max_combo: failed ? 214 : j.miss ? Math.round((j.marv + j.perf) / 3) : j.marv + j.perf + j.great + j.good + j.okay, judgments: j, rate: row[3], mods: row[4], rating: rating },
        map: Object.assign({}, map, { difficulty: Math.round(map.difficulty * row[3] * 100) / 100 }),
        is_pb: !failed && acc > sameRate, local_rank: board.indexOf(play) + 1, local_total: board.length,
        is_local_best: !failed && passes.length > 0 && acc > prevBest, first_play: board.length === 1,
        full_combo: !failed && j.miss === 0, failed: failed, paused: false, autoplay: false,
      };
      s.plays++;
      if (failed) { s.fails++; s.streak = 0; } else {
        s.avg_accuracy = (s.avg_accuracy * s.passes + acc) / (s.passes + 1);
        s.passes++; s.streak++;
        s.best_accuracy = Math.max(s.best_accuracy, acc);
        s.best_rating = Math.max(s.best_rating, rating);
        if (result.full_combo) s.full_combos++;
      }
      s.best_streak = Math.max(s.best_streak, s.streak);
      return { map: map, result: result, session: Object.assign({}, s) };
    }

    return { next: next, reset: function () { return Object.assign({}, reset()); }, current: function () { return Object.assign({}, s); }, first: MAPS[0] };
  }

  // keysFrame gives a lane keys frame for a time in the play, from the same 4K pattern idea as the hero lanes.
  function keysFrame(t, presses) {
    const lane = Math.floor(t / 120) % 4;
    const down = [0, 1, 2, 3].map(function (i) { return i === lane || (Math.floor(t / 480) % 3 === 1 && i === 3 - lane); });
    down.forEach(function (d, i) { if (d) presses[i]++; });
    const kps = 8 + Math.round(4 * Math.sin(t / 700) + 4);
    return { layout: '4k', lanes: 4, scratch: false, down: down, presses: presses.slice(), kps: kps, peak: Math.max(kps, 16) };
  }

  window.qscSim = { createSession: createSession, keysFrame: keysFrame };
})();
