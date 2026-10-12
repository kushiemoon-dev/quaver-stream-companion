//go:build !windows

package keys

// Supported tells whether this platform can sample key state.
func Supported() bool { return false }

// NewSampler returns ErrUnsupported: only Windows is implemented for now.
func NewSampler() (Sampler, error) { return nil, ErrUnsupported }
