'use strict';
const assert = require('node:assert/strict');
const { test, run } = require('../../testsupport/run.js');
const { open, stream, visible } = require('./helpers.js');

const PB = { is_pb: true, score: { accuracy: 97.5 }, map: { title: 'Song' } };
const SEP = ' ' + String.fromCharCode(183) + ' ';

function fakeAudio() {
  const made = { contexts: [], oscillators: [], resumed: 0, suspended: false };
  made.AudioContext = class {
    constructor() { this.currentTime = 1; this.state = made.suspended ? 'suspended' : 'running'; this.destination = {}; made.contexts.push(this); }
    resume() { made.resumed++; this.state = 'running'; return Promise.resolve(); }
    createGain() { return { gain: { setValueAtTime() {}, exponentialRampToValueAtTime() {} }, connect() {} }; }
    createOscillator() {
      const o = { frequency: {}, connect() {}, start() {}, stop() {} };
      made.oscillators.push(o);
      return o;
    }
  };
  return made;
}
const withAudio = (made) => ({ globals: { AudioContext: made.AudioContext } });

test('demo shows the example alert silently, even with sound on', () => {
  const made = fakeAudio();
  const win = open('alert', 'demo=1&sound=1', withAudio(made));
  assert.ok(visible(win.$('#card')));
  assert.equal(win.$('.title').textContent, 'New best + Full combo');
  assert.equal(win.$('.sub').textContent, 'Example Song' + SEP + '98.40%');
  assert.equal(made.contexts.length, 0);
  assert.equal(win.eventSources.length, 0);
});

test('the live card stays hidden and ignores snapshots', () => {
  const win = open('alert', '');
  const es = stream(win);
  assert.ok(!visible(win.$('#card')));
  es.emit('snapshot', { game: 'results', last_score: PB });
  assert.ok(!visible(win.$('#card')));
});

test('a personal best shows the title, the map and the accuracy', () => {
  const win = open('alert', '');
  stream(win).emit('score.saved', PB);
  assert.ok(visible(win.$('#card')));
  assert.equal(win.$('.title').textContent, 'New personal best');
  assert.equal(win.$('.title').className, 'title gold');
  assert.equal(win.$('.sub').textContent, 'Song' + SEP + '97.50%');
});

test('a full combo with a best score joins both labels', () => {
  const win = open('alert', 'lang=fr');
  stream(win).emit('score.saved', { ...PB, is_pb: false, is_local_best: true, full_combo: true });
  assert.equal(win.$('.title').textContent, 'Nouveau meilleur score + Full combo');
});

test('plays that earn nothing, or were failed, paused or autoplayed, show nothing', () => {
  const win = open('alert', '');
  const es = stream(win);
  es.emit('score.saved', { score: { accuracy: 90 }, map: { title: 'Song' } });
  for (const flag of ['failed', 'paused', 'autoplay']) es.emit('score.saved', { ...PB, [flag]: true });
  assert.ok(!visible(win.$('#card')));
});

test('a rejected score does not replace the alert on screen', () => {
  const win = open('alert', '');
  const es = stream(win);
  es.emit('score.saved', PB);
  es.emit('score.saved', { ...PB, failed: true, map: { title: 'Other' } });
  assert.equal(win.$('.sub').textContent, 'Song' + SEP + '97.50%');
});

test('the alert hides after eight seconds by default and the timer restarts on a new alert', async () => {
  const win = open('alert', '');
  const es = stream(win);
  es.emit('score.saved', PB);
  await win.advance(7999);
  assert.ok(visible(win.$('#card')));
  es.emit('score.saved', { ...PB, map: { title: 'Again' } });
  await win.advance(7999);
  assert.ok(visible(win.$('#card')));
  await win.advance(1);
  assert.ok(!visible(win.$('#card')));
});

test('autohide parameter overrides the delay and 0 keeps the alert up', async () => {
  const short = open('alert', 'autohide=2');
  stream(short).emit('score.saved', PB);
  await short.advance(2000);
  assert.ok(!visible(short.$('#card')));
  const never = open('alert', 'autohide=0');
  stream(never).emit('score.saved', PB);
  await never.advance(3600000);
  assert.ok(visible(never.$('#card')));
});

test('sound plays two notes only when sound=1', () => {
  const noSound = fakeAudio();
  const quiet = open('alert', '', withAudio(noSound));
  stream(quiet).emit('score.saved', PB);
  assert.equal(noSound.contexts.length, 0);

  const made = fakeAudio();
  const loud = open('alert', 'sound=1', withAudio(made));
  const es = stream(loud);
  es.emit('score.saved', PB);
  assert.equal(made.contexts.length, 1);
  assert.deepEqual(made.oscillators.map((o) => o.frequency.value), [659.25, 880]);
  es.emit('score.saved', PB);
  assert.equal(made.contexts.length, 1, 'the audio context is reused');
  assert.equal(made.oscillators.length, 4);
});

test('a suspended audio context is resumed before the notes play', async () => {
  const made = fakeAudio();
  made.suspended = true;
  const win = open('alert', 'sound=1', withAudio(made));
  stream(win).emit('score.saved', PB);
  await win.flush();
  assert.equal(made.resumed, 1);
  assert.equal(made.oscillators.length, 2);
});

test('a browser without audio support still shows the alert', () => {
  const win = open('alert', 'sound=1');
  stream(win).emit('score.saved', PB);
  assert.ok(visible(win.$('#card')));
});

run();
