(function () {
  const qsc = window.qsc;
  const t = qsc.t;
  const h = qsc.h;
  const GAME_STATES = ['initialization', 'menu', 'select', 'loading', 'gameplay', 'results', 'closed', 'unknown'];
  const CHECKS = ['quaver_dir', 'log', 'database', 'port'];
  const JUDGMENTS = ['marv', 'perf', 'great', 'good', 'okay', 'miss'];
  const DETAIL_KEYS = {
    'readable': 'check.detail.readable',
    'in use by this qsc': 'check.detail.owned',
    'free': 'check.detail.free',
    'Quaver folder not found': 'check.detail.folder_not_found',
  };
  const REFRESH_MS = 5000;

  const RETRY_MS = 3000;
  const DIAG_TIMEOUT_MS = 8000;

  let es = null;
  let timer = null;
  let retryTimer = null;
  let mountId = 0;
  let inflight = false;
  let ui = null;
  let state = { game: 'unknown', map: null, last_score: null };
  let diag = { checks: null, error: false, version: '', configPath: '' };
  let connection = 'connecting';

  function fmtRate(rate) {
    return !rate || rate === 1 ? '' : String(Number(rate.toFixed(2))) + 'x';
  }

  function cardLabel(key) {
    return h('h2', { class: 'card-label' }, t(key));
  }

  function renderGame() {
    const g = GAME_STATES.indexOf(state.game) >= 0 ? state.game : 'unknown';
    if (ui.game.getAttribute('data-state') === g) return;
    ui.game.setAttribute('data-state', g);
    ui.game.className = 'pill pill-state state-' + g;
    ui.game.replaceChildren(h('span', { class: 'dot', 'aria-hidden': 'true' }), t('game.' + g));
  }

  function renderConnection() {
    const ok = connection === 'live';
    ui.conn.className = 'pill ' + (ok ? 'pill-ok' : 'pill-warn');
    ui.conn.replaceChildren(h('span', { class: 'dot', 'aria-hidden': 'true' }), t('status.' + connection));
  }

  function renderNow() {
    const m = state.map;
    ui.now.replaceChildren(cardLabel('status.now_playing'));
    if (!m) {
      ui.now.appendChild(h('p', { class: 'empty-title' }, t('status.no_map')));
      ui.now.appendChild(h('p', { class: 'muted' }, t('status.no_map_hint')));
      return;
    }
    ui.now.appendChild(h('p', { class: 'map-title' }, m.title));
    ui.now.appendChild(h('p', { class: 'map-artist' }, m.artist));
    ui.now.appendChild(h('div', { class: 'chips' },
      m.difficulty_name ? h('span', { class: 'chip' }, m.difficulty_name) : null,
      h('span', { class: 'chip chip-accent' }, '★ ' + Number(m.difficulty || 0).toFixed(1)),
      h('span', { class: 'chip' }, t('status.keys', { n: m.keys }))));
  }

  function badge(on, key, cls) {
    return on ? h('span', { class: 'pill ' + cls }, t(key)) : null;
  }

  function renderLast() {
    const r = state.last_score;
    ui.last.replaceChildren(cardLabel('status.last_play'));
    if (!r) {
      ui.last.appendChild(h('p', { class: 'empty-title' }, t('status.no_score')));
      ui.last.appendChild(h('p', { class: 'muted' }, t('status.no_score_hint')));
      return;
    }
    const s = r.score;
    const rate = fmtRate(s.rate);
    ui.last.appendChild(h('div', { class: 'last-top' },
      h('div', { class: 'grade', 'data-grade': String(s.grade || '').toLowerCase() },
        h('span', { class: 'sr-only' }, t('status.grade') + ' '), s.grade || '?'),
      h('div', { class: 'last-meta' },
        h('p', { class: 'map-title small' }, r.map.title),
        h('p', { class: 'num acc' }, Number(s.accuracy).toFixed(2) + '%'),
        h('p', { class: 'muted' }, t('status.max_combo', { n: s.max_combo }))),
    ));
    ui.last.appendChild(h('div', { class: 'judgments' }, JUDGMENTS.map(function (j) {
      return h('div', { class: 'judgment j-' + j },
        h('span', { class: 'j-label' }, t('judgment.' + j)),
        h('span', { class: 'num j-count' }, String((s.judgments || {})[j] || 0)));
    })));
    ui.last.appendChild(h('div', { class: 'chips' },
      rate ? h('span', { class: 'chip chip-accent' }, rate) : null,
      (s.mods || []).map(function (m) { return h('span', { class: 'chip' }, m); }),
      badge(r.is_pb, 'status.pb', 'pill-ok'),
      badge(r.full_combo, 'status.fc', 'pill-amber'),
      badge(r.failed, 'status.failed', 'pill-bad'),
      badge(r.paused, 'status.paused', 'pill-muted'),
      badge(r.autoplay, 'status.autoplay', 'pill-muted')));
  }

  function renderDiag() {
    ui.diag.replaceChildren();
    const byId = {};
    (diag.checks || []).forEach(function (c) { byId[c.id] = c; });
    CHECKS.forEach(function (id) {
      const c = byId[id];
      const cls = !c ? 'unknown' : c.ok ? 'ok' : 'bad';
      const detail = c ? (Object.prototype.hasOwnProperty.call(DETAIL_KEYS, c.detail) ? t(DETAIL_KEYS[c.detail]) : c.detail) : diag.error ? t('status.unreachable') : t('status.checking');
      ui.diag.appendChild(h('section', { class: 'card diag diag-' + cls },
        cardLabel('check.' + id),
        h('p', { class: 'diag-state' }, h('span', { class: 'dot', 'aria-hidden': 'true' }),
          t(!c ? 'status.unknown' : c.ok ? 'status.ok' : 'status.problem')),
        h('p', { class: 'muted diag-detail' }, qsc.pathNodes(detail))));
    });
    const items = [];
    if (diag.version) items.push(h('span', null, t('status.version', { v: diag.version })));
    if (diag.configPath) items.push(h('span', null, t('status.config_file'), ' ', h('code', null, qsc.pathNodes(diag.configPath))));
    const foot = [];
    items.forEach(function (item, i) {
      if (i > 0) foot.push(h('span', { class: 'sep', 'aria-hidden': 'true' }));
      foot.push(item);
    });
    ui.foot.replaceChildren.apply(ui.foot, foot);
    ui.diagErr.replaceChildren(diag.error && diag.checks ? t('status.refresh_failed') : '');
  }

  async function refreshDiag() {
    if (inflight || !ui) return;
    const id = mountId;
    inflight = true;
    let next;
    const ctl = new AbortController();
    const timeout = setTimeout(function () { ctl.abort(); }, DIAG_TIMEOUT_MS);
    try {
      const s = await qsc.api.get('/api/status', ctl.signal);
      next = { checks: s.checks || [], error: false, version: s.version, configPath: s.config_path };
    } catch (e) {
      next = Object.assign({}, diag, { error: true });
    } finally {
      clearTimeout(timeout);
    }
    if (id !== mountId) return;
    inflight = false;
    diag = next;
    renderDiag();
  }

  function isObject(v) {
    return v !== null && typeof v === 'object' && !Array.isArray(v);
  }

  function isScore(r) {
    return isObject(r) && isObject(r.score) && isObject(r.map);
  }

  function reduce(name, d) {
    if (!isObject(d)) throw new Error('bad payload');
    if (name === 'snapshot') {
      return { game: typeof d.game === 'string' ? d.game : 'unknown', map: isObject(d.map) ? d.map : null, last_score: isScore(d.last_score) ? d.last_score : null };
    }
    if (name === 'game.state') {
      if (typeof d.state !== 'string') throw new Error('bad payload');
      return Object.assign({}, state, { game: d.state });
    }
    if (name === 'map.started') return Object.assign({}, state, { map: d });
    if (!isScore(d)) throw new Error('bad payload');
    return Object.assign({}, state, { last_score: d });
  }

  function listen(source, name) {
    source.addEventListener(name, function (ev) {
      if (source !== es) return;
      let next;
      try {
        next = reduce(name, JSON.parse(ev.data).data);
      } catch (e) {
        return;
      }
      const prev = state;
      state = next;
      if (!renderAll()) {
        state = prev;
        renderAll();
      }
    });
  }

  function renderAll() {
    if (!ui) return false;
    try {
      renderGame();
      renderNow();
      renderLast();
      return true;
    } catch (e) {
      return false;
    }
  }

  function connect() {
    retryTimer = null;
    const source = new EventSource('/events');
    es = source;
    source.onopen = function () { connection = 'live'; renderConnection(); };
    source.onerror = function () {
      connection = 'reconnecting';
      renderConnection();
      if (source.readyState === EventSource.CLOSED && source === es) {
        source.close();
        retryTimer = setTimeout(connect, RETRY_MS);
      }
    };
    ['snapshot', 'game.state', 'map.started', 'score.saved'].forEach(function (name) { listen(source, name); });
  }

  function onVisibility() {
    if (!document.hidden) refreshDiag();
  }

  function mount(root) {
    ui = {
      game: h('span', { class: 'pill pill-state state-unknown' }),
      diagErr: h('p', { class: 'muted diag-error' }),
      conn: h('span', { class: 'pill pill-warn', role: 'status' }),
      now: h('section', { class: 'card card-now' }),
      last: h('section', { class: 'card card-last' }),
      diag: h('div', { class: 'grid grid-4' }),
      foot: h('footer', { class: 'footer' }),
    };
    root.appendChild(h('header', { class: 'page-head' },
      h('div', null, h('h1', null, t('nav.status')), h('p', { class: 'subtitle' }, t('status.subtitle'))),
      h('div', { class: 'head-pills' }, ui.conn, h('span', { 'aria-live': 'polite' }, ui.game))));
    root.appendChild(h('div', { class: 'grid grid-2' }, ui.now, ui.last));
    root.appendChild(h('h2', { class: 'section-label' }, t('status.diagnostics')));
    root.appendChild(ui.diag);
    root.appendChild(ui.diagErr);
    root.appendChild(ui.foot);
    mountId++;
    inflight = false;
    connection = 'connecting';
    state = { game: 'unknown', map: null, last_score: null };
    diag = { checks: null, error: false, version: '', configPath: '' };
    renderConnection();
    renderAll();
    renderDiag();
    connect();
    refreshDiag();
    timer = setInterval(function () { if (!document.hidden) refreshDiag(); }, REFRESH_MS);
    document.addEventListener('visibilitychange', onVisibility);
  }

  function unmount() {
    mountId++;
    inflight = false;
    if (es) es.close();
    es = null;
    clearTimeout(retryTimer);
    clearInterval(timer);
    document.removeEventListener('visibilitychange', onVisibility);
    ui = null;
  }

  qsc.registerPage('status', { title: 'nav.status', mount: mount, unmount: unmount });
})();
