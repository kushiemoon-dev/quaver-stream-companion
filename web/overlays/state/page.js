const params = qscParams;
let game = 'unknown';
const pill = document.getElementById('pill');
const dot = document.getElementById('dot');
document.title = qscT('state.title');

function render() {
  if (qscModel.isHiddenState(game, params.hide)) {
    pill.classList.add('hidden');
    return;
  }
  document.getElementById('label').textContent = qscT(qscModel.stateKey(game));
  dot.className = 'dot ' + qscModel.stateTone(game) + (game === 'gameplay' ? ' pulse' : '');
  pill.classList.remove('hidden');
}

if (params.demo) {
  game = 'gameplay';
  render();
} else {
  render();
  qscConnect({
    'snapshot': function (d) { game = d.game; render(); },
    'game.state': function (d) { game = d.state; render(); },
  });
}
