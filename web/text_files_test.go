package web

import (
	"io/fs"
	"os"
	"strings"
	"testing"
)

func TestSettingsPageHasTheTextFilesCheckbox(t *testing.T) {
	data, err := fs.ReadFile(FS, "dashboard/pages/settings.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"id: 'set-text'", "set-text-help", "text_files"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("settings.js lacks %s", want)
		}
	}
}

func TestDocsDescribeTheTextFiles(t *testing.T) {
	for file, wants := range map[string][]string{
		"../docs/OBS.md":     {"## Text source, read from file", "nowplaying.txt", "last_accuracy.txt", "Read from file", "256 bytes", "/api/text/nowplaying.txt"},
		"../docs/INSTALL.md": {"`text` folder", "Write the now playing text files"},
		"../README.md":       {"`text_files`", "docs/OBS.md#text-source-read-from-file", "GET /api/text/{file}"},
		"../CHANGELOG.md":    {"`text_files`", "GET /api/text/{file}"},
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
