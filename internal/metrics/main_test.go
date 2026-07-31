package metrics

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	_ = os.Unsetenv("SYPL_LEVEL")
	_ = os.Unsetenv("SYPL_FILTER")

	// Publishing makes lifecycle metrics global, so entities sharing a type and
	// name share counters. Tests that need it on set it themselves with
	// `t.Setenv`, which also restores it afterwards.
	_ = os.Unsetenv(PublishEnvVar)

	os.Exit(m.Run())
}
