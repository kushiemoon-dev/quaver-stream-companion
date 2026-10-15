//go:build !windows

package procwatch

import "runtime"

// NewLister returns the process lister of this system, or false when the check is unsupported.
func NewLister() (Lister, bool) {
	if runtime.GOOS == "linux" {
		return ProcFS{Root: "/proc"}, true
	}
	return nil, false
}
