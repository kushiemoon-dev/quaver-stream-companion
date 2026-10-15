package config

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
)

// FormatVersion is the version written to the settings file. A file without one is version 1.
const FormatVersion = 1

type object = map[string]json.RawMessage

// syncFile flushes a file to disk; tests replace it.
var syncFile = func(f *os.File) error { return f.Sync() }

// encode renders c as the new file content. Keys that the file at path holds and this version
// does not know, also inside nested objects, are kept, and a newer format version is not lowered.
func encode(path string, c Config) ([]byte, error) {
	fresh, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	var next object
	if err := json.Unmarshal(fresh, &next); err != nil {
		return nil, err
	}
	old := readObject(path)
	merged := mergeObjects(old, next)
	merged["version"] = json.RawMessage("1")
	var v int
	if json.Unmarshal(old["version"], &v) == nil && v > FormatVersion {
		merged["version"] = old["version"]
	}
	return json.MarshalIndent(merged, "", "  ")
}

// readObject returns the top-level object of the file at path, or nil when it is missing or not an object.
func readObject(path string) object {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var o object
	if json.Unmarshal(bytes.TrimPrefix(data, []byte("\xEF\xBB\xBF")), &o) != nil {
		return nil
	}
	return o
}

// mergeObjects returns old with every key of next set on top; objects present in both are merged the same way.
// A key of old that only differs from a key of next by case folding is dropped: the JSON decoder reads
// both as the same field and the later one would win on the next Load.
func mergeObjects(old, next object) object {
	out := make(object, len(old)+len(next))
	for k, v := range old {
		if !foldsToAnother(k, next) {
			out[k] = v
		}
	}
	for k, v := range next {
		var a, b object
		if json.Unmarshal(out[k], &a) == nil && a != nil && json.Unmarshal(v, &b) == nil && b != nil {
			if m, err := json.Marshal(mergeObjects(a, b)); err == nil {
				v = m
			}
		}
		out[k] = v
	}
	return out
}

// foldsToAnother reports whether k equals a different key of o under Unicode case folding.
func foldsToAnother(k string, o object) bool {
	for other := range o {
		if other != k && strings.EqualFold(other, k) {
			return true
		}
	}
	return false
}
