//go:build windows

package keys

import "syscall"

var getAsyncKeyState = syscall.NewLazyDLL("user32.dll").NewProc("GetAsyncKeyState")

type winSampler struct{}

// Supported tells whether this platform can sample key state.
func Supported() bool { return true }

// NewSampler returns a sampler reading the key state through GetAsyncKeyState.
func NewSampler() (Sampler, error) {
	if err := getAsyncKeyState.Find(); err != nil {
		return nil, err
	}
	return winSampler{}, nil
}

func (winSampler) Down(keys []Key) []bool {
	out := make([]bool, len(keys))
	for i, k := range keys {
		if vk, ok := vkCodes[k]; ok {
			r, _, _ := getAsyncKeyState.Call(uintptr(vk))
			out[i] = r&0x8000 != 0
		}
	}
	return out
}
