// Package keys samples the configured lane keys while a map is played and turns them into
// per-lane counters and keys per second.
//
// Privacy contract: the sampler is off by default and read-only; it asks the system only
// about the configured lane keys, and only while the game state is gameplay. Key names never
// leave the configuration: only booleans per lane, counters and KPS are stored or emitted.
// The numbers come from raw key state, they are not note-accurate judgments.
package keys

import (
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

// Config is the keys section of the settings file.
type Config struct {
	Enabled bool   `json:"enabled"`
	Layout  string `json:"layout"`
	K4      []Key  `json:"k4"`
	K7      []Key  `json:"k7"`
}

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

// Defaults returns the settings used when none are saved: sampling off.
func Defaults() Config {
	return Config{
		Layout: "auto",
		K4:     []Key{"A", "S", "K", "L"},
		K7:     []Key{"A", "S", "D", "SPACE", "J", "K", "L"},
	}
}

// Normalized returns a copy where an empty layout or an absent (nil) key list takes its default.
// An explicit empty list is kept so that Check rejects it.
func (c Config) Normalized() Config {
	d := Defaults()
	if c.Layout == "" {
		c.Layout = d.Layout
	}
	if c.K4 == nil {
		c.K4 = d.K4
	}
	if c.K7 == nil {
		c.K7 = d.K7
	}
	c.K4, c.K7 = slices.Clone(c.K4), slices.Clone(c.K7)
	return c
}

// Check validates the settings and returns the first invalid field with a message, or "" "".
func (c Config) Check() (field, msg string) {
	switch c.Layout {
	case "auto", "4k", "7k":
	default:
		return "keys.layout", "must be one of auto, 4k, 7k"
	}
	if m := checkList(c.K4, 4); m != "" {
		return "keys.k4", m
	}
	if m := checkList(c.K7, 7); m != "" {
		return "keys.k7", m
	}
	return "", ""
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
