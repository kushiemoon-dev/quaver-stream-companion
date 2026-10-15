package web

import "testing"

func TestStatusModel(t *testing.T) {
	runNodeModel(t, "dashboard/pages/status-model.js", `
const assert = require('assert');
const m = require(process.argv[2]);
const score = { score: { id: 1 }, map: { md5: 'a' } };
const mapA = { md5: 'a', title: 'A' };
const mapB = { md5: 'b', title: 'B' };
const ended = (md5) => ({ map: { md5 }, saved: false });
const run = (s, steps) => steps.reduce((acc, [name, d]) => m.reduce(acc, name, d), s);

let s = run(m.initial(), [['map.started', mapA], ['score.saved', score]]);
assert.strictEqual(s.unsaved, false);

s = run(s, [['map.started', mapA], ['play.ended', ended('a')]]);
assert.strictEqual(s.unsaved, true);
assert.strictEqual(m.showUnsaved(s), true);

assert.strictEqual(run(s, [['map.started', mapB]]).unsaved, false, 'the next play clears the note');
assert.strictEqual(run(s, [['score.saved', score]]).unsaved, false, 'a saved score clears the note');
assert.strictEqual(run(s, [['snapshot', { game: 'menu', map: mapA, last_score: score }]]).unsaved, false);
assert.strictEqual(run(s, [['game.state', { state: 'results' }]]).unsaved, true, 'a state change keeps it');

const late = run(m.initial(), [['score.saved', score], ['map.started', mapA], ['map.started', mapB], ['play.ended', ended('a')]]);
assert.strictEqual(late.unsaved, false, 'an event for another map is ignored');
assert.strictEqual(run(m.initial(), [['play.ended', ended('a')]]).unsaved, false, 'no map shown');
assert.strictEqual(run(s, [['play.ended', { map: { md5: 'a' }, saved: true }]]).unsaved, true);
assert.strictEqual(run(m.initial(), [['map.started', mapA], ['play.ended', { map: { md5: 'a' }, saved: true }]]).unsaved, false);

const first = run(m.initial(), [['map.started', mapA], ['play.ended', ended('a')]]);
assert.strictEqual(first.unsaved, true);
assert.strictEqual(m.showUnsaved(first), false, 'no previous play in the card');

assert.throws(() => m.reduce(s, 'play.ended', null));
assert.throws(() => m.reduce(s, 'score.saved', { score: 1 }));
`)
}
