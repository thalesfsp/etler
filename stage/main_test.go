package stage

import (
	"os"
	"testing"
)

// TestMain clears sypl's runtime level overrides for this package.
//
// The Example functions below compare captured stdout. `SYPL_LEVEL` and
// `SYPL_FILTER` raise the effective log level at runtime, so a developer with
// either exported would see the library's own debug output interleaved into the
// example's expected output. Clearing them here keeps the package hermetic
// without weakening a single assertion.
func TestMain(m *testing.M) {
	_ = os.Unsetenv("SYPL_LEVEL")
	_ = os.Unsetenv("SYPL_FILTER")

	os.Exit(m.Run())
}
