const params = qscParams;
const TICK_MS = 1000;
const card = document.getElementById('card');
document.title = qscT('session.title');
const FIELDS = [
  ['plays', qscT('session.plays')],
  ['outcome', qscT('session.outcome')],
  ['average', qscT('session.average')],
  ['best', qscT('session.best')],
  ['streak', qscT('session.streak')],
  ['bestStreak', qscT('session.best_streak')],
  ['fullCombos', qscT('session.full_combos')],
  ['rating', qscT('session.rating')],
];
const values = {};
const grid = qscEl('div', 'session-grid');
FIELDS.forEach(function (f) {
  const value = qscEl('b');
  values[f[0]] = value;
  const cell = qscEl('div', 'session-cell');
  cell.append(qscEl('span', null, f[1]), value);
  grid.appendChild(cell);
});
const elapsed = qscEl('span', 'session-time');
const head = qscEl('div', 'session-head');
head.append(qscEl('span', 'title', qscT('session.heading')), elapsed);
card.append(head, grid);

let session = null;

function render() {
  const v = qscModel.sessionView(session, Date.now(), params.lang);
  elapsed.textContent = v.elapsed;
  FIELDS.forEach(function (f) { values[f[0]].textContent = v[f[0]]; });
}

function show(next) {
  session = next;
  render();
  card.classList.remove('hidden');
}

if (params.demo) {
  show({ started_at: new Date(Date.now() - 3723000).toISOString(), plays: 12, passes: 9, fails: 3, avg_accuracy: 95.21, best_accuracy: 98.7, best_rating: 24.53, full_combos: 2, streak: 4, best_streak: 6 });
  setInterval(render, TICK_MS);
} else {
  qscConnect({
    'snapshot': function (d) { if (d.session) show(d.session); },
    'session.updated': show,
  });
  setInterval(function () { if (session) render(); }, TICK_MS);
}
