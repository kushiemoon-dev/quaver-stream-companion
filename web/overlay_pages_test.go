package web

import (
	"os/exec"
	"testing"
)

// runNodeTest runs a script of web/testdata under node, which loads the real page code through web/testsupport/dom.js.
func runNodeTest(t *testing.T, script string) {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not on PATH")
	}
	out, err := exec.Command(node, mustAbs(t, "testdata/"+script)).CombinedOutput()
	if err != nil {
		t.Fatalf("%s failed: %v\n%s", script, err, out)
	}
}

func TestSimulatedBrowserStub(t *testing.T) {
	runNodeTest(t, "dom_stub.js")
}

func TestOverlayPagesRunInASimulatedBrowser(t *testing.T) {
	for _, name := range overlayPages {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			runNodeTest(t, "overlays/"+name+".js")
		})
	}
}
