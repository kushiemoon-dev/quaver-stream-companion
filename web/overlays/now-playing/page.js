let game = 'unknown';
let map = null;
const params = qscParams;
const card = document.getElementById('card');
document.title = qscT('nowplaying.title');
let expired = false;
let firstSnapshot = true;
const autohide = qscAutohide(function () { expired = true; card.classList.add('hidden'); });

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
  card.classList.remove('hidden');
}

const isDemo = new URLSearchParams(location.search).get('demo') === '1';

if (isDemo) {
  game = 'gameplay';
  map = { title: 'Example Song', artist: 'Example Artist', difficulty_name: 'Hard', difficulty: 18.5, keys: 4 };
  fresh();
} else {
  qscConnect({
    'snapshot': function (d) {
      game = d.game; map = d.map;
      if (firstSnapshot) { firstSnapshot = false; fresh(); } else render();
    },
    'game.state': function (d) { game = d.state; render(); },
    'map.started': function (d) { map = d; fresh(); },
    'score.saved': function () { fresh(); },
  });
}
