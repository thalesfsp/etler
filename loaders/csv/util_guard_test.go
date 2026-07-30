package csv

import (
	"reflect"
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

func TestInspectCSVFields_handlesSupportedFieldShapes(t *testing.T) {
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
		Items []struct {
			ID int
		}
		When time.Time
	}

	fields, err := inspectCSVFields(reflect.TypeFor[row](), nil, false, nil)
	require.NoError(t, err)

	keys := make([]string, 0, len(fields))
	for _, field := range fields {
		keys = append(keys, field.keys...)
	}
	assert.ElementsMatch(t, []string{"Value", "Parent.Count", "Items", "When"}, keys)
}

func TestInspectCSVFields_rejectsCycles(t *testing.T) {
	fields, err := inspectCSVFields(reflect.TypeFor[PointerCycle](), nil, false, nil)
	assert.Nil(t, fields)
	require.Error(t, err)
	assert.ErrorContains(t, err, "cyclic CSV output type")

	fields, err = inspectCSVFields(reflect.TypeFor[SliceCycle](), nil, false, nil)
	assert.Nil(t, fields)
	require.Error(t, err)
	assert.ErrorContains(t, err, "cyclic CSV output type")
}

func TestNumericRangeError_unsignedFractions(t *testing.T) {
	assert.False(t, numericRangeError("255.9", reflect.TypeFor[uint8]()))
	assert.True(t, numericRangeError("256.0", reflect.TypeFor[uint8]()))
	assert.True(t, numericRangeError("-1.0", reflect.TypeFor[uint8]()))
	assert.False(t, numericRangeError("bad.0", reflect.TypeFor[uint8]()))
}
