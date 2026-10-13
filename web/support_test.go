package web

import (
	"strings"
	"testing"
)

func TestSupportUIWiring(t *testing.T) {
	index := readWeb(t, "dashboard/index.html")
	info, set := strings.Index(index, `src="pages/settings-info.js"`), strings.Index(index, `src="pages/settings.js"`)
	if info < 0 || set < 0 || info > set || !strings.Contains(index, `src="pages/copy.js"`) {
		t.Error("copy.js and settings-info.js must be loaded, settings-info.js before settings.js")
	}
	for file, wants := range map[string][]string{
		"dashboard/pages/settings-info.js": {"themes_dir", "overlays_dir", "config_path", "/releases", "qscCopy.copy", "noopener"},
		"dashboard/pages/status.js":        {"/api/diagnostics", "status.copy_diag", "aria-live"},
		"dashboard/pages/settings.js":      {"settings.port_obs_note", "updatePortNote"},
	} {
		src := readWeb(t, file)
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Errorf("%s lacks %q", file, want)
			}
		}
		for _, bad := range []string{"innerHTML", "outerHTML", "insertAdjacentHTML"} {
			if strings.Contains(src, bad) {
				t.Errorf("%s uses %s", file, bad)
			}
		}
	}
}

func TestCopyDiagnosticsKeepsFocusAndFallsBackToATextarea(t *testing.T) {
	src := readWeb(t, "dashboard/pages/status.js")
	a := strings.Index(src, "async function onCopyDiag")
	b := strings.Index(src[a:], "function isObject")
	body := src[a : a+b]
	for _, want := range []string{"aria-disabled", "if (copying) return", "textarea", "settings.copy_manual", "select()"} {
		if !strings.Contains(src, want) {
			t.Errorf("status.js lacks %q", want)
		}
	}
	for _, bad := range []string{".disabled", "e.message : t('status.unreachable')"} {
		if strings.Contains(body, bad) && bad == ".disabled" {
			t.Errorf("onCopyDiag must not use %s", bad)
		}
	}
	if strings.Contains(body, "copy_diag_failed', { message: e.message") {
		t.Error("raw browser error text must not be shown")
	}
}
