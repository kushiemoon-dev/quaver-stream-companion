//go:build !windows

package keys

import (
	"runtime"
	"strings"
	"testing"
)

func TestUnsupportedMessageNamesThePlatform(t *testing.T) {
	_, err := NewSampler()
	if err == nil || !strings.Contains(err.Error(), runtime.GOOS) || !strings.Contains(err.Error(), "Windows") {
		t.Fatalf("err = %v", err)
	}
}
