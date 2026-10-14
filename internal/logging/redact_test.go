package logging

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRedactorIsAppliedToEveryLoggedValue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "qsc.log")
	var console bytes.Buffer
	redact := func(s string) string { return strings.ReplaceAll(s, "SECRETNAME", "~") }
	log, closer := Setup(path, false, &console, redact)

	log.Info("Quaver folder found", "dir", "C:/Users/SECRETNAME/Games", "err", os.ErrNotExist, "n", 3)
	closer.Close()

	data, _ := os.ReadFile(path)
	for name, text := range map[string]string{"file": string(data), "console": console.String()} {
		if strings.Contains(text, "SECRETNAME") || !strings.Contains(text, "C:/Users/~/Games") {
			t.Errorf("%s = %q", name, text)
		}
	}
}

func TestFileOnlyRecordsStayOffTheConsole(t *testing.T) {
	path := filepath.Join(t.TempDir(), "qsc.log")
	var console bytes.Buffer
	log, closer := Setup(path, false, &console)

	log.Error("cannot listen", FileOnly, "addr", "127.0.0.1:1")
	log.Info("visible")
	closer.Close()

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "cannot listen") || !strings.Contains(string(data), "visible") {
		t.Fatalf("file = %q", data)
	}
	if strings.Contains(console.String(), "cannot listen") || !strings.Contains(console.String(), "visible") {
		t.Fatalf("console = %q", console.String())
	}
}

func TestRedactorSeesTheTextOfAnError(t *testing.T) {
	var console bytes.Buffer
	redact := func(s string) string { return strings.ReplaceAll(s, "SECRETNAME", "~") }
	log, closer := Setup("", false, &console, redact)
	defer closer.Close()

	log.Warn("cannot open", "err", errors.New("open C:/Users/SECRETNAME/x: access denied"))

	if strings.Contains(console.String(), "SECRETNAME") || !strings.Contains(console.String(), "C:/Users/~/x") {
		t.Fatalf("console = %q", console.String())
	}
}

func TestATypedNilErrorDoesNotPanicTheLogger(t *testing.T) {
	var console bytes.Buffer
	log, closer := Setup("", false, &console, func(s string) string { return s })
	defer closer.Close()
	var e *os.PathError
	var err error = e

	log.Warn("typed nil", "err", err)

	if !strings.Contains(console.String(), "typed nil") {
		t.Fatalf("console = %q", console.String())
	}
}
