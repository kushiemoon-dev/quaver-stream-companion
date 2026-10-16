const params = qscParams;
const DEFAULT_SECONDS = 8;
const NOTES = [[659.25, 0], [880, 0.12]];
const card = document.getElementById('card');
document.title = qscT('alert.title');
const autohide = qscAutohide(function () { card.classList.add('hidden'); }, DEFAULT_SECONDS);
let audio = null;

function chime() {
  try {
    const Ctx = window.AudioContext || window.webkitAudioContext;
    if (!Ctx) return;
    audio = audio || new Ctx();
    const play = function () {
      const now = audio.currentTime;
      NOTES.forEach(function (n) {
        const osc = audio.createOscillator();
        const gain = audio.createGain();
        osc.type = 'sine';
        osc.frequency.value = n[0];
        gain.gain.setValueAtTime(0.0001, now + n[1]);
        gain.gain.exponentialRampToValueAtTime(0.2, now + n[1] + 0.02);
        gain.gain.exponentialRampToValueAtTime(0.0001, now + n[1] + 0.35);
        osc.connect(gain);
        gain.connect(audio.destination);
        osc.start(now + n[1]);
        osc.stop(now + n[1] + 0.4);
      });
    };
    if (audio.state === 'suspended') audio.resume().then(play, function () {});
    else play();
  } catch (e) {}
}

function render(view) {
  const head = view.kinds.map(function (k) { return qscT('alert.' + k); }).join(' + ');
  const sub = view.title ? view.title + ' · ' + view.accuracy : view.accuracy;
  card.replaceChildren(qscEl('div', 'title gold', head), qscEl('div', 'sub', sub));
  card.classList.remove('hidden');
}

function onScore(result) {
  const view = qscModel.alertView(result, params.lang);
  if (!view) return;
  render(view);
  autohide.touch();
  if (params.sound) chime();
}

if (params.demo) {
  render(qscModel.alertView({ is_pb: true, is_local_best: true, full_combo: true, score: { accuracy: 98.4 }, map: { title: 'Example Song' } }, params.lang));
} else {
  qscConnect({ 'score.saved': onScore });
}
