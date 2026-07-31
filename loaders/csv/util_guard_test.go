package csv

import (
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type GuardCustomInt int

func (value *GuardCustomInt) UnmarshalCSV(string) error {
	*value = 1
	return nil
}

func TestValidateCSVOutputType_handlesSupportedFieldShapes(t *testing.T) {
	type Embedded struct {
		Value int
	}
	type row struct {
		Embedded
		GuardCustomInt
		Ignored int `csv:"-"`
		hidden  int
		Parent  struct {
			Count uint8
		}
		When time.Time
	}

	err := validateCSVOutputType(reflect.TypeFor[row](), nil)
	require.NoError(t, err)

	// An unexported field must never be discovered: reflection cannot set it,
	// so assigning to one would panic. The literal below also proves the field
	// is real rather than a name the compiler dropped.
	require.Equal(t, 42, row{hidden: 42}.hidden)
}

func TestLoad_rejectsCSVSliceExpansion(t *testing.T) {
	type row struct {
		Items []struct {
			ID uint8
		} `csv:"Items" csv[]:"2"`
	}

	out, err := Load[[]row](strings.NewReader("Items[0].ID\n256\n"))
	assert.Nil(t, out)
	require.Error(t, err)
	assert.ErrorContains(t, err, "unsupported csv[] slice expansion")
	assert.ErrorContains(t, err, "Items")
}

func TestValidateCSVOutputType_rejectsCycles(t *testing.T) {
	err := validateCSVOutputType(reflect.TypeFor[PointerCycle](), nil)
	require.Error(t, err)
	assert.ErrorContains(t, err, "cyclic CSV output type")

	err = validateCSVOutputType(reflect.TypeFor[SliceCycle](), nil)
	require.Error(t, err)
	assert.ErrorContains(t, err, "cyclic CSV output type")
}

func TestLoad_round6TimeFieldDoesNotHideGocsvUint8Truncation(t *testing.T) {
	type row struct {
		When time.Time
		N    uint8
	}

	out, err := Load[[]row](strings.NewReader(
		"When,N\n2026-07-30T00:00:00Z,256\n",
	))
	require.NoError(t, err)
	require.Equal(t, []row{{
		When: time.Date(2026, time.July, 30, 0, 0, 0, 0, time.UTC),
		N:    0,
	}}, out)
}

var unmarshalCSVCallCount atomic.Int32

type countedCSVValue string

func (*countedCSVValue) UnmarshalCSV(string) error {
	unmarshalCSVCallCount.Add(1)

	return nil
}

func TestLoad_invokesCustomUnmarshallerOncePerValue(t *testing.T) {
	type row struct {
		Value countedCSVValue
		N     int
	}

	unmarshalCSVCallCount.Store(0)
	out, err := Load[[]row](strings.NewReader("Value,N\ntext,1\n"))
	require.NoError(t, err)
	require.Len(t, out, 1)
	assert.Equal(t, int32(1), unmarshalCSVCallCount.Load())
}
