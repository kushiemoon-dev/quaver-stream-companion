package web

import "testing"

func TestOverlaysPageRunsInASimulatedBrowser(t *testing.T) {
	for _, name := range []string{"overlays", "overlay-units"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			runNodeTest(t, "dashboard/"+name+".js")
		})
	}
}
