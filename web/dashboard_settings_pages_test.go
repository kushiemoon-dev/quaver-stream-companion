package web

import "testing"

func TestSettingsHistoryAndObsPagesRunInASimulatedBrowser(t *testing.T) {
	for _, name := range []string{"settings", "settings-parts", "history", "obs"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			runNodeTest(t, "dashboard/"+name+".js")
		})
	}
}

func TestSharedOverlayHelpersRunInASimulatedBrowser(t *testing.T) {
	runNodeTest(t, "overlays/common.js")
}
