package web

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

func quitKeys(t *testing.T, lang string) map[string]string {
	t.Helper()
	var all map[string]string
	if err := json.Unmarshal([]byte(readWeb(t, "dashboard/i18n/"+lang+".json")), &all); err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for k, v := range all {
		if strings.HasPrefix(k, "settings.quit.") {
			out[k] = v
		}
	}
	return out
}

func TestSettingsQuitSectionSource(t *testing.T) {
	src := readWeb(t, "dashboard/pages/settings-quit.js")
	for _, want := range []string{"/api/quit", "qsc.api.post", "aria-live", "role: 'alert'", "'Escape'", "tabindex", "title.focus()", "aria-busy", "settings.quit.stopped", "settings.quit.confirm", "settings.quit.cancel", "settings.quit.now"} {
		if !strings.Contains(src, want) {
			t.Errorf("settings-quit.js is missing %q", want)
		}
	}
	for _, bad := range []string{".innerHTML", ".outerHTML", "insertAdjacentHTML", "document.write", "eval(", "new Function", "confirm(", "alert(", "prompt("} {
		if strings.Contains(src, bad) {
			t.Errorf("settings-quit.js uses %s", bad)
		}
	}
	if lines := strings.Count(src, "\n"); lines > 300 {
		t.Errorf("settings-quit.js has %d lines", lines)
	}
}

func TestSettingsPageLoadsAndBuildsTheQuitSection(t *testing.T) {
	index := readWeb(t, "dashboard/index.html")
	quit := strings.Index(index, `src="pages/settings-quit.js"`)
	page := strings.Index(index, `src="pages/settings.js"`)
	if quit < 0 || quit > page {
		t.Error("settings-quit.js must be loaded before settings.js")
	}
	settings := readWeb(t, "dashboard/pages/settings.js")
	info := strings.Index(settings, "buildInfo()")
	built := strings.Index(settings, "qscSettingsQuit.build")
	if built < 0 || built < info {
		t.Error("the quit section must be built after the other settings cards")
	}
	if !strings.Contains(readWeb(t, "dashboard/api.js"), "post: function") {
		t.Error("api.js must expose post")
	}
}

func TestQuitStringsExistInEveryLanguageAndAreTranslated(t *testing.T) {
	en := quitKeys(t, "en")
	src := readWeb(t, "dashboard/pages/settings-quit.js")
	for _, m := range regexp.MustCompile(`'(settings\.quit\.[a-z_]+)'`).FindAllStringSubmatch(src, -1) {
		if _, ok := en[m[1]]; !ok {
			t.Errorf("key %s is used but missing from en.json", m[1])
		}
	}
	if len(en) < 8 {
		t.Fatalf("only %d quit strings in en.json", len(en))
	}
	for _, lang := range []string{"fr", "de"} {
		got := quitKeys(t, lang)
		for k, v := range en {
			tr, ok := got[k]
			switch {
			case !ok:
				t.Errorf("%s.json is missing %s", lang, k)
			case tr == v && k != "settings.quit.button" && k != "settings.quit.title":
				t.Errorf("%s.json: %s is not translated", lang, k)
			case strings.Contains(v, "{message}") != strings.Contains(tr, "{message}"):
				t.Errorf("%s.json: %s placeholder mismatch", lang, k)
			}
		}
	}
}

func TestDocsExplainHowToStop(t *testing.T) {
	readme := readDoc(t, "README.md")
	if !strings.Contains(readme, "qsc stop") {
		t.Error("README.md commands must list qsc stop")
	}
	install := readDoc(t, "docs/INSTALL.md")
	for _, want := range []string{"## Stopping qsc", "Quit qsc", "qsc stop", "Ctrl+C", "systemctl --user stop qsc", "does not restart", "Startup"} {
		if !strings.Contains(install, want) {
			t.Errorf("INSTALL.md does not mention %q", want)
		}
	}
}
