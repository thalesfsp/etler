package csvshape

import (
	"encoding"
	"reflect"
	"testing"

	"github.com/gocarina/gocsv"
	"github.com/stretchr/testify/require"
)

type pointerCycle struct {
	Next *pointerCycle
}

type sliceCycle struct {
	Children []sliceCycle
}

type outputFirstLossyExpansion struct {
	Values *[]int `csv:"values" csv[]:"2"`
}

type cycleFirstLossyExpansion struct {
	Values []int `csv:"values" csv[]:"0"`
}

func TestValidateCycleAndOutput_doNotShareCachedVerdicts(t *testing.T) {
	t.Run("output rejection does not leak to cycle validation", func(t *testing.T) {
		require.ErrorContains(t, ValidateOutput(reflect.TypeFor[outputFirstLossyExpansion]()), "unsupported csv[] slice expansion")
		require.NoError(t, ValidateCycle(reflect.TypeFor[outputFirstLossyExpansion]()))
		require.ErrorContains(t, ValidateOutput(reflect.TypeFor[outputFirstLossyExpansion]()), "unsupported csv[] slice expansion")
	})

	t.Run("cycle acceptance does not leak to output validation", func(t *testing.T) {
		require.NoError(t, ValidateCycle(reflect.TypeFor[cycleFirstLossyExpansion]()))
		require.ErrorContains(t, ValidateOutput(reflect.TypeFor[cycleFirstLossyExpansion]()), "positive integer count")
		require.NoError(t, ValidateCycle(reflect.TypeFor[cycleFirstLossyExpansion]()))
	})
}

func TestValidateCycle_rejectsCycles(t *testing.T) {
	require.ErrorContains(t, ValidateCycle(reflect.TypeFor[pointerCycle]()), "invalid cyclic CSV output type csvshape.pointerCycle")
	require.ErrorContains(t, ValidateCycle(reflect.TypeFor[sliceCycle]()), "invalid cyclic CSV output type csvshape.sliceCycle")
}

func TestValidateOutput_handlesExpansionAndTagNameForms(t *testing.T) {
	t.Run("direct positive slice expansion", func(t *testing.T) {
		type row struct {
			Values []int `csv:"values" csv[]:"2"`
		}

		require.NoError(t, ValidateOutput(reflect.TypeFor[row]()))
	})

	t.Run("non-numeric expansion", func(t *testing.T) {
		type row struct {
			Values []int `csv:"values" csv[]:"many"`
		}

		require.ErrorContains(t, ValidateOutput(reflect.TypeFor[row]()), "positive integer count")
	})

	t.Run("tag name has its own cache entry", func(t *testing.T) {
		type row struct {
			Values []int `csv:"values" csv[]:"0" alternate:"values" alternate[]:"2"`
		}

		originalTagName := gocsv.TagName
		defer func() { gocsv.TagName = originalTagName }()

		gocsv.TagName = "csv"
		require.ErrorContains(t, ValidateOutput(reflect.TypeFor[row]()), "positive integer count")

		gocsv.TagName = "alternate"
		require.NoError(t, ValidateOutput(reflect.TypeFor[row]()))
	})
}

type valueCSVMarshaler struct {
	Next *valueCSVMarshaler
}

func (valueCSVMarshaler) MarshalCSV() (string, error) {
	return "", nil
}

type pointerTextMarshaler struct {
	Next *pointerTextMarshaler
}

func (*pointerTextMarshaler) MarshalText() ([]byte, error) {
	return nil, nil
}

type valueCSVUnmarshaller struct {
	Next *valueCSVUnmarshaller
}

func (valueCSVUnmarshaller) UnmarshalCSV(string) error {
	return nil
}

type pointerFieldsUnmarshaller struct {
	Next *pointerFieldsUnmarshaller
}

func (*pointerFieldsUnmarshaller) UnmarshalCSVWithFields(_, _ string) error {
	return nil
}

func TestValidateCycle_treatsGocsvScalarInterfacesAsLeaves(t *testing.T) {
	types := []reflect.Type{
		reflect.TypeFor[valueCSVMarshaler](),
		reflect.TypeFor[pointerTextMarshaler](),
		reflect.TypeFor[valueCSVUnmarshaller](),
		reflect.TypeFor[pointerFieldsUnmarshaller](),
	}

	for _, typ := range types {
		require.NoError(t, ValidateCycle(typ))
	}
}

func TestValidateCycle_handlesNonStructAndSupportedNesting(t *testing.T) {
	type child struct {
		Name string
	}
	type rows []child
	type row struct {
		Child  child
		Rows   rows
		hidden *pointerCycle
	}

	require.NoError(t, ValidateCycle(reflect.TypeFor[int]()))
	require.NoError(t, ValidateCycle(reflect.TypeFor[row]()))
	require.NotNil(t, row{hidden: &pointerCycle{}}.hidden)
}

var (
	_ gocsv.TypeMarshaller             = valueCSVMarshaler{}
	_ encoding.TextMarshaler           = (*pointerTextMarshaler)(nil)
	_ gocsv.TypeUnmarshaller           = valueCSVUnmarshaller{}
	_ gocsv.TypeUnmarshalCSVWithFields = (*pointerFieldsUnmarshaller)(nil)
)
