package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/testutil"
)

func TestTextFilesEnabledReadsTheFileEachTime(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), "config.json")
	on := textFilesEnabled(cfg)
	if on() {
		t.Fatal("a missing file means off")
	}
	for body, want := range map[string]bool{`{"text_files": true}`: true, `{"port": 7878}`: false, `{"text_files": false}`: false, `{not json`: false} {
		if err := os.WriteFile(cfg, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if on() != want {
			t.Fatalf("%s gave %v", body, !want)
		}
	}
}

func TestServeWritesTheTextFilesWhenEnabled(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	d.AddMap(t, testutil.MapRow{MD5: "m1", Artist: "Some Artist", Title: "Some Title", DiffName: "Hard", Mode: 1, LastPlayedMS: time.Now().Add(-2 * time.Hour).UnixMilli(), Difficulty: 15.7})
	d.AddScore(t, testutil.ScoreRow{MapMD5: "m1", Total: 900000, Grade: 7, Accuracy: 98.2, MaxCombo: 300})
	if err := os.WriteFile(filepath.Join(d.Path, "Logs", "runtime.log"), []byte("[1:00:00] - RUNTIME - IMPORTANT: Screen has been switched to type: `Results`\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(cfg, []byte(`{"text_files": true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runInBackground(t, ctx, cancel, []string{"--quaver-dir", d.Path, "--port", fmt.Sprint(freePort(t)), "--config", cfg})

	file := filepath.Join(filepath.Dir(cfg), "text", "nowplaying.txt")
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if b, _ := os.ReadFile(file); string(b) == "Some Artist - Some Title [Hard]" {
			acc, _ := os.ReadFile(filepath.Join(filepath.Dir(file), "last_accuracy.txt"))
			if string(acc) != "98.20%" {
				t.Fatalf("last_accuracy = %q", acc)
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	b, _ := os.ReadFile(file)
	t.Fatalf("nowplaying.txt = %q", b)
}

func TestServeLeavesNoTextFolderWhenDisabled(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	cfg := filepath.Join(t.TempDir(), "config.json")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	port := freePort(t)
	runInBackground(t, ctx, cancel, []string{"--quaver-dir", d.Path, "--port", fmt.Sprint(port), "--config", cfg})
	waitGame(t, port, "unknown")
	time.Sleep(200 * time.Millisecond)
	if _, err := os.Stat(filepath.Join(filepath.Dir(cfg), "text")); !os.IsNotExist(err) {
		t.Fatalf("the text folder exists with the setting off: %v", err)
	}
}
