package web

import (
	"strings"
	"testing"
)

func TestOverlaysResyncOnSnapshots(t *testing.T) {
	for _, name := range []string{"result", "curve"} {
		src := readWeb(t, "overlays/"+name+"/index.html")
		for _, want := range []string{"qscModel.createIdGate()", "'snapshot'", "gate.fresh("} {
			if !strings.Contains(src, want) {
				t.Errorf("%s is missing %q", name, want)
			}
		}
		if strings.Contains(src, "!shown") {
			t.Errorf("%s must decide on the score id, not on a shown flag", name)
		}
	}
}

func TestIdGateAndMalformedEvents(t *testing.T) {
	runNodeModel(t, "overlays/model.js", `
const assert = require('assert');
const m = require(process.argv[2]);

const gate = m.createIdGate();
assert.strictEqual(gate.fresh(null), false);
assert.strictEqual(gate.fresh({ score: { id: 5 } }), true);
assert.strictEqual(gate.fresh({ score: { id: 5 } }), false, 'an id already rendered is never rendered again');
assert.strictEqual(gate.fresh({ score: { id: 6 } }), true);
assert.strictEqual(gate.fresh({ score: { id: 5 } }), true, 'a different id renders');
assert.strictEqual(gate.fresh({ score: {} }), true, 'a score without an id still renders');
assert.strictEqual(gate.fresh({ score: {} }), true);

const r = m.resultView({ score: { mods: ['Paused', 'Mirror', 'Autoplay'] }, paused: true, autoplay: true });
assert.deepStrictEqual(r.badges, ['paused', 'autoplay']);
assert.deepStrictEqual(r.mods.shown, ['Mirror'], 'badge mods are not repeated as chips');
assert.strictEqual(r.mods.extra, 0);
const keep = m.resultView({ score: { mods: ['Paused'] }, paused: false });
assert.deepStrictEqual(keep.mods.shown, ['Paused']);

const made = [];
const fake = (url) => {
  const es = { handlers: {}, readyState: 1, addEventListener(n, f) { this.handlers[n] = f; }, close() {} };
  made.push(es);
  return es;
};
const timers = { setTimeout() { return 1; }, clearTimeout() {} };
const got = [];
m.createConnection(fake, '/events', { 'score.saved': (d) => got.push(d) }, timers);
for (const bad of ['not json', '', 'null', '{"data":', '[1]']) made[0].handlers['score.saved']({ data: bad });
made[0].handlers['score.saved']({ data: JSON.stringify({ data: { id: 2 } }) });
assert.deepStrictEqual(got, [{ id: 2 }], 'malformed events are ignored');
`)
}
