(function () {
  const qsc = window.qsc;
  const t = qsc.t;
  const h = qsc.h;
  const GAME_STATES = ['initialization', 'menu', 'select', 'loading', 'gameplay', 'results', 'editor', 'importing', 'download', 'lobby', 'multiplayer', 'music', 'theatre', 'closed', 'unknown'];
  const CHECKS = ['quaver_dir', 'log', 'database', 'port'];
  const JUDGMENTS = ['marv', 'perf', 'great', 'good', 'okay', 'miss'];
  const DETAIL_KEYS = {
    'readable': 'check.detail.readable',
    'in use by this qsc': 'check.detail.owned',
    'free': 'check.detail.free',
    'Quaver folder not found': 'check.detail.folder_not_found',
    'busy: Quaver is writing the database': 'check.detail.busy',
  };
  const REFRESH_MS = 5000;

  const RETRY_MS = 3000;
  const DIAG_TIMEOUT_MS = 8000;

  let es = null;
  let timer = null;
  let retryTimer = null;
  let mountId = 0;
  let inflight = false;
  let copying = false;
  let ui = null;
  let state = window.qscStatusModel.initial();
  let diag = { checks: null, error: false, version: '', configPath: '' };
  let connection = 'connecting';
  let shownDegraded = '';
  let offStop = null;

  function fmtRate(rate) {
    return !rate || rate === 1 ? '' : window.qscHistoryModel.formatRate(rate, window.qscI18n.lang());
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
      h('span', { class: 'chip chip-accent' }, '★ ' + window.qscHistoryModel.formatStars(m.difficulty, window.qscI18n.lang())),
      h('span', { class: 'chip' }, t('status.keys', { n: m.keys }))));
  }

  function badge(on, key, cls) {
    return on ? h('span', { class: 'pill ' + cls }, t(key)) : null;
  }

  function renderLast() {
    const r = state.last_score;
    ui.lastBody.replaceChildren(cardLabel('status.last_play'));
    if (!r) {
      ui.lastBody.appendChild(h('p', { class: 'empty-title' }, t('status.no_score')));
      ui.lastBody.appendChild(h('p', { class: 'muted' }, t('status.no_score_hint')));
    } else {
      fillLast(r);
    }
    const note = window.qscStatusModel.showUnsaved(state) ? t('status.unsaved') : '';
    if (ui.unsaved.textContent !== note) ui.unsaved.textContent = note;
  }

  function fillLast(r) {
    const s = r.score;
    const rate = fmtRate(s.rate);
    ui.lastBody.appendChild(h('div', { class: 'last-top' },
      h('div', { class: 'grade', 'data-grade': String(s.grade || '').toLowerCase() },
        h('span', { class: 'sr-only' }, t('status.grade') + ' '), s.grade || '?'),
      h('div', { class: 'last-meta' },
        h('p', { class: 'map-title small' }, r.map.title),
        h('p', { class: 'num acc' }, window.qscHistoryModel.formatAccuracy(s.accuracy, window.qscI18n.lang())),
        h('p', { class: 'muted' }, t('status.max_combo', { n: s.max_combo }))),
    ));
    ui.lastBody.appendChild(h('div', { class: 'judgments' }, JUDGMENTS.map(function (j) {
      return h('div', { class: 'judgment j-' + j },
        h('span', { class: 'j-label' }, t('judgment.' + j)),
        h('span', { class: 'num j-count' }, String((s.judgments || {})[j] || 0)));
    })));
    ui.lastBody.appendChild(h('div', { class: 'chips' },
      rate ? h('span', { class: 'chip chip-accent' }, rate) : null,
      window.qscHistoryModel.modsWithoutBadges(s.mods, r).map(function (m) { return h('span', { class: 'chip' }, m); }),
      badge(r.is_pb, 'status.pb', 'pill-ok'),
      badge(r.full_combo, 'status.fc', 'pill-amber'),
      badge(r.failed, 'status.failed', 'pill-bad'),
      badge(r.paused, 'status.paused', 'pill-muted'),
      badge(r.autoplay, 'status.autoplay', 'pill-muted')));
  }

  const WIN_PATH = ['C:', 'Program Files (x86)', 'Steam', 'steamapps', 'common', 'Quaver'].join(String.fromCharCode(92));
  const LINUX_PATH = '~/.local/share/Steam/steamapps/common/Quaver';

  // A passing check reports a path. A failing one without a translation keeps its raw text after a generic line.
  function detailNodes(detail, ok) {
    if (Object.prototype.hasOwnProperty.call(DETAIL_KEYS, detail)) return qsc.pathNodes(t(DETAIL_KEYS[detail]));
    if (ok) return qsc.pathNodes(detail);
    return [t('check.detail.other'), ' ', h('code', null, qsc.pathNodes(detail))];
  }

  function isStale() {
    return window.qscOffline.isOffline() || (diag.error && !!diag.checks);
  }

  function failing(kind) {
    const ids = kind === 'folder' ? ['quaver_dir'] : ['log', 'database'];
    return (diag.checks || []).filter(function (c) { return ids.indexOf(c.id) >= 0 && !c.ok; })[0] || null;
  }

  function renderDegraded() {
    const kind = failing('folder') ? 'folder' : failing('files') ? 'files' : '';
    const bad = kind ? failing(kind) : null;
    const key = kind + (bad ? bad.detail : '');
    if (key === shownDegraded) return;
    shownDegraded = key;
    ui.degraded.hidden = !kind;
    ui.degraded.replaceChildren();
    if (!kind) return;
    ui.degraded.appendChild(h('strong', null, t('status.degraded.' + kind + '_title')));
    ui.degraded.appendChild(h('p', null, t('status.degraded.intro')));
    if (kind === 'folder') {
      ui.degraded.appendChild(h('ul', { class: 'degraded-steps' },
        h('li', null, t('status.degraded.step_install')),
        h('li', null, t('status.degraded.step_windows'), ' ', h('code', null, qsc.pathNodes(WIN_PATH))),
        h('li', null, t('status.degraded.step_linux'), ' ', h('code', null, qsc.pathNodes(LINUX_PATH))),
        h('li', null, t('status.degraded.step_settings'))));
    }
    ui.degraded.appendChild(h('p', { class: 'muted' }, detailNodes(bad.detail, false)));
    ui.degraded.appendChild(h('p', null, t('status.degraded.auto')));
    if (kind === 'folder') ui.degraded.appendChild(h('a', { class: 'btn', href: '#/settings' }, t('status.degraded.open_settings')));
  }

  function renderDiag() {
    renderDegraded();
    ui.diag.replaceChildren();
    const stale = isStale();
    const byId = {};
    (diag.checks || []).forEach(function (c) { byId[c.id] = c; });
    CHECKS.forEach(function (id) {
      const c = byId[id];
      const cls = !c ? 'unknown' : c.ok ? 'ok' : 'bad';
      const detail = c ? detailNodes(c.detail, c.ok) : qsc.pathNodes(diag.error ? t('status.unreachable') : t('status.checking'));
      ui.diag.appendChild(h('section', { class: 'card diag diag-' + cls + (stale ? ' is-stale' : '') },
        cardLabel('check.' + id),
        stale ? h('p', { class: 'stale-label' }, t('status.stale')) : null,
        h('p', { class: 'diag-state' }, h('span', { class: 'dot', 'aria-hidden': 'true' }),
          t(!c ? 'status.unknown' : c.ok ? 'status.ok' : 'status.problem')),
        h('p', { class: 'muted diag-detail' }, detail)));
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
    ui.diagErr.replaceChildren(stale && diag.checks ? t('status.refresh_failed') : '');
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

  function showManualReport(text) {
    ui.copyBox.replaceChildren(h('textarea', { class: 'copy-area', readonly: true, rows: 8, 'aria-label': t('status.copy_diag') }, text));
    ui.copyBox.hidden = false;
    const area = ui.copyBox.firstChild;
    area.focus();
    area.select();
  }

  // The button stays enabled (aria-disabled only) so keyboard focus is never lost.
  async function onCopyDiag() {
    if (copying) return;
    copying = true;
    const id = mountId;
    ui.copyDiag.setAttribute('aria-disabled', 'true');
    ui.copyStatus.textContent = '';
    ui.copyBox.hidden = true;
    const ctl = new AbortController();
    const timeout = setTimeout(function () { ctl.abort(); }, DIAG_TIMEOUT_MS);
    let message;
    let manual = null;
    try {
      const res = await qsc.api.get('/api/diagnostics', ctl.signal);
      const report = String((res && res.report) || '');
      try {
        await window.qscCopy.copy(report);
        message = t('status.copy_diag_done');
      } catch (e) {
        manual = report;
        message = t('settings.copy_manual');
      }
    } catch (e) {
      const why = e && e.name === 'AbortError' ? t('settings.timeout') : e && e.status ? e.message : t('status.unreachable');
      message = t('status.copy_diag_failed', { message: why });
    } finally {
      clearTimeout(timeout);
      copying = false;
    }
    if (id !== mountId) return;
    ui.copyDiag.removeAttribute('aria-disabled');
    if (manual !== null) showManualReport(manual);
    requestAnimationFrame(function () { if (ui) ui.copyStatus.textContent = message; });
  }

  function listen(source, name) {
    source.addEventListener(name, function (ev) {
      if (source !== es) return;
      let next;
      try {
        next = window.qscStatusModel.reduce(state, name, JSON.parse(ev.data).data);
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
    source.onopen = function () { connection = 'live'; renderConnection(); window.qscOffline.report(true); };
    source.onerror = function () {
      connection = 'reconnecting';
      renderConnection();
      if (source.readyState === EventSource.CLOSED && source === es) {
        source.close();
        retryTimer = setTimeout(connect, RETRY_MS);
      }
    };
    ['snapshot', 'game.state', 'map.started', 'score.saved', 'play.ended'].forEach(function (name) { listen(source, name); });
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
      lastBody: h('div'),
      unsaved: h('p', { class: 'unsaved-note', role: 'status', 'aria-live': 'polite' }),
      degraded: h('section', { class: 'banner banner-degraded', role: 'status', hidden: '' }),
      diag: h('div', { class: 'grid grid-4' }),
      copyDiag: h('button', { type: 'button', class: 'btn', onclick: onCopyDiag }, t('status.copy_diag')),
      copyBox: h('div', { class: 'copy-box', hidden: '' }),
      copyStatus: h('p', { class: 'muted status-line', role: 'status', 'aria-live': 'polite' }),
      foot: h('footer', { class: 'footer' }),
    };
    root.appendChild(h('header', { class: 'page-head' },
      h('div', null, h('h1', null, t('nav.status')), h('p', { class: 'subtitle' }, t('status.subtitle'))),
      h('div', { class: 'head-pills' }, ui.conn, h('span', { 'aria-live': 'polite' }, ui.game))));
    ui.last.append(ui.lastBody, ui.unsaved);
    root.appendChild(ui.degraded);
    root.appendChild(h('div', { class: 'grid grid-2' }, ui.now, ui.last));
    root.appendChild(h('h2', { class: 'section-label' }, t('status.diagnostics')));
    root.appendChild(ui.diag);
    root.appendChild(ui.diagErr);
    root.appendChild(h('div', { class: 'save-row' }, ui.copyDiag, h('p', { class: 'muted hint' }, t('status.copy_diag_hint'))));
    root.appendChild(ui.copyStatus);
    root.appendChild(ui.copyBox);
    root.appendChild(ui.foot);
    mountId++;
    inflight = false;
    copying = false;
    connection = 'connecting';
    state = window.qscStatusModel.initial();
    diag = { checks: null, error: false, version: '', configPath: '' };
    shownDegraded = '';
    renderConnection();
    renderAll();
    renderDiag();
    offStop = window.qscOffline.onChange(function () { if (ui) renderDiag(); });
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
    if (offStop) offStop();
    offStop = null;
    document.removeEventListener('visibilitychange', onVisibility);
    ui = null;
  }

  qsc.registerPage('status', { title: 'nav.status', mount: mount, unmount: unmount });
})();
