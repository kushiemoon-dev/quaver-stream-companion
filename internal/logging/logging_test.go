package logging

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestRotationKeepsTwoFilesUnderTheLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "qsc.log")
	f, err := OpenRolling(path, 100)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		if _, err := fmt.Fprintf(f, "line %02d %s\n", i, strings.Repeat("x", 20)); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 2 {
		t.Fatalf("want qsc.log and qsc.log.1, got %d files", len(entries))
	}
	for _, name := range []string{"qsc.log", "qsc.log.1"} {
		fi, err := os.Stat(filepath.Join(filepath.Dir(path), name))
		if err != nil || fi.Size() > 100 {
			t.Fatalf("%s: %v size %v", name, err, fi)
		}
	}
	last, _ := os.ReadFile(path)
	if !strings.Contains(string(last), "line 19") {
		t.Fatalf("the newest line must be in qsc.log: %q", last)
	}
}

func TestRollingResumesAfterReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "qsc.log")
	for i := 0; i < 3; i++ {
		f, err := OpenRolling(path, 1<<20)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(f, "run %d\n", i)
		f.Close()
	}
	data, _ := os.ReadFile(path)
	if string(data) != "run 0\nrun 1\nrun 2\n" {
		t.Fatalf("log = %q", data)
	}
}

func TestLogFileIsPrivateWhereSupported(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits are not meaningful on Windows")
	}
	path := filepath.Join(t.TempDir(), "qsc.log")
	f, err := OpenRolling(path, 1000)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	if fi, _ := os.Stat(path); fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v", fi.Mode().Perm())
	}
}

func TestSetupFallsBackToTheConsoleWhenTheFileCannotBeOpened(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	var console bytes.Buffer
	log, closer := Setup(filepath.Join(blocker, "qsc.log"), false, &console)
	defer closer.Close()
	log.Info("hello")
	if !strings.Contains(console.String(), "hello") || !strings.Contains(console.String(), "log file") {
		t.Fatalf("console = %q", console.String())
	}
}

func TestSetupWritesToFileAndConsoleAndHonorsVerbose(t *testing.T) {
	for _, verbose := range []bool{false, true} {
		path := filepath.Join(t.TempDir(), "qsc.log")
		var console bytes.Buffer
		log, closer := Setup(path, verbose, &console)
		log.Debug("debug-line")
		log.Info("info-line")
		closer.Close()
		data, _ := os.ReadFile(path)
		if !strings.Contains(string(data), "info-line") || !strings.Contains(console.String(), "info-line") {
			t.Fatalf("info missing: file %q console %q", data, console.String())
		}
		if got := strings.Contains(string(data), "debug-line"); got != verbose {
			t.Fatalf("verbose=%v: debug in file = %v", verbose, got)
		}
	}
}

func TestTailReturnsTheLastLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "qsc.log")
	var b strings.Builder
	for i := 1; i <= 120; i++ {
		fmt.Fprintf(&b, "line %d\n", i)
	}
	os.WriteFile(path, []byte(b.String()), 0o600)
	got := Tail(path, 50)
	if len(got) != 50 || got[0] != "line 71" || got[49] != "line 120" {
		t.Fatalf("got %d lines: first %q last %q", len(got), got[0], got[len(got)-1])
	}
	if Tail(filepath.Join(t.TempDir(), "missing.log"), 50) != nil {
		t.Fatal("a missing file has no lines")
	}
}

func TestTailOfAHugeFileReadsOnlyTheEnd(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runtime.log")
	f, _ := os.Create(path)
	f.Write(bytes.Repeat([]byte("old line\n"), 200000))
	f.WriteString("the end\n")
	f.Close()
	got := Tail(path, 2)
	if len(got) != 2 || got[1] != "the end" {
		t.Fatalf("got %q", got)
	}
}

func TestFailedRollKeepsTheOlderCopyAndBacksOff(t *testing.T) {
	path := filepath.Join(t.TempDir(), "qsc.log")
	old := path + ".1"
	os.WriteFile(old, []byte("precious\n"), 0o600)
	calls := 0
	rename = func(a, b string) error { calls++; return os.ErrPermission }
	defer func() { rename = os.Rename }()
	f, _ := OpenRolling(path, 50)
	clock := time.Now()
	f.now = func() time.Time { return clock }
	for i := 0; i < 20; i++ {
		fmt.Fprintf(f, "line %02d %s\n", i, strings.Repeat("x", 20))
	}
	if calls != 1 {
		t.Fatalf("the roll was tried %d times within a minute", calls)
	}
	if data, _ := os.ReadFile(old); string(data) != "precious\n" {
		t.Fatalf("older copy lost: %q", data)
	}
	if data, _ := os.ReadFile(path); !strings.Contains(string(data), "line 19") || !strings.Contains(string(data), "line 00") {
		t.Fatal("nothing may be lost while the roll is blocked")
	}
	rename = os.Rename
	clock = clock.Add(2 * time.Minute)
	fmt.Fprintln(f, "after the backoff")
	f.Close()
	if data, _ := os.ReadFile(old); !strings.Contains(string(data), "line 00") {
		t.Fatalf("the roll must happen after the backoff: %q", data)
	}
}

func TestRollWithASecondHandleOpen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "qsc.log")
	f, _ := OpenRolling(path, 60)
	other, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	for i := 0; i < 10; i++ {
		fmt.Fprintf(f, "line %d %s\n", i, strings.Repeat("y", 20))
	}
	f.Close()
	if data, _ := os.ReadFile(path); !strings.Contains(string(data), "line 9") {
		t.Fatal("the newest line must always reach qsc.log")
	}
}

func TestEmptyPathLogsToTheConsoleOnly(t *testing.T) {
	wd, _ := os.Getwd()
	var console bytes.Buffer
	log, closer := Setup("", false, &console)
	log.Info("hi")
	closer.Close()
	if !strings.Contains(console.String(), "hi") {
		t.Fatal("no console output")
	}
	if _, err := os.Stat(filepath.Join(wd, "qsc.log")); err == nil {
		t.Fatal("qsc.log must not be created in the working directory")
	}
}
