//go:build !windows && !linux

package procwatch

import "testing"

func TestUnsupportedSystemHasNoLister(t *testing.T) {
	if l, ok := NewLister(); ok || l != nil {
		t.Fatal("the process check must be disabled on this system")
	}
}
