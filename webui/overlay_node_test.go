package webui

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestOverlayNode checks the scripts of the page with node: that they
// parse, and that the overlay keeps what it doesn't change of index.js.
func TestOverlayNode(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not found in PATH")
	}
	for _, args := range [][]string{
		{"--check", "assets/index.js"},
		{"--check", "overlay/overlay.js"},
		{"--test", "overlay/overlay_test.mjs"},
	} {
		out, err := exec.Command(node, args...).CombinedOutput()
		t.Logf("node %v:\n%s", args, out)
		assert.NoError(t, err, "node %v failed:\n%s", args, out)
	}
}
