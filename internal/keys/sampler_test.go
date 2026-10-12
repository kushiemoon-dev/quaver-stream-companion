package keys

import (
	"errors"
	"testing"
)

func TestNewSamplerMatchesPlatformSupport(t *testing.T) {
	s, err := NewSampler()
	if Supported() {
		if err != nil || s == nil {
			t.Fatalf("supported platform: sampler = %v err = %v", s, err)
		}
		return
	}
	if s != nil || !errors.Is(err, ErrUnsupported) {
		t.Fatalf("unsupported platform: sampler = %v err = %v", s, err)
	}
}
