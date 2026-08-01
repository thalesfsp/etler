package shared

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
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

			childArgs := []string{"-test.run=^TestExtractID_interfaceFieldCyclesReturnEmpty$"}
			for _, arg := range os.Args {
				if strings.HasPrefix(arg, "-test.gocoverdir=") {
					childArgs = append(childArgs, arg)

					break
				}
			}

			cmd := exec.CommandContext(ctx, os.Args[0], childArgs...)
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

//////
// Pointer aliases across ExtractID sibling branches.
//////

func TestExtractID_pointerAliasResolvesRegardlessOfSiblingOrder(t *testing.T) {
	type Account struct {
		ID string
	}

	type Owner struct {
		ID *Account
	}

	type Ref struct {
		*Account
	}

	type Record struct {
		Owner
		Ref
	}

	type RecordReversed struct {
		Ref
		Owner
	}

	account := &Account{ID: "acct-42"}
	recordID := ExtractID(Record{Owner: Owner{ID: account}, Ref: Ref{Account: account}}, "")
	reversedID := ExtractID(RecordReversed{Ref: Ref{Account: account}, Owner: Owner{ID: account}}, "")

	assert.Equal(t, "acct-42", recordID)
	assert.Equal(t, recordID, reversedID)
}

func TestExtractID_distinctPointersKeepSiblingPrecedence(t *testing.T) {
	type Account struct {
		ID string
	}

	type Owner struct {
		ID *Account
	}

	type Ref struct {
		*Account
	}

	type Record struct {
		Owner
		Ref
	}

	ownerAccount := &Account{ID: "acct-A"}
	refAccount := &Account{ID: "acct-B"}
	record := Record{Owner: Owner{ID: ownerAccount}, Ref: Ref{Account: refAccount}}

	assert.Equal(t, "acct-B", ExtractID(record, ""))
}

func TestExtractID_unsupportedBranchDoesNotBlockLaterSibling(t *testing.T) {
	type Key struct {
		ExternalID string
	}

	type Unsupported struct {
		ExternalID *Key
	}

	type Reachable struct {
		*Key
	}

	type Record struct {
		Unsupported
		Reachable
	}

	key := &Key{ExternalID: "reachable"}
	record := Record{
		Unsupported: Unsupported{ExternalID: key},
		Reachable:   Reachable{Key: key},
	}

	assert.Equal(t, "reachable", ExtractID(record, "ExternalID"))
}
