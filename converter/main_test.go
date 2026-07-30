package converter

import (
	"os"
	"testing"
)

// TestMain clears sypl's runtime level overrides for this package.
func TestMain(m *testing.M) {
	_ = os.Unsetenv("SYPL_LEVEL")
	_ = os.Unsetenv("SYPL_FILTER")

	os.Exit(m.Run())
}
