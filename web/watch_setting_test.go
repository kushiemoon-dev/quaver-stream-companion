package web

import (
	"io/fs"
	"os"
	"strings"
	"testing"
)

func TestSettingsPageHasTheProcessCheckboxWithItsDisclosure(t *testing.T) {
	data, err := fs.ReadFile(FS, "dashboard/pages/settings.js")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	for _, want := range []string{"id: 'set-watch'", "type: 'checkbox'", "settings.watch_help", "set-watch-help", "watch_quaver_process"} {
		if !strings.Contains(src, want) {
			t.Errorf("settings.js lacks %s", want)
		}
	}
	for _, lang := range []string{"en", "fr", "de"} {
		cat, err := fs.ReadFile(FS, "dashboard/i18n/"+lang+".json")
		if err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{`"settings.watch"`, `"settings.watch_help"`} {
			if !strings.Contains(string(cat), key) {
				t.Errorf("%s lacks %s", lang, key)
			}
		}
	}
}

func TestDocsDescribeTheProcessCheck(t *testing.T) {
	for file, wants := range map[string][]string{
		"../README.md":        {"### Crash detection", "names of the running programs", "watch_quaver_process", "`Quaver.exe`", "nothing", "3 checks in a row"},
		"../docs/INSTALL.md":  {"watch_quaver_process", "/proc"},
		"../docs/OVERLAYS.md": {"no Quaver process"},
	} {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, w := range wants {
			if !strings.Contains(string(data), w) {
				t.Errorf("%s lacks %q", file, w)
			}
		}
	}
}
