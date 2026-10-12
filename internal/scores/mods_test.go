package scores

import (
	"reflect"
	"testing"
)

func TestDecodeMods(t *testing.T) {
	for _, tc := range []struct {
		name  string
		mods  int64
		rate  float64
		names []string
	}{
		{"none", 0, 1.0, []string{}},
		{"1.15x", 1 << 34, 1.15, []string{}},
		{"paused", 1 << 20, 1.0, []string{"Paused"}},
		{"1.1x", 64, 1.1, []string{}},
		{"1.2x", 128, 1.2, []string{}},
		{"0.9x", 32, 0.9, []string{}},
		{"0.55x", 1 << 24, 0.55, []string{}},
		{"2.0x", 1 << 15, 2.0, []string{}},
		{"rate and paused", 1<<34 | 1<<20, 1.15, []string{"Paused"}},
		{"several flags", 1<<0 | 1<<17 | 1<<21 | 1<<45, 1.0, []string{"NoSV", "Chill", "NoFail", "NoMines"}},
		{"two speeds, lowest bit", 1<<7 | 1<<34, 1.2, []string{}},
		{"unknown high bit", 1 << 60, 1.0, []string{}},
		{"unknown bit with known", 1<<60 | 1<<31, 1.0, []string{"Mirror"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rate, names := decodeMods(tc.mods)
			if rate != tc.rate || !reflect.DeepEqual(names, tc.names) {
				t.Fatalf("decodeMods(%d) = %v %v, want %v %v", tc.mods, rate, names, tc.rate, tc.names)
			}
		})
	}
}

func TestEveryModHasOneClass(t *testing.T) {
	speed := map[uint]bool{}
	for _, f := range speedFlags {
		speed[f.bit] = true
		if c, ok := classOf(f.bit); !ok || c != modCounts {
			t.Errorf("speed flag %s (bit %d) class = %v, want counts", f.col, f.bit, c)
		}
	}
	for _, f := range modFlags {
		if c, ok := modClasses[f.bit]; !ok || c < modCounts || c > modDisqualifying {
			t.Errorf("mod %s (bit %d) has no valid class", f.name, f.bit)
		}
		if speed[f.bit] {
			t.Errorf("bit %d is both a speed flag and a mod", f.bit)
		}
	}
	if len(modClasses) != len(modFlags) {
		t.Errorf("modClasses has %d entries, modFlags %d", len(modClasses), len(modFlags))
	}
}

func TestMasks(t *testing.T) {
	const ignoredBits, dq = int64(1<<21 | 1<<18), int64(1<<20 | 1<<19)
	if disqualifyMask != dq {
		t.Fatalf("disqualifyMask = %b", disqualifyMask)
	}
	if keyMask != ^(ignoredBits | dq) {
		t.Fatalf("keyMask = %b", keyMask)
	}
	if keyMask&(1<<50) == 0 || keyMask&(1<<34) == 0 {
		t.Fatal("unknown and speed bits must be in the key")
	}
}
