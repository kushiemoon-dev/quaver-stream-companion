// The overlay demo: real overlay pages in frames, a theme and language picker, and a simulator that sends them made-up plays.
(function () {
  const frames = Array.from(document.querySelectorAll('[data-overlay]'));
  const themePick = document.getElementById('theme');
  const langPick = document.getElementById('lang');
  const playBtn = document.getElementById('play');
  const autoBtn = document.getElementById('auto');
  const resetBtn = document.getElementById('reset');
  const status = document.getElementById('sim-status');
  if (!frames.length || !window.qscSim) return;

  const PLAY_MS = 3600, MAP_AT_MS = 500, KEYS_EVERY_MS = 100, AUTO_GAP_MS = 2600;
  const session = window.qscSim.createSession();
  const state = { game: 'menu', map: null, last_score: null, keys: null, session: session.current() };
  let timers = [], playing = false, auto = false, count = 0;

  function frameSrc(fig) {
    const p = new URLSearchParams({ theme: themePick.value, lang: langPick.value });
    if (fig.hasAttribute('data-live')) p.set('feed', 'parent'); else p.set('demo', '1');
    new URLSearchParams(fig.getAttribute('data-params') || '').forEach(function (v, k) { p.set(k, v); });
    return '/overlays/' + fig.getAttribute('data-overlay') + '/?' + p.toString();
  }

  function send(fig, event, data) {
    const win = fig.querySelector('iframe').contentWindow;
    if (win) win.postMessage({ type: 'qsc-event', event: event, data: data }, location.origin);
  }

  function broadcast(event, data) {
    frames.forEach(function (fig) { if (fig.hasAttribute('data-live')) send(fig, event, data); });
  }

  function later(ms, fn) { timers.push(setTimeout(fn, ms)); }

  function describe(r) {
    const tags = [];
    if (r.failed) tags.push('failed');
    if (r.is_local_best) tags.push('new best on the local board');
    else if (r.is_pb) tags.push('new personal best');
    if (r.full_combo) tags.push('full combo');
    return 'Play ' + count + ': ' + r.map.title + ', grade ' + r.score.grade + ', ' + r.score.accuracy.toFixed(2) + '%' + (tags.length ? ', ' + tags.join(', ') : '') + '.';
  }

  function setGame(name) {
    state.game = name;
    broadcast('game.state', { state: name, screen: name });
  }

  function play() {
    if (playing) return;
    playing = true;
    playBtn.disabled = true;
    const p = session.next();
    count++;
    status.textContent = 'Playing ' + p.map.title + '...';
    setGame('select');
    later(MAP_AT_MS, function () {
      setGame('gameplay');
      state.map = p.map;
      broadcast('map.started', p.map);
      const presses = [0, 0, 0, 0];
      for (let t = 0; t < PLAY_MS - MAP_AT_MS; t += KEYS_EVERY_MS) {
        later(t, function () { state.keys = window.qscSim.keysFrame(t, presses); broadcast('keys.frame', state.keys); });
      }
    });
    later(PLAY_MS, function () {
      if (state.keys) {
        state.keys = Object.assign({}, state.keys, { down: [false, false, false, false] });
        broadcast('keys.frame', state.keys);
      }
      setGame('results');
      state.last_score = p.result;
      state.session = p.session;
      broadcast('score.saved', p.result);
      broadcast('session.updated', p.session);
      status.textContent = describe(p.result);
      playing = false;
      playBtn.disabled = false;
      if (auto) later(AUTO_GAP_MS, play);
    });
  }

  function stopAll() {
    timers.forEach(clearTimeout);
    timers = [];
    playing = false;
    playBtn.disabled = false;
  }

  function setAuto(on) {
    auto = on;
    autoBtn.setAttribute('aria-pressed', String(on));
    if (on) play(); else if (!playing) stopAll();
  }

  function reload() {
    frames.forEach(function (fig) { fig.querySelector('iframe').src = frameSrc(fig); });
  }

  frames.forEach(function (fig) {
    const box = fig.querySelector('.ov-box');
    const iframe = fig.querySelector('iframe');
    const w = Number(iframe.getAttribute('width')), h = Number(iframe.getAttribute('height'));
    iframe.addEventListener('load', function () { if (fig.hasAttribute('data-live')) send(fig, 'snapshot', state); });
    new ResizeObserver(function () {
      const k = Math.min(1, box.clientWidth / w);
      box.style.height = Math.ceil(h * k) + 'px';
      iframe.style.transform = 'scale(' + k + ')';
    }).observe(box);
  });

  themePick.addEventListener('change', reload);
  langPick.addEventListener('change', reload);
  playBtn.addEventListener('click', play);
  autoBtn.addEventListener('click', function () { setAuto(autoBtn.getAttribute('aria-pressed') !== 'true'); });
  resetBtn.addEventListener('click', function () {
    setAuto(false);
    stopAll();
    count = 0;
    state.session = session.reset();
    broadcast('session.updated', state.session);
    status.textContent = 'Session reset.';
  });
  document.addEventListener('visibilitychange', function () { if (document.hidden && auto) setAuto(false); });

  reload();
  const first = new IntersectionObserver(function (entries) {
    if (!entries.some(function (e) { return e.isIntersecting; })) return;
    first.disconnect();
    if (count === 0) later(800, play);
  }, { threshold: 0.3 });
  first.observe(document.getElementById('demo-frames'));
})();
