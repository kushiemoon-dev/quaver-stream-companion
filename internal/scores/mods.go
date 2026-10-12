package scores

type speedFlag struct {
	bit  uint
	rate float64
	col  string
}

// Ordered by bit, so the lowest set speed flag wins.
var speedFlags = []speedFlag{
	{1, 0.5, "Difficulty05X"}, {2, 0.6, "Difficulty06X"}, {3, 0.7, "Difficulty07X"}, {4, 0.8, "Difficulty08X"}, {5, 0.9, "Difficulty09X"}, {6, 1.1, "Difficulty11X"}, {7, 1.2, "Difficulty12X"}, {8, 1.3, "Difficulty13X"},
	{9, 1.4, "Difficulty14X"}, {10, 1.5, "Difficulty15X"}, {11, 1.6, "Difficulty16X"}, {12, 1.7, "Difficulty17X"}, {13, 1.8, "Difficulty18X"}, {14, 1.9, "Difficulty19X"}, {15, 2.0, "Difficulty20X"},
	{24, 0.55, "Difficulty055X"}, {25, 0.65, "Difficulty065X"}, {26, 0.75, "Difficulty075X"}, {27, 0.85, "Difficulty085X"}, {28, 0.95, "Difficulty095X"},
	{33, 1.05, "Difficulty105X"}, {34, 1.15, "Difficulty115X"}, {35, 1.25, "Difficulty125X"}, {36, 1.35, "Difficulty135X"}, {37, 1.45, "Difficulty145X"}, {38, 1.55, "Difficulty155X"},
	{39, 1.65, "Difficulty165X"}, {40, 1.75, "Difficulty175X"}, {41, 1.85, "Difficulty185X"}, {42, 1.95, "Difficulty195X"},
}

type modFlag struct {
	bit  uint
	name string
}

var modFlags = []modFlag{
	{0, "NoSV"}, {16, "Strict"}, {17, "Chill"}, {18, "NoPause"}, {19, "Autoplay"},
	{20, "Paused"}, {21, "NoFail"}, {22, "NoLN"}, {23, "Random"}, {29, "Inverse"},
	{30, "FullLN"}, {31, "Mirror"}, {32, "Coop"}, {43, "HealthAdjust"},
	{44, "NoMiss"}, {45, "NoMines"},
}

const (
	autoplayMod int64 = 1 << 19
	pausedMod   int64 = 1 << 20
)

type modClass int

const (
	modCounts modClass = iota + 1
	modIgnored
	modDisqualifying
)

// modClasses classifies every mod once for score comparison. Speed flags always count.
var modClasses = map[uint]modClass{
	0: modCounts, 16: modCounts, 17: modCounts, 18: modIgnored, 19: modDisqualifying,
	20: modDisqualifying, 21: modIgnored, 22: modCounts, 23: modCounts, 29: modCounts,
	30: modCounts, 31: modCounts, 32: modCounts, 43: modCounts,
	44: modCounts, 45: modCounts,
}

func classOf(bit uint) (modClass, bool) {
	for _, f := range speedFlags {
		if f.bit == bit {
			return modCounts, true
		}
	}
	c, ok := modClasses[bit]
	return c, ok
}

var disqualifyMask, keyMask = classMasks()

// classMasks: every bit counts by default, so unknown future bits join the key.
func classMasks() (disqualify, key int64) {
	var ignored int64
	for bit, c := range modClasses {
		switch c {
		case modIgnored:
			ignored |= 1 << bit
		case modDisqualifying:
			disqualify |= 1 << bit
		}
	}
	return disqualify, ^(ignored | disqualify)
}

func speedOf(mods int64) speedFlag {
	for _, f := range speedFlags {
		if mods&(1<<f.bit) != 0 {
			return f
		}
	}
	return speedFlag{rate: 1.0, col: "Difficulty10X"}
}

func decodeMods(mods int64) (rate float64, names []string) {
	names = []string{}
	for _, f := range modFlags {
		if mods&(1<<f.bit) != 0 {
			names = append(names, f.name)
		}
	}
	return speedOf(mods).rate, names
}
