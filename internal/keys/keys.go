// Package keys samples the configured lane keys while a map is played and turns them into
// per-lane counters and keys per second.
//
// Privacy contract: the sampler is off by default and read-only; it asks the system only
// about the configured lane keys, and only while the game state is gameplay. Key names never
// leave the configuration: only booleans per lane, counters and KPS are stored or emitted.
// The numbers come from raw key state, they are not note-accurate judgments.
package keys

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
)

// Key is a key name from the whitelist, such as "A", "SPACE" or "F5".
type Key string

// ErrUnsupported is returned by NewSampler on platforms without a key state source.
var ErrUnsupported = errors.New("lane key sampling is not supported on this platform")

// Sampler reports whether each of the given keys is down right now.
type Sampler interface {
	Down(keys []Key) []bool
}

// MaxKeys is the largest number of keys of a layout, 10K.
const MaxKeys = 10

// DefaultScratch is the key of the scratch lane when none is configured.
const DefaultScratch Key = "SHIFT_L"

// Config is the keys section of the settings file: one key list per layout, K1 to K10, with
// exactly that many keys, and the key of the scratch lane.
type Config struct {
	Enabled bool   `json:"enabled"`
	Layout  string `json:"layout"`
	K1      []Key  `json:"k1"`
	K2      []Key  `json:"k2"`
	K3      []Key  `json:"k3"`
	K4      []Key  `json:"k4"`
	K5      []Key  `json:"k5"`
	K6      []Key  `json:"k6"`
	K7      []Key  `json:"k7"`
	K8      []Key  `json:"k8"`
	K9      []Key  `json:"k9"`
	K10     []Key  `json:"k10"`
	// Scratch is the key of the scratch lane. Empty, and absent from the file, means DefaultScratch.
	Scratch Key `json:"scratch,omitempty"`
}

// list returns the key list of the n-key layout, or nil when n is out of range.
func (c *Config) list(n int) *[]Key {
	if n < 1 || n > MaxKeys {
		return nil
	}
	return []*[]Key{&c.K1, &c.K2, &c.K3, &c.K4, &c.K5, &c.K6, &c.K7, &c.K8, &c.K9, &c.K10}[n-1]
}

// Lanes returns a copy of the keys of the n-key layout, or nil when n is out of range.
func (c Config) Lanes(n int) []Key {
	if l := c.list(n); l != nil {
		return slices.Clone(*l)
	}
	return nil
}

// LayoutName returns the name of the n-key layout, such as "7k".
func LayoutName(n int) string { return strconv.Itoa(n) + "k" }

// vkCodes maps every accepted key name to its Windows virtual-key code.
var vkCodes = buildCodes()

func buildCodes() map[Key]uint16 {
	m := map[Key]uint16{
		"SPACE": 0x20, "LEFT": 0x25, "UP": 0x26, "RIGHT": 0x27, "DOWN": 0x28,
		"SHIFT_L": 0xA0, "SHIFT_R": 0xA1, "CTRL_L": 0xA2, "CTRL_R": 0xA3, "ALT_L": 0xA4, "ALT_R": 0xA5,
		";": 0xBA, ",": 0xBC, ".": 0xBE, "/": 0xBF, "'": 0xDE,
	}
	for c := 'A'; c <= 'Z'; c++ {
		m[Key(string(c))] = uint16(c)
	}
	for c := '0'; c <= '9'; c++ {
		m[Key(string(c))] = uint16(c)
	}
	for n := 1; n <= 12; n++ {
		m[Key("F"+strconv.Itoa(n))] = uint16(0x6F + n)
	}
	return m
}

// Valid tells whether name is an accepted key name.
func Valid(name Key) bool {
	_, ok := vkCodes[name]
	return ok
}

// defaultKeys holds the default bindings of Quaver itself for each mode, indexed by key count.
var defaultKeys = [MaxKeys + 1][]Key{
	1:  {"SPACE"},
	2:  {"A", "L"},
	3:  {"A", "SPACE", "L"},
	4:  {"A", "S", "K", "L"},
	5:  {"A", "S", "SPACE", "K", "L"},
	6:  {"A", "S", "D", "J", "K", "L"},
	7:  {"A", "S", "D", "SPACE", "J", "K", "L"},
	8:  {"A", "S", "D", "F", "H", "J", "K", "L"},
	9:  {"A", "S", "D", "F", "SPACE", "H", "J", "K", "L"},
	10: {"A", "S", "D", "F", "V", "B", "H", "J", "K", "L"},
}

// Defaults returns the settings used when none are saved: sampling off.
func Defaults() Config {
	c := Config{Layout: "auto"}
	return c.Normalized()
}

// Normalized returns a copy where an empty layout or an absent (nil) key
// list takes its default. An explicit empty list is kept so that Check rejects it.
func (c Config) Normalized() Config {
	if c.Layout == "" {
		c.Layout = "auto"
	}
	for n := 1; n <= MaxKeys; n++ {
		l := c.list(n)
		if *l == nil {
			*l = defaultKeys[n]
		}
		*l = slices.Clone(*l)
	}
	return c
}

// Check validates the settings and returns the first invalid field with a message, or "" "".
func (c Config) Check() (field, msg string) {
	if !validLayout(c.Layout) {
		return "keys.layout", "must be auto or a key count from 1k to 10k"
	}
	for n := 1; n <= MaxKeys; n++ {
		if m := checkList(*c.list(n), n); m != "" {
			return "keys.k" + strconv.Itoa(n), m
		}
	}
	if c.Scratch == "" {
		return "", ""
	}
	if !Valid(c.Scratch) {
		return "keys.scratch", "is not a supported key name"
	}
	if c.scratchClashes() {
		return "keys.scratch", ScratchClashMsg
	}
	return "", ""
}

// ScratchKey returns the key of the scratch lane, DefaultScratch when none is set.
func (c Config) ScratchKey() Key {
	if c.Scratch == "" {
		return DefaultScratch
	}
	return c.Scratch
}

// ScratchClashMsg is the error of a scratch key that is also a lane key of the layout in use.
const ScratchClashMsg = "must differ from the lane keys of the layout in use"

// scratchClashes tells whether the scratch key is one of the lane keys that can be active:
// those of the fixed layout, or of every layout with auto.
func (c Config) scratchClashes() bool {
	for n := 1; n <= MaxKeys; n++ {
		if (c.Layout == "auto" || c.Layout == LayoutName(n)) && slices.Contains(*c.list(n), c.Scratch) {
			return true
		}
	}
	return false
}

// emptyScratch marks an explicit empty "scratch" in a settings document: it is no key name, so
// Check rejects it, while an absent scratch keeps its default.
const emptyScratch Key = " "

// UnmarshalJSON reads the settings, keeping the current values of absent fields.
func (c *Config) UnmarshalJSON(data []byte) error {
	type plain Config
	p := plain(*c)
	if err := json.Unmarshal(data, &p); err != nil {
		return err
	}
	var probe struct {
		Scratch *Key `json:"scratch"`
	}
	if json.Unmarshal(data, &probe) == nil && probe.Scratch != nil && *probe.Scratch == "" {
		p.Scratch = emptyScratch
	}
	*c = Config(p)
	return nil
}

func validLayout(l string) bool {
	if l == "auto" {
		return true
	}
	for n := 1; n <= MaxKeys; n++ {
		if l == LayoutName(n) {
			return true
		}
	}
	return false
}

func checkList(list []Key, want int) string {
	if len(list) != want {
		return fmt.Sprintf("must list exactly %d keys", want)
	}
	seen := map[Key]bool{}
	for i, k := range list {
		if !Valid(k) {
			return fmt.Sprintf("entry %d is not a supported key name", i+1)
		}
		if seen[k] {
			return "must not list the same key twice"
		}
		seen[k] = true
	}
	return ""
}
