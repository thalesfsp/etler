package shared

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const extractIDInterfaceCycleChildCase = "ETLER_EXTRACT_ID_INTERFACE_CYCLE_CHILD_CASE"

//////
// Interface-mediated ExtractID cycles.
//////

func TestExtractID_interfaceFieldCyclesReturnEmpty(t *testing.T) {
	if childCase := os.Getenv(extractIDInterfaceCycleChildCase); childCase != "" {
		fmt.Printf("id=%q\n", extractIDInterfaceCycleResult(t, childCase))

		return
	}

	for _, childCase := range []string{"self-reference", "nested-reference"} {
		t.Run(childCase, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestExtractID_interfaceFieldCyclesReturnEmpty$")
			cmd.Env = append(
				os.Environ(),
				extractIDInterfaceCycleChildCase+"="+childCase,
				"GORACE=atexit_sleep_ms=0",
			)

			output, err := cmd.CombinedOutput()
			require.NotErrorIs(t, ctx.Err(), context.DeadlineExceeded, "ExtractID subprocess did not return")
			require.NoError(t, err, "ExtractID subprocess failed: %s", output)
			assert.Contains(t, string(output), "id=\"\"\n")
		})
	}
}

func extractIDInterfaceCycleResult(t *testing.T, childCase string) string {
	t.Helper()

	switch childCase {
	case "self-reference":
		type recID struct {
			ID any
		}

		record := recID{}
		record.ID = &record.ID

		return ExtractID(record, "")
	case "nested-reference":
		type RecID struct {
			ID any
		}

		type Envelope struct {
			RecID
			Back any
		}

		record := Envelope{}
		nested := &record.RecID
		nested.ID = &record.Back
		record.Back = &nested.ID

		return ExtractID(record, "")
	default:
		t.Fatalf("unknown interface cycle case %q", childCase)

		return ""
	}
}

func TestExtractID_ordinaryInterfaceValuesStillResolve(t *testing.T) {
	t.Run("string ID", func(t *testing.T) {
		type recID struct {
			ID any
		}

		assert.Equal(t, "usable", ExtractID(recID{ID: "usable"}, ""))
	})

	t.Run("pointer to struct with ID", func(t *testing.T) {
		value := &idCarrier{ID: "pointer-struct"}

		assert.Equal(t, "pointer-struct", ExtractID(embedsInterface{Stringer: value}, ""))
	})
}

func TestExtractID_pointerCycleBehaviorIsUnchanged(t *testing.T) {
	type pointerCycle struct {
		ID *pointerCycle
	}

	value := &pointerCycle{}
	value.ID = value

	assert.Empty(t, ExtractID(value, ""))
}
