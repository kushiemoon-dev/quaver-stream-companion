// Falling notes on four lanes, drawn on a canvas. Hover or tap a lane, or press D F J K, to hit its receptor.
(function () {
  const canvas = document.getElementById('lanes');
  if (!canvas || !canvas.getContext) return;
  const ctx = canvas.getContext('2d');
  const KEYS = ['d', 'f', 'j', 'k'];
  const STEP_MS = 200;
  const TRAVEL_MS = 1400;
  const WINDOWS = [[25, 'Marvelous'], [55, 'Perfect'], [90, 'Great'], [130, 'Good']];
  const motion = window.matchMedia('(prefers-reduced-motion: reduce)');
  const css = getComputedStyle(document.documentElement);
  const color = function (name) { return css.getPropertyValue(name).trim(); };
  const palette = { lane: color('--lane'), outer: color('--note-a'), inner: color('--note-b'), text: color('--text'), gold: color('--marv') };

  let width = 0, height = 0, notes = [], flashes = [0, 0, 0, 0], hover = -1;
  let judgment = { text: '', at: -1e9 }, nextStep = 0, step = 0, frame = 0, clock = 0, last = 0;

  function resize() {
    const dpr = Math.min(window.devicePixelRatio || 1, 2);
    width = canvas.clientWidth; height = canvas.clientHeight;
    canvas.width = Math.round(width * dpr); canvas.height = Math.round(height * dpr);
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    if (!frame) draw();
  }

  // pattern makes a short stair, a chord or a jack in turn, like a 4K chart.
  function pattern(i) {
    const kind = Math.floor(i / 8) % 3;
    if (kind === 0) return [i % 4];
    if (kind === 1) return i % 2 ? [0, 3] : [1 + (i % 4 === 0 ? 0 : 1)];
    return [i % 4 < 2 ? 1 : 2];
  }

  function spawn(now) {
    while (nextStep <= now + TRAVEL_MS) {
      pattern(step).forEach(function (lane) { notes.push({ lane: lane, at: nextStep, hit: false }); });
      step++; nextStep += STEP_MS;
    }
  }

  function receptorY() { return height - 56; }
  function laneX(lane) { return (width / 4) * lane; }

  function judge(text, lane, now) {
    judgment = { text: text, at: now };
    flashes[lane] = now;
  }

  function press(lane) {
    const now = clock;
    let best = null;
    notes.forEach(function (n) { if (!n.hit && n.lane === lane && (!best || Math.abs(n.at - now) < Math.abs(best.at - now))) best = n; });
    const off = best ? Math.abs(best.at - now) : Infinity;
    const win = WINDOWS.find(function (w) { return off <= w[0]; });
    if (win) best.hit = true;
    judge(win ? win[1] : 'Miss', lane, now);
    if (!frame) draw();
  }

  function note(x, y, w, lane, alpha) {
    ctx.globalAlpha = alpha;
    ctx.fillStyle = lane === 0 || lane === 3 ? palette.outer : palette.inner;
    ctx.beginPath();
    ctx.roundRect(x + 8, y - 9, w - 16, 18, 9);
    ctx.fill();
    ctx.globalAlpha = 1;
  }

  function draw() {
    const now = clock, w = width / 4, ry = receptorY();
    ctx.clearRect(0, 0, width, height);
    for (let i = 0; i < 4; i++) {
      ctx.fillStyle = i === hover ? 'rgba(255,255,255,0.06)' : 'rgba(255,255,255,0.015)';
      ctx.fillRect(laneX(i), 0, w, height);
      ctx.fillStyle = palette.lane;
      ctx.fillRect(laneX(i), 0, 1, height);
    }
    notes.forEach(function (n) {
      if (n.hit) return;
      const y = ry - ((n.at - now) / TRAVEL_MS) * ry;
      if (y > -20 && y < height + 20) note(laneX(n.lane), y, w, n.lane, n.lane === hover ? 1 : 0.85);
    });
    for (let i = 0; i < 4; i++) {
      const glow = Math.max(0, 1 - (now - flashes[i]) / 220);
      ctx.strokeStyle = palette.text;
      ctx.globalAlpha = 0.35 + 0.65 * glow;
      ctx.lineWidth = 2;
      ctx.beginPath();
      ctx.roundRect(laneX(i) + 8, ry - 9, w - 16, 18, 9);
      ctx.stroke();
      if (glow > 0) note(laneX(i), ry, w, i, glow * 0.9);
      ctx.globalAlpha = 1;
    }
    const age = now - judgment.at;
    if (age < 500) {
      ctx.globalAlpha = 1 - age / 500;
      ctx.fillStyle = judgment.text === 'Marvelous' ? palette.gold : palette.text;
      ctx.font = '600 22px "Chakra Petch", sans-serif';
      ctx.textAlign = 'center';
      ctx.fillText(judgment.text, width / 2, ry - 120 - age / 25);
      ctx.globalAlpha = 1;
    }
  }

  function tick(t) {
    clock += Math.min(t - last, 50);
    last = t;
    spawn(clock);
    notes = notes.filter(function (n) {
      if (!n.hit && n.at <= clock) { n.hit = true; judge(n.lane === hover ? 'Perfect' : 'Marvelous', n.lane, clock); }
      return n.at > clock - 300;
    });
    draw();
    frame = requestAnimationFrame(tick);
  }

  function start() {
    if (frame || motion.matches || document.hidden) return;
    last = performance.now();
    frame = requestAnimationFrame(tick);
  }
  function stop() {
    if (frame) cancelAnimationFrame(frame);
    frame = 0;
    draw();
  }

  canvas.addEventListener('pointermove', function (e) { hover = Math.floor((e.offsetX / width) * 4); if (!frame) draw(); });
  canvas.addEventListener('pointerleave', function () { hover = -1; if (!frame) draw(); });
  canvas.addEventListener('pointerdown', function (e) { press(Math.min(3, Math.floor((e.offsetX / width) * 4))); });
  canvas.addEventListener('keydown', function (e) {
    const lane = KEYS.indexOf(e.key.toLowerCase());
    if (lane < 0 || e.repeat) return;
    e.preventDefault();
    press(lane);
  });
  document.addEventListener('visibilitychange', function () { if (document.hidden) stop(); else start(); });
  motion.addEventListener('change', function () { if (motion.matches) stop(); else start(); });
  new ResizeObserver(resize).observe(canvas);

  spawn(0);
  resize();
  start();
})();
