//go:build !windows

package keys

import (
	"fmt"
	"runtime"
)

// Supported tells whether this platform can sample key state.
func Supported() bool { return false }

// NewSampler returns ErrUnsupported naming the platform: only Windows is implemented for now.
func NewSampler() (Sampler, error) {
	return nil, fmt.Errorf("%w (%s has no key state source yet; Windows only for now)", ErrUnsupported, runtime.GOOS)
}
