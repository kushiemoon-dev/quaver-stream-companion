package gamestate

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

const (
	seedMenu   = "[1:00:00] - RUNTIME - IMPORTANT: Screen has been switched to type: `Menu`"
	seedSelect = "[1:01:00] - RUNTIME - IMPORTANT: Screen has been switched to type: `Select`"
	seedExit   = "[3:31:14] - RUNTIME - IMPORTANT: Discord RPC has shutdown"
	seedScore  = "[1:06:06] - RUNTIME - IMPORTANT: Successfully saved score to database with id: 78"
	seedReplay = "[1:06:07] - RUNTIME - IMPORTANT: Successfully saved replay to path: C:\r\a.qr"
)

func writeLog(t *testing.T, lines ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "runtime.log")
	var s string
	for _, l := range lines {
		s += l + "\n"
	}
	if err := os.WriteFile(path, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLastStateLineReturnsLastState(t *testing.T) {
	path := writeLog(t, seedMenu, "noise", seedSelect, "more noise")
	got, ok := LastStateLine(path, time.Hour)
	if !ok || got != seedSelect {
		t.Fatalf("got %q %v", got, ok)
	}
}

func TestLastStateLineIgnoresScoreAndReplay(t *testing.T) {
	path := writeLog(t, seedMenu, seedScore, seedReplay)
	got, ok := LastStateLine(path, time.Hour)
	if !ok || got != seedMenu {
		t.Fatalf("got %q %v", got, ok)
	}
	path = writeLog(t, seedScore, seedReplay)
	if got, ok := LastStateLine(path, time.Hour); ok {
		t.Fatalf("got %q", got)
	}
}

func TestLastStateLineReturnsExitLine(t *testing.T) {
	path := writeLog(t, seedMenu, seedSelect, seedScore, seedExit)
	got, ok := LastStateLine(path, time.Hour)
	if !ok || got != seedExit {
		t.Fatalf("got %q %v", got, ok)
	}
}

func TestLastStateLineNoStateLine(t *testing.T) {
	if _, ok := LastStateLine(writeLog(t, "noise"), time.Hour); ok {
		t.Fatal("expected false")
	}
}

func TestLastStateLineMissingFile(t *testing.T) {
	if _, ok := LastStateLine(filepath.Join(t.TempDir(), "nope.log"), time.Hour); ok {
		t.Fatal("expected false")
	}
}

func TestLastStateLineTooOld(t *testing.T) {
	path := writeLog(t, seedMenu)
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	if _, ok := LastStateLine(path, time.Hour); ok {
		t.Fatal("expected false")
	}
}
