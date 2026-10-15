package procwatch

import (
	"errors"
	"unsafe"

	"golang.org/x/sys/windows"
)

type windowsLister struct{}

// NewLister returns the process lister of this system, or false when the check is unsupported.
func NewLister() (Lister, bool) { return windowsLister{}, true }

func (windowsLister) Names() ([]string, error) {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(snap)
	var e windows.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	err = windows.Process32First(snap, &e)
	var names []string
	for ; err == nil; err = windows.Process32Next(snap, &e) {
		names = append(names, windows.UTF16ToString(e.ExeFile[:]))
	}
	if !errors.Is(err, windows.ERROR_NO_MORE_FILES) {
		return nil, err
	}
	return names, nil
}
