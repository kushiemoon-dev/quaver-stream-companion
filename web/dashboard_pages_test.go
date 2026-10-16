package web

import "testing"

func TestDashboardRunsInASimulatedBrowser(t *testing.T) {
	for _, name := range []string{"shell", "core", "status"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			runNodeTest(t, "dashboard/"+name+".js")
		})
	}
}
