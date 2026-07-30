package csv

import (
	"reflect"
	"strconv"
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

	// An unexported field must never be discovered: reflection cannot set it,
	// so assigning to one would panic. The literal below also proves the field
	// is real rather than a name the compiler dropped.
	require.Equal(t, 42, row{hidden: 42}.hidden)
	assert.NotContains(t, keys, "hidden")
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

func TestNumericValueError_validatesSyntaxAndRange(t *testing.T) {
	assert.NoError(t, numericValueError("", reflect.TypeFor[*int]()))
	assert.NoError(t, numericValueError("255", reflect.TypeFor[uint8]()))
	assert.NoError(t, numericValueError("-1.5e-2", reflect.TypeFor[float32]()))
	assert.NoError(t, numericValueError("not numeric", reflect.TypeFor[string]()))
	assert.ErrorIs(t, numericValueError("255.9", reflect.TypeFor[uint8]()), strconv.ErrSyntax)
	assert.ErrorIs(t, numericValueError("256", reflect.TypeFor[uint8]()), strconv.ErrRange)
	assert.ErrorIs(t, numericValueError("-1", reflect.TypeFor[uint8]()), strconv.ErrSyntax)
	assert.ErrorIs(t, numericValueError("1_0", reflect.TypeFor[int]()), strconv.ErrSyntax)
}
