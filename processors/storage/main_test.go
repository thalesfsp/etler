package storage

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	_ = os.Unsetenv("SYPL_LEVEL")
	_ = os.Unsetenv("SYPL_FILTER")

	os.Exit(m.Run())
}
