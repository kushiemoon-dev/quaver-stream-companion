package web

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"
)

func readWeb(t *testing.T, name string) string {
	t.Helper()
	data, err := fs.ReadFile(FS, name)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestAlertOverlayNeverReplaysOldScores(t *testing.T) {
	src := readWeb(t, "overlays/alert/index.html")
	if strings.Contains(src, "snapshot") {
		t.Error("alert must ignore the snapshot so a connect never replays an old score")
	}
	for _, want := range []string{"'score.saved'", "qscModel.alertView", "qscAutohide(", "DEFAULT_SECONDS = 8", "qscT('alert.' + k)", "card hidden"} {
		if !strings.Contains(src, want) {
			t.Errorf("alert is missing %q", want)
		}
	}
}

func TestAlertSoundIsSynthesizedAndOptIn(t *testing.T) {
	src := readWeb(t, "overlays/alert/index.html")
	for _, want := range []string{"AudioContext", "params.sound", "createOscillator", ".resume()"} {
		if !strings.Contains(src, want) {
			t.Errorf("alert is missing %q", want)
		}
	}
	if regexp.MustCompile(`(?i)<audio|new Audio|\.(mp3|wav|ogg|flac)\b`).MatchString(src) {
		t.Error("alert must not use audio files")
	}
	demo := src[strings.Index(src, "if (params.demo) {"):]
	demo = demo[:strings.Index(demo, "} else {")]
	if strings.Contains(demo, "chime(") {
		t.Error("the demo must stay silent")
	}
	common := readWeb(t, "overlays/common.js")
	if !strings.Contains(common, "sound: params.get('sound') === '1'") || !strings.Contains(common, "best: params.get('best') === '1'") {
		t.Error("common.js must expose the sound and best flags, off unless the value is 1")
	}
}

func TestHistoryBestLineIsOptIn(t *testing.T) {
	src := readWeb(t, "overlays/history/index.html")
	if !strings.Contains(src, "params.best") || !strings.Contains(src, "qscModel.bestLine(") {
		t.Error("history must draw the best line only when params.best is set")
	}
	if !strings.Contains(readWeb(t, "overlays/overlay.css"), ".curve .best") {
		t.Error("overlay.css must style the best line")
	}
}
