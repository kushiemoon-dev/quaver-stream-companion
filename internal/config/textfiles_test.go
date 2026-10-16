package config

import (
	"os"
	"strings"
	"testing"
)

func TestTextFilesIsOffByDefaultAndRoundTrips(t *testing.T) {
	if Defaults().TextFiles {
		t.Fatal("text_files must default to off")
	}
	p := write(t, `{"port": 9000}`)
	c, err := Load(p)
	if err != nil || c.TextFiles {
		t.Fatalf("got %v, %v", c.TextFiles, err)
	}
	c.TextFiles = true
	if err := Save(p, c); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(p)
	if !strings.Contains(string(raw), `"text_files": true`) {
		t.Fatalf("file = %s", raw)
	}
	again, err := Load(p)
	if err != nil || !again.TextFiles {
		t.Fatalf("reloaded %v, %v", again.TextFiles, err)
	}
}
