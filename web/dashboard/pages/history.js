(function () {
  const qsc = window.qsc;
  const t = qsc.t;
  const h = qsc.h;
  const m = window.qscHistoryModel;
  const TIMEOUT_MS = 8000;
  const FILTERS = ['all', 'pb', 'fc', 'failed'];
  const COLUMNS = ['map', 'difficulty', 'result', 'accuracy', 'combo', 'judgments'];
  const JUDGMENTS = ['marv', 'perf', 'great', 'good', 'okay', 'miss'];
  const BADGES = [
    ['is_pb', 'status.pb', 'pill-amber'],
    ['full_combo', 'status.fc', 'pill-ok'],
    ['failed', 'status.failed', 'pill-bad'],
    ['paused', 'status.paused', 'pill-muted'],
    ['autoplay', 'status.autoplay', 'pill-muted'],
  ];

  let mountId = 0;
  let loadId = 0;
  let lifetime = null;
  let active = null;
  let ui = null;
  let plays = [];
  let filter = 'all';
  let limit = m.DEFAULT_LIMIT;

  function request(path, signal) {
    const life = lifetime;
    const stop = function () { signal.abort(); };
    life.signal.addEventListener('abort', stop);
    let timedOut = false;
    const timer = setTimeout(function () { timedOut = true; signal.abort(); }, TIMEOUT_MS);
    return qsc.api.get(path, signal.signal)
      .catch(function (e) {
        if (e && e.name === 'AbortError') {
          const err = new Error(timedOut ? t('history.timeout') : 'aborted');
          err.aborted = !timedOut;
          throw err;
        }
        throw e;
      })
      .finally(function () {
        clearTimeout(timer);
        life.signal.removeEventListener('abort', stop);
      });
  }

  function announce(text) {
    ui.live.textContent = '';
    requestAnimationFrame(function () { if (ui) ui.live.textContent = text; });
  }

  function judgmentsCell(j) {
    return h('td', { class: 'jcell' }, h('div', { class: 'jgrid' }, JUDGMENTS.map(function (name) {
      return h('span', { class: 'jn num j-' + name }, String((j && j[name]) || 0),
        h('span', { class: 'sr-only' }, ' ' + t('judgment.' + name) + ' '));
    })));
  }

  function modsLine(play) {
    const chips = m.modChips(m.modsWithoutBadges(play.mods, play));
    if (chips.shown.length === 0) return null;
    const out = chips.shown.map(function (name) { return h('span', { class: 'chip' }, name); });
    if (chips.extra > 0) {
      const names = chips.hidden.join(', ');
      out.push(h('span', { class: 'chip', title: names }, '+' + chips.extra, h('span', { class: 'sr-only' }, ' (' + names + ')')));
    }
    return h('div', { class: 'chips tight' }, out);
  }

  function row(p) {
    const lang = window.qscI18n.lang();
    const rate = m.formatRate(p.rate, lang);
    return h('tr', null,
      h('td', { class: 'col-map' },
        h('div', { class: 'hist-title' }, p.title),
        h('div', { class: 'muted hist-artist' }, p.artist),
        h('div', { class: 'muted hist-date nowrap' }, m.formatPlayedAt(p.played_at, lang))),
      h('td', { class: 'col-diff' }, h('div', null, p.difficulty_name),
        h('div', { class: 'num muted' }, '★ ' + m.formatStars(p.difficulty, lang) + (rate ? ' ' + rate : '')),
        modsLine(p)),
      h('td', null,
        h('div', null, h('span', { class: 'gr gr-' + m.gradeClass(p.grade) }, m.gradeText(p.grade))),
        h('div', { class: 'chips tight' }, BADGES.map(function (b) {
          return p[b[0]] === true ? h('span', { class: 'pill ' + b[2] }, t(b[1])) : null;
        }))),
      h('td', { class: 'num' }, m.formatAccuracy(p.accuracy, lang)),
      h('td', { class: 'num' }, String(p.max_combo || 0) + 'x'),
      judgmentsCell(p.judgments));
  }

  function emptyCard(titleKey, bodyKey) {
    return h('section', { class: 'card empty' },
      h('h2', { class: 'empty-title' }, t(titleKey)),
      h('p', { class: 'muted' }, t(bodyKey)));
  }

  function renderResults() {
    if (plays.length === 0) {
      ui.results.replaceChildren(emptyCard('history.empty', 'history.empty_hint'));
      announce(t('history.empty'));
      return;
    }
    const shown = m.filterPlays(plays, filter);
    if (shown.length === 0) {
      ui.results.replaceChildren(emptyCard('history.empty_filtered', 'history.empty_filtered_hint'));
      announce(t('history.empty_filtered'));
      return;
    }
    ui.results.replaceChildren(h('div', { class: 'card table-wrap', role: 'region', tabindex: '0', 'aria-label': t('history.scroll_label') },
      h('table', { class: 'hist-table' },
        h('caption', null, t('history.caption')),
        h('thead', null, h('tr', null, COLUMNS.map(function (c) {
          return h('th', { scope: 'col' }, t('history.col.' + c));
        }))),
        h('tbody', null, shown.map(row)))));
    announce(t('history.count', { shown: shown.length, total: plays.length }));
  }

  function showError(e, refocus) {
    const retry = h('button', { type: 'button', class: 'btn', onclick: function () { load(true); } }, t('history.retry'));
    ui.results.replaceChildren(h('section', { class: 'card empty' },
      e.status === 503
        ? h('p', { class: 'save-status bad', role: 'alert' }, t('history.unavailable'), ' ', h('a', { href: '#/status' }, t('history.unavailable_link')))
        : h('p', { class: 'save-status bad', role: 'alert' }, t('history.load_failed', { message: e.message })),
      retry));
    if (refocus) retry.focus();
  }

  async function load(refocus) {
    const id = mountId;
    const mine = ++loadId;
    if (active) active.abort();
    const ctl = new AbortController();
    active = ctl;
    ui.results.replaceChildren(h('p', { class: 'muted', role: 'status' }, t('history.loading')));
    ui.results.setAttribute('aria-busy', 'true');
    try {
      const data = await request('/api/history?limit=' + limit, ctl);
      if (id !== mountId || mine !== loadId) return;
      plays = m.playsOf(data);
      ui.results.removeAttribute('aria-busy');
      renderResults();
      if (refocus) ui.limit.focus();
    } catch (e) {
      if (id !== mountId || mine !== loadId || e.aborted) return;
      ui.results.removeAttribute('aria-busy');
      showError(e, refocus);
    }
  }

  function buildControls() {
    ui.limit = h('select', { id: 'hist-limit', onchange: function (ev) {
      limit = m.parseLimit(ev.target.value);
      load(false);
    } }, m.LIMITS.map(function (n) { return h('option', { value: n, selected: n === limit }, String(n)); }));
    const seg = FILTERS.map(function (name) {
      return h('label', { class: 'seg' },
        h('input', { type: 'radio', name: 'hist-filter', value: name, class: 'seg-input', checked: name === filter, onchange: function () {
          filter = name;
          if (!ui.results.hasAttribute('aria-busy') && !ui.results.querySelector('[role="alert"]')) renderResults();
        } }),
        h('span', { class: 'seg-label' }, t('history.filter.' + name)));
    });
    return h('div', { class: 'controls-row' },
      h('fieldset', { class: 'seg-group' }, h('legend', null, t('history.filter_label')), h('div', { class: 'seg-row' }, seg)),
      h('div', { class: 'field' }, h('label', { for: 'hist-limit' }, t('history.limit')), ui.limit));
  }

  function mount(root) {
    mountId++;
    lifetime = new AbortController();
    plays = [];
    filter = 'all';
    ui = {
      live: h('p', { class: 'sr-only', role: 'status', 'aria-live': 'polite' }),
      results: h('div', { class: 'history-results' }),
    };
    root.appendChild(h('header', { class: 'page-head' },
      h('div', null, h('h1', null, t('nav.history')), h('p', { class: 'subtitle' }, t('history.subtitle')))));
    root.appendChild(buildControls());
    root.appendChild(ui.live);
    root.appendChild(ui.results);
    load(false);
  }

  function unmount() {
    mountId++;
    if (active) active.abort();
    active = null;
    if (lifetime) lifetime.abort();
    lifetime = null;
    ui = null;
    plays = [];
  }

  qsc.registerPage('history', { title: 'nav.history', mount: mount, unmount: unmount });
})();
