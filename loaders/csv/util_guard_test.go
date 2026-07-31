package csv

import (
	"reflect"
	"strconv"
	"strings"
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
		When time.Time
	}

	fields, err := inspectCSVFields(reflect.TypeFor[row](), "", true, nil)
	require.NoError(t, err)

	keys := make([]string, 0, len(fields))
	for _, field := range fields {
		if field.key != "" {
			keys = append(keys, field.key)
		}
	}
	assert.ElementsMatch(t, []string{"Value", "Parent.Count", "When"}, keys)

	// An unexported field must never be discovered: reflection cannot set it,
	// so assigning to one would panic. The literal below also proves the field
	// is real rather than a name the compiler dropped.
	require.Equal(t, 42, row{hidden: 42}.hidden)
	assert.NotContains(t, keys, "hidden")
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

func TestInspectCSVFields_rejectsCycles(t *testing.T) {
	fields, err := inspectCSVFields(reflect.TypeFor[PointerCycle](), "", true, nil)
	assert.Nil(t, fields)
	require.Error(t, err)
	assert.ErrorContains(t, err, "cyclic CSV output type")

	fields, err = inspectCSVFields(reflect.TypeFor[SliceCycle](), "", true, nil)
	assert.Nil(t, fields)
	require.Error(t, err)
	assert.ErrorContains(t, err, "cyclic CSV output type")
}

func TestNumericValueError_validatesSyntaxAndRange(t *testing.T) {
	assert.NoError(t, numericValueError("", reflect.TypeFor[*int]()))
	assert.NoError(t, numericValueError(" +42 ", reflect.TypeFor[int]()))
	assert.NoError(t, numericValueError("255", reflect.TypeFor[uint8]()))
	assert.NoError(t, numericValueError(" -.5e+2 ", reflect.TypeFor[float32]()))
	assert.NoError(t, numericValueError("not numeric", reflect.TypeFor[string]()))
	assert.ErrorIs(t, numericValueError("255.9", reflect.TypeFor[uint8]()), strconv.ErrSyntax)
	assert.ErrorIs(t, numericValueError("256", reflect.TypeFor[uint8]()), strconv.ErrRange)
	assert.ErrorIs(t, numericValueError("-1", reflect.TypeFor[uint8]()), strconv.ErrSyntax)
	assert.ErrorIs(t, numericValueError("1_0", reflect.TypeFor[int]()), strconv.ErrSyntax)
}
