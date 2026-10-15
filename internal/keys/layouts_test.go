package keys

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/bus"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/scores"
)

func TestDefaultsCoverEveryLayoutWithQuaverBindings(t *testing.T) {
	d := Defaults()
	for n := 1; n <= MaxKeys; n++ {
		got := d.Lanes(n)
		if len(got) != n || len(slices.Compact(slices.Sorted(slices.Values(got)))) != n {
			t.Errorf("default %dk = %v", n, got)
		}
	}
	if !reflect.DeepEqual(d.Lanes(10), []Key{"A", "S", "D", "F", "V", "B", "H", "J", "K", "L"}) || !reflect.DeepEqual(d.Lanes(1), []Key{"SPACE"}) {
		t.Fatalf("defaults = %+v", d)
	}
	if d.Scratch != "" || d.ScratchKey() != DefaultScratch || d.Lanes(0) != nil || d.Lanes(11) != nil {
		t.Fatalf("scratch or out of range lanes: %+v", d)
	}
}

func TestEveryLayoutChecksItsOwnCount(t *testing.T) {
	for n := 1; n <= MaxKeys; n++ {
		field := fmt.Sprintf("keys.k%d", n)
		edits := map[string]func(*Config){
			"too short":  func(c *Config) { *c.list(n) = (*c.list(n))[:n-1] },
			"too long":   func(c *Config) { *c.list(n) = append(*c.list(n), "Z") },
			"unknown":    func(c *Config) { (*c.list(n))[0] = "ENTER" },
			"lowercase":  func(c *Config) { (*c.list(n))[0] = "a" },
			"empty list": func(c *Config) { *c.list(n) = []Key{} },
		}
		if n > 1 {
			edits["duplicate"] = func(c *Config) { (*c.list(n))[n-1] = (*c.list(n))[0] }
		}
		for name, edit := range edits {
			c := Defaults()
			edit(&c)
			if f, m := c.Check(); f != field || m == "" {
				t.Errorf("%dk %s: field = %q msg = %q", n, name, f, m)
			}
		}
	}
}

func TestLayoutValuesAndScratchValidation(t *testing.T) {
	for _, l := range []string{"auto", "1k", "2k", "3k", "4k", "5k", "6k", "7k", "8k", "9k", "10k"} {
		c := Defaults()
		c.Layout = l
		if f, _ := c.Check(); f != "" {
			t.Errorf("layout %q rejected on %s", l, f)
		}
	}
	for _, l := range []string{"0k", "11k", "4K", "k4", "4", "10", "-1k", " 4k", "auto ", "all"} {
		c := Defaults()
		c.Layout = l
		if f, _ := c.Check(); f != "keys.layout" {
			t.Errorf("layout %q accepted (field %q)", l, f)
		}
	}
	c := Defaults()
	c.Scratch = "ENTER"
	if f, _ := c.Check(); f != "keys.scratch" {
		t.Fatalf("field = %q", f)
	}
}

func TestEveryLayoutRoundTripsThroughJSON(t *testing.T) {
	c := Defaults()
	c.Enabled, c.Layout, c.Scratch = true, "10k", "CTRL_L"
	for n := 1; n <= MaxKeys; n++ {
		l := make([]Key, n)
		for i := range l {
			l[i] = Key(string(rune('A' + i)))
		}
		*c.list(n) = l
	}
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	var back Config
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(back.Normalized(), c) {
		t.Fatalf("round trip lost data: %s", data)
	}
	for _, name := range []string{`"k1"`, `"k2"`, `"k3"`, `"k4"`, `"k5"`, `"k6"`, `"k7"`, `"k8"`, `"k9"`, `"k10"`, `"scratch"`} {
		if !strings.Contains(string(data), name) {
			t.Errorf("%s missing from %s", name, data)
		}
	}
}

func TestOldConfigWithOnlyK4AndK7StillLoads(t *testing.T) {
	var c Config
	if err := json.Unmarshal([]byte(`{"enabled":true,"layout":"7k","k4":["Q","W","E","R"],"k7":["A","S","D","SPACE","J","K","L"]}`), &c); err != nil {
		t.Fatal(err)
	}
	n := c.Normalized()
	if f, m := n.Check(); f != "" {
		t.Fatalf("%s %s", f, m)
	}
	if !reflect.DeepEqual(n.Lanes(4), []Key{"Q", "W", "E", "R"}) || n.Layout != "7k" || !reflect.DeepEqual(n.Lanes(5), defaultKeys[5]) {
		t.Fatalf("config = %+v", n)
	}
}

func TestEngineWithTenAndElevenLanes(t *testing.T) {
	for _, tc := range []struct {
		lanes   int
		scratch bool
	}{{10, false}, {11, true}} {
		e := NewEngine("10k", tc.lanes, tc.scratch)
		down := make([]bool, tc.lanes)
		for i := range down {
			down[i] = true
		}
		f := e.Update(at(0), down)
		if f.Lanes != tc.lanes || f.Scratch != tc.scratch || len(f.Down) != tc.lanes || len(f.Presses) != tc.lanes || f.KPS != tc.lanes {
			t.Fatalf("frame = %+v", f)
		}
		f = e.Release(at(10))
		if slices.Contains(f.Down, true) || f.Presses[tc.lanes-1] != 1 {
			t.Fatalf("release = %+v", f)
		}
	}
}

func TestEngineWithoutLanesPublishesEmptyArrays(t *testing.T) {
	data, _ := json.Marshal(NewEngine("unknown", 0, false).Update(at(0), nil))
	if !strings.Contains(string(data), `"down":[]`) || !strings.Contains(string(data), `"presses":[]`) {
		t.Fatalf("frame = %s", data)
	}
}

func mapEvent(h *harness, keys int, scratch bool) {
	h.b.Publish(bus.TypeMapStarted, scores.Map{Keys: keys, Scratch: scratch})
}

func TestAutoLayoutFollowsEveryMapMode(t *testing.T) {
	for n := 1; n <= MaxKeys; n++ {
		for _, scratch := range []bool{false, true} {
			t.Run(fmt.Sprintf("%dk scratch=%v", n, scratch), func(t *testing.T) {
				h := start(t, enabled("auto"))
				mapEvent(h, n, scratch)
				f := h.begin(t)
				want := n
				if scratch {
					want++
				}
				if f.Layout != LayoutName(n) || f.Lanes != want || f.Scratch != scratch || len(f.Down) != want {
					t.Fatalf("frame = %+v", f)
				}
				asked := h.fake.asked
				base := Defaults().Lanes(n)
				if scratch {
					base = append([]Key{DefaultScratch}, base...)
				}
				if !slices.Equal(asked, base) {
					t.Fatalf("sampler was asked for %v, want %v", asked, base)
				}
			})
		}
	}
}

func TestScratchLaneCountsItsOwnKey(t *testing.T) {
	cfg := enabled("auto")
	cfg.Scratch = "CTRL_L"
	h := start(t, cfg)
	mapEvent(h, 7, true)
	h.begin(t)
	h.fake.set("CTRL_L", true)
	h.tick(t, 40*time.Millisecond)
	f := h.next(t)
	if !f.Down[0] || f.Presses[0] != 1 || f.Presses[1] != 0 || f.Lanes != 8 {
		t.Fatalf("frame = %+v", f)
	}
}

func TestHostileMapKeyCountsGiveNoLane(t *testing.T) {
	for _, n := range []int{0, -1, 11, 12, 1 << 30, -1 << 30} {
		h := start(t, enabled("auto"))
		mapEvent(h, n, true)
		if f := h.begin(t); f.Layout != "unknown" || f.Lanes != 0 || f.Scratch {
			t.Errorf("keys %d: frame = %+v", n, f)
		}
		if len(h.fake.asked) != 0 {
			t.Errorf("keys %d: sampler asked for %v", n, h.fake.asked)
		}
	}
}

func TestFixedLayoutIgnoresScratchOfTheMap(t *testing.T) {
	h := start(t, enabled("5k"))
	mapEvent(h, 7, true)
	if f := h.begin(t); f.Layout != "5k" || f.Lanes != 5 || f.Scratch {
		t.Fatalf("frame = %+v", f)
	}
}

func TestInvalidLayoutNeverSamples(t *testing.T) {
	h := start(t, enabled("99k"))
	mapEvent(h, 4, false)
	if f := h.begin(t); f.Layout != "unknown" || f.Lanes != 0 {
		t.Fatalf("frame = %+v", f)
	}
}

func TestScratchClashingWithTheLayoutInUseIsRejected(t *testing.T) {
	for n := 1; n <= MaxKeys; n++ {
		field := Defaults().Lanes(n)[0]
		c := Defaults()
		c.Layout, c.Scratch = LayoutName(n), field
		f, m := c.Check()
		if f != "keys.scratch" || m != ScratchClashMsg || strings.Contains(m, string(field)) {
			t.Errorf("%dk: field %q msg %q", n, f, m)
		}
		other := Defaults()
		other.Layout = LayoutName(n)
		other.Scratch = "F12"
		if f, _ := other.Check(); f != "" {
			t.Errorf("%dk with a free scratch key rejected on %s", n, f)
		}
	}
}

func TestScratchClashOnlyLooksAtTheActiveLayout(t *testing.T) {
	c := Defaults()
	c.Layout, c.Scratch = "4k", "V"
	if f, _ := c.Check(); f != "" {
		t.Fatalf("V is only a 10K key but 4k was rejected on %s", f)
	}
	c.Layout = "10k"
	if f, _ := c.Check(); f != "keys.scratch" {
		t.Fatalf("field = %q", f)
	}
	c.Layout = "auto"
	if f, _ := c.Check(); f != "keys.scratch" {
		t.Fatalf("auto must look at every layout, field = %q", f)
	}
}

func TestExplicitEmptyScratchIsRejectedButAbsentTakesTheDefault(t *testing.T) {
	read := func(doc string) Config {
		c := Defaults()
		if err := json.Unmarshal([]byte(doc), &c); err != nil {
			t.Fatal(err)
		}
		return c.Normalized()
	}
	if f, m := read(`{"scratch":""}`).Check(); f != "keys.scratch" || m == "" {
		t.Fatalf("explicit empty: %q %q", f, m)
	}
	for _, doc := range []string{`{}`, `{"scratch":null}`, `{"enabled":true}`} {
		c := read(doc)
		if c.ScratchKey() != DefaultScratch {
			t.Errorf("%s: scratch = %q", doc, c.Scratch)
		}
		if f, _ := c.Check(); f != "" {
			t.Errorf("%s rejected on %s", doc, f)
		}
	}
	var bare Config
	if err := json.Unmarshal([]byte(`{"scratch":""}`), &bare); err != nil {
		t.Fatal(err)
	}
	if f, _ := bare.Normalized().Check(); f != "keys.scratch" {
		t.Fatalf("zero config with explicit empty scratch: %q", f)
	}
	if err := json.Unmarshal([]byte(`{"scratch":5}`), &bare); err == nil {
		t.Fatal("a number is not a key name")
	}
}

func TestDefaultScratchClashingWithTheLaneListLetsTheListWin(t *testing.T) {
	cfg := enabled("auto")
	cfg.K7 = []Key{"SHIFT_L", "S", "D", "SPACE", "J", "K", "L"}
	if f, m := cfg.Check(); f != "" {
		t.Fatalf("config rejected: %s %s", f, m)
	}
	h := start(t, cfg)
	mapEvent(h, 7, true)
	f := h.begin(t)
	if f.Layout != "7k" || f.Lanes != 7 || f.Scratch || len(f.Down) != 7 {
		t.Fatalf("frame = %+v", f)
	}
	if !slices.Equal(h.fake.asked, cfg.K7) {
		t.Fatalf("sampler was asked for %v", h.fake.asked)
	}
	h.fake.set("SHIFT_L", true)
	h.tick(t, 40*time.Millisecond)
	if f := h.next(t); !f.Down[0] || f.Presses[0] != 1 {
		t.Fatalf("frame = %+v", f)
	}
}

func TestDefaultScratchStillWorksWhenItDoesNotClash(t *testing.T) {
	h := start(t, enabled("auto"))
	mapEvent(h, 7, true)
	if f := h.begin(t); f.Lanes != 8 || !f.Scratch {
		t.Fatalf("frame = %+v", f)
	}
}

func TestAbsentScratchIsNotWrittenBack(t *testing.T) {
	raw, _ := json.Marshal(Defaults())
	if strings.Contains(string(raw), "scratch") {
		t.Fatalf("json = %s", raw)
	}
}
