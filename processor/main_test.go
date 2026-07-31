package processor

import (
	"os"
	"testing"
)

// TestMain clears sypl's runtime level overrides for this package.
func TestMain(m *testing.M) {
	_ = os.Unsetenv("SYPL_LEVEL")
	_ = os.Unsetenv("SYPL_FILTER")

	// Publishing makes lifecycle metrics global, so entities sharing a type and
	// name share counters. Per-instance assertions need it off.
	_ = os.Unsetenv("ETLER_METRICS_PUBLISH")

	os.Exit(m.Run())
}
