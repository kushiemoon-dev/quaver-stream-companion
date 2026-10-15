package packaging

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestBuildScriptsDisableVCSStamping(t *testing.T) {
	for _, rel := range []string{"packaging/build-archive.sh", "packaging/build-appimage.sh"} {
		if !strings.Contains(read(t, rel), "-trimpath -buildvcs=false") {
			t.Errorf("%s must build with -trimpath -buildvcs=false", rel)
		}
	}
}

func TestDocsStateLinuxAndMacOSAccurately(t *testing.T) {
	readme := read(t, "README.md")
	if strings.Contains(readme, "Login Items") {
		t.Error("README must not describe macOS autostart: macOS is not supported yet")
	}
	if !strings.Contains(readme, "Not supported yet") {
		t.Error("README must say macOS is not supported yet")
	}
	install := read(t, "docs/INSTALL.md")
	if strings.Contains(install, "Quaver is a Windows game") {
		t.Error("INSTALL.md must not say Quaver is only a Windows game")
	}
	for _, want := range []string{"native Linux build", "steamapps/common/Quaver", "not been run on a real Linux machine"} {
		if !strings.Contains(install, want) {
			t.Errorf("INSTALL.md lacks %q", want)
		}
	}
}

func TestAppRunDispatch(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("AppRun runs on Linux")
	}
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh")
	}
	home := t.TempDir()
	bin := filepath.Join(home, "usr", "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	fake := "#!/bin/sh\nfor a in \"$@\"; do printf '[%s]' \"$a\"; done\nexit \"${FAKE_EXIT:-0}\"\n"
	if err := os.WriteFile(filepath.Join(bin, "qsc"), []byte(fake), 0o755); err != nil {
		t.Fatal(err)
	}
	apprun := filepath.Join(home, "AppRun")
	if err := os.WriteFile(apprun, []byte(read(t, "packaging/appimage/AppRun")), 0o755); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"no arguments", nil, "[serve][--open]"},
		{"flags only", []string{"--port", "9000"}, "[serve][--open][--port][9000]"},
		{"doctor", []string{"doctor"}, "[doctor]"},
		{"doctor with flags", []string{"doctor", "--port", "9000"}, "[doctor][--port][9000]"},
		{"stop", []string{"stop"}, "[stop]"},
		{"version", []string{"version"}, "[version]"},
		{"--version", []string{"--version"}, "[--version]"},
		{"help", []string{"help"}, "[help]"},
		{"-h", []string{"-h"}, "[-h]"},
		{"--help", []string{"--help"}, "[--help]"},
		{"explicit serve", []string{"serve", "--quaver-dir", "/data"}, "[serve][--quaver-dir][/data]"},
		{"path with spaces stays one argument", []string{"--quaver-dir", "/my games/Quaver  x"}, "[serve][--open][--quaver-dir][/my games/Quaver  x]"},
		{"double dash is not a command", []string{"--", "doctor"}, "[serve][--open][--][doctor]"},
		{"unknown first argument is passed to serve", []string{"bogus"}, "[serve][--open][bogus]"},
		{"empty first argument", []string{""}, "[serve][--open][]"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, err := exec.Command(sh, append([]string{apprun}, c.args...)...).CombinedOutput()
			if err != nil || string(out) != c.want {
				t.Errorf("output = %q, %v; want %q", out, err, c.want)
			}
		})
	}
	t.Run("exit code passes through", func(t *testing.T) {
		for _, args := range [][]string{{"doctor"}, nil} {
			cmd := exec.Command(sh, append([]string{apprun}, args...)...)
			cmd.Env = append(os.Environ(), "FAKE_EXIT=3")
			err := cmd.Run()
			var ee *exec.ExitError
			if !errors.As(err, &ee) || ee.ExitCode() != 3 {
				t.Errorf("args %v: err = %v, want exit code 3", args, err)
			}
		}
	})
}
