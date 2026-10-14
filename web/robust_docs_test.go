package web

import (
	"strings"
	"testing"
)

func TestInstallExplainsRestoringTheQuaverDatabase(t *testing.T) {
	src := readDoc(t, "docs/INSTALL.md")
	for _, want := range []string{"## Troubleshooting", "restore", "quaver.db", "`qsc stop`", "read-only", "Quaver is writing the database"} {
		if !strings.Contains(src, want) {
			t.Errorf("INSTALL.md troubleshooting must mention %q", want)
		}
	}
}

func TestInstallStatesTheRecheckDelayAndTheKeysWatchdog(t *testing.T) {
	src := readDoc(t, "docs/INSTALL.md")
	for _, want := range []string{"within about 15 seconds", "15 minutes"} {
		if !strings.Contains(src, want) {
			t.Errorf("INSTALL.md must say %q", want)
		}
	}
	if strings.Contains(src, "within a few seconds") {
		t.Error("the database is rechecked every 15 seconds, not every few")
	}
}
