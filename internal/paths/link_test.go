package paths

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func fakeQuaver(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Quaver.exe"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func makeLink(t *testing.T, link, target string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput(); err != nil {
			t.Skipf("cannot create a junction: %v %s", err, out)
		}
		return
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("cannot create a symlink: %v", err)
	}
}

func TestIsQuaverDirAcceptsLinkToQuaverFolderAndRefusesOtherLinks(t *testing.T) {
	link := filepath.Join(t.TempDir(), "link")
	makeLink(t, link, fakeQuaver(t))
	if !IsQuaverDir(link) {
		t.Error("a link to a Quaver folder should be accepted")
	}
	empty := filepath.Join(t.TempDir(), "empty-link")
	makeLink(t, empty, t.TempDir())
	if IsQuaverDir(empty) {
		t.Error("a link to a folder without Quaver.exe must be refused")
	}
}

func TestNetworkTargetIsRefusedWithInjectedResolver(t *testing.T) {
	local := fakeQuaver(t)
	target := uncPath("invalid-host-xyz", "share", "Quaver")
	old := evalSymlinks
	evalSymlinks = func(p string) (string, error) {
		if p == local {
			return target, nil
		}
		return old(p)
	}
	t.Cleanup(func() { evalSymlinks = old })
	if IsQuaverDir(local) {
		t.Error("IsQuaverDir accepted a folder that resolves to a network path")
	}
	if !ResolvesToUNC(local) {
		t.Error("ResolvesToUNC missed the network target")
	}
	if _, err := Find(local, nil); err == nil {
		t.Error("Find accepted a folder that resolves to a network path")
	}
	if ResolvesToUNC(fakeQuaver(t)) {
		t.Error("a plain local folder is not a network path")
	}
}
