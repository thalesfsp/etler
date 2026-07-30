package csv

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetFieldInfo_reportsNestedCycles(t *testing.T) {
	t.Run("pointer field", func(t *testing.T) {
		type pointerCycle struct {
			Next *pointerCycle
		}

		fields, err := getFieldInfo(reflect.TypeFor[pointerCycle](), nil, nil)

		assert.Nil(t, fields)
		require.Error(t, err)
		assert.ErrorContains(t, err, "cyclic CSV output type")
	})

	t.Run("slice field", func(t *testing.T) {
		type sliceCycle struct {
			Children []sliceCycle
		}

		fields, err := getFieldInfo(reflect.TypeFor[sliceCycle](), nil, nil)

		assert.Nil(t, fields)
		require.Error(t, err)
		assert.ErrorContains(t, err, "cyclic CSV output type")
	})
}

func TestDecoderHeaderAssignments_escapesCollidingUnmatchedHeader(t *testing.T) {
	fields := []fieldInfo{
		{keys: []string{"expected"}},
		{keys: []string{"\x00etler-unmatched-0"}},
	}

	headers, manualFields := decoderHeaderAssignments(
		[]string{"no matching field"},
		[]int{0},
		fields,
	)

	assert.Equal(t, []string{"\x00etler-unmatched-0\x00"}, headers)
	assert.Equal(t, map[int]struct{}{0: {}}, manualFields)
}

func TestDecodeFieldValue_rejectsInvalidDestinationValue(t *testing.T) {
	decoded, err := decodeFieldValue("not-a-bool", reflect.TypeFor[bool]())

	assert.False(t, decoded.IsValid())
	require.Error(t, err)
	assert.ErrorContains(t, err, `strconv.ParseBool: parsing "not-a-bool": invalid syntax`)
}

func TestAssignManualFields_rejectsValueTypeMismatch(t *testing.T) {
	type row struct {
		Count int
	}

	fields, err := getFieldInfo(reflect.TypeFor[row](), nil, nil)
	require.NoError(t, err)
	require.Len(t, fields, 1)

	assigned, err := assignManualFields(
		row{},
		[]string{"not-an-int"},
		[]int{0},
		map[int]struct{}{0: {}},
		fields,
	)

	assert.Nil(t, assigned)
	require.Error(t, err)
	assert.ErrorContains(t, err, `invalid syntax`)
}
