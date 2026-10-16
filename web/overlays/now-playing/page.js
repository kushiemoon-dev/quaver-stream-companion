let game = 'unknown';
let map = null;
let rate = 1;
const params = qscParams;
const card = document.getElementById('card');
document.title = qscT('nowplaying.title');
let expired = false;
let firstSnapshot = true;
const META_LABELS = { by: qscT('nowplaying.by'), notes: qscT('nowplaying.notes'), ln: qscT('nowplaying.ln'), mines: qscT('nowplaying.mines') };
const autohide = qscAutohide(function () { expired = true; card.classList.add('hidden'); });

// The rate is only known once a score of this map exists.
function rateOf(result) {
  const r = result && result.score ? result.score.rate : 0;
  return result && result.map && map && result.map.md5 === map.md5 && r > 0 ? r : 1;
}

function fresh() {
  expired = false;
  render();
  autohide.touch();
}

function render() {
  if (expired || !map || game === 'menu' || game === 'initialization' || game === 'closed') {
    autohide.stop();
    card.classList.add('hidden');
    return;
  }
  const v = qscModel.nowPlayingView(map, params.lang);
  document.getElementById('title').textContent = v.title;
  document.getElementById('artist').textContent = v.artist;
  document.getElementById('diff').textContent = v.line;
  const meta = params.meta ? qscModel.metaLine(map, rate, params.lang, META_LABELS) : '';
  document.getElementById('meta').textContent = meta;
  document.getElementById('meta').hidden = meta === '';
  card.classList.remove('hidden');
}

const isDemo = new URLSearchParams(location.search).get('demo') === '1';

if (isDemo) {
  game = 'gameplay';
  map = { title: 'Example Song', artist: 'Example Artist', difficulty_name: 'Hard', difficulty: 18.5, keys: 4, creator: 'Example Mapper', bpm: 150, length_ms: 120000, regular_notes: 900, long_notes: 150, mines: 0 };
  fresh();
} else {
  qscConnect({
    'snapshot': function (d) {
      game = d.game; map = d.map; rate = rateOf(d.last_score);
      if (firstSnapshot) { firstSnapshot = false; fresh(); } else render();
    },
    'game.state': function (d) { game = d.state; render(); },
    'map.started': function (d) { map = d; rate = 1; fresh(); },
    'score.saved': function (d) { rate = rateOf(d); fresh(); },
  });
}
