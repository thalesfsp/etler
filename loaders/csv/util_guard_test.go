package csv

import (
	"errors"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gocarina/gocsv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type GuardCustomInt int

func (value *GuardCustomInt) UnmarshalCSV(string) error {
	*value = 1
	return nil
}

func TestLoad_rejectsInterfaceElement(t *testing.T) {
	var (
		out []any
		err error
	)

	require.NotPanics(t, func() {
		out, err = Load[[]any](strings.NewReader("Value\nx\n"))
	})
	assert.Nil(t, out)
	require.Error(t, err)
	assert.ErrorContains(t, err, "gocsv panic for CSV output type []interface {}")
	assert.ErrorContains(t, err, "runtime error")
}

func TestLoad_rejectsInterfaceField(t *testing.T) {
	type Row struct {
		Value any
	}

	var (
		out []Row
		err error
	)

	require.NotPanics(t, func() {
		out, err = Load[[]Row](strings.NewReader("Value\nx\n"))
	})
	assert.Nil(t, out)
	require.Error(t, err)
	assert.ErrorContains(t, err, "gocsv panic for CSV output type []csv.Row")
	assert.ErrorContains(t, err, "reflect: Elem of invalid type interface {}")
}

func TestLoad_rejectsPointerToInterfaceField(t *testing.T) {
	type Row struct {
		Value *any
	}

	var (
		out []Row
		err error
	)

	require.NotPanics(t, func() {
		out, err = Load[[]Row](strings.NewReader("Value\nx\n"))
	})
	assert.Nil(t, out)
	require.Error(t, err)
	assert.ErrorContains(t, err, "gocsv panic for CSV output type []csv.Row")
	assert.ErrorContains(t, err, "reflect: Elem of invalid type interface {}")
}

func TestLoad_rejectsNestedInterfaceField(t *testing.T) {
	type Nested struct {
		Value any
	}
	type Row struct {
		Nested Nested
	}

	var (
		out []Row
		err error
	)

	require.NotPanics(t, func() {
		out, err = Load[[]Row](strings.NewReader("Nested.Value\nx\n"))
	})
	assert.Nil(t, out)
	require.Error(t, err)
	assert.ErrorContains(t, err, "gocsv panic for CSV output type []csv.Row")
	assert.ErrorContains(t, err, "reflect: Elem of invalid type interface {}")
}

func TestLoad_ignoresInterfaceFields(t *testing.T) {
	type Row struct {
		Name           string
		Ignored        any  `csv:"-"`
		IgnoredPointer *any `csv:"-"`
	}

	out, err := Load[[]Row](strings.NewReader("Name\nalice\n"))
	require.NoError(t, err)
	require.Equal(t, []Row{{Name: "alice"}}, out)
}

func TestLoad_acceptsSupportedOutputShapes(t *testing.T) {
	type Row struct {
		Name string
	}

	t.Run("struct element", func(t *testing.T) {
		out, err := Load[[]Row](strings.NewReader("Name\nalice\n"))
		require.NoError(t, err)
		require.Equal(t, []Row{{Name: "alice"}}, out)
	})

	t.Run("pointer-to-struct element", func(t *testing.T) {
		out, err := Load[[]*Row](strings.NewReader("Name\nalice\n"))
		require.NoError(t, err)
		require.Equal(t, []*Row{{Name: "alice"}}, out)
	})

	t.Run("anonymous struct element", func(t *testing.T) {
		out, err := Load[[]struct{ Name string }](strings.NewReader("Name\nalice\n"))
		require.NoError(t, err)
		require.Equal(t, []struct{ Name string }{{Name: "alice"}}, out)
	})

	t.Run("time.Time field", func(t *testing.T) {
		type TimeRow struct {
			When time.Time
		}

		out, err := Load[[]TimeRow](strings.NewReader("When\n2026-07-30T00:00:00Z\n"))
		require.NoError(t, err)
		require.Equal(t, []TimeRow{{
			When: time.Date(2026, time.July, 30, 0, 0, 0, 0, time.UTC),
		}}, out)
	})
}

func TestLoad_rejectsAnonymousNamedString(t *testing.T) {
	type Name string
	type Row struct {
		Name
	}

	var (
		out []Row
		err error
	)

	require.NotPanics(t, func() {
		out, err = Load[[]Row](strings.NewReader("Name\nalice\n"))
	})
	assert.Nil(t, out)
	assert.EqualError(t, err, "no csv struct tags found")
}

func TestLoad_rejectsAnonymousSlice(t *testing.T) {
	type Items []string
	type row struct {
		Items
	}

	var (
		out []row
		err error
	)

	require.NotPanics(t, func() {
		out, err = Load[[]row](strings.NewReader("Items\nalice\n"))
	})
	assert.Nil(t, out)
	assert.EqualError(t, err, "no csv struct tags found")
}

func TestLoad_rejectsAnonymousMap(t *testing.T) {
	type Values map[string]string
	type row struct {
		Values
	}

	var (
		out []row
		err error
	)

	require.NotPanics(t, func() {
		out, err = Load[[]row](strings.NewReader("Values\nalice\n"))
	})
	assert.Nil(t, out)
	assert.EqualError(t, err, "no csv struct tags found")
}

func TestLoad_rejectsAnonymousInterface(t *testing.T) {
	type Value interface{}
	type row struct {
		Value
	}

	var (
		out []row
		err error
	)

	require.NotPanics(t, func() {
		out, err = Load[[]row](strings.NewReader("Value\nalice\n"))
	})
	assert.Nil(t, out)
	assert.EqualError(t, err, "no csv struct tags found")
}

func TestLoad_acceptsAnonymousStruct(t *testing.T) {
	type Embedded struct {
		Name string
	}
	type row struct {
		Embedded
	}

	out, err := Load[[]row](strings.NewReader("Name\nalice\n"))
	require.NoError(t, err)
	require.Equal(t, []row{{
		Embedded: Embedded{Name: "alice"},
	}}, out)
}

func TestValidateCSVOutputType_handlesSupportedFieldShapes(t *testing.T) {
	type Embedded struct {
		Value int
	}
	type row struct {
		Embedded
		Custom  GuardCustomInt
		Ignored int `csv:"-"`
		hidden  int
		Parent  struct {
			Count uint8
		}
		When time.Time
	}

	err := validateCSVOutputType(reflect.TypeFor[row]())
	require.NoError(t, err)

	// An unexported field must never be discovered: reflection cannot set it,
	// so assigning to one would panic. The literal below also proves the field
	// is real rather than a name the compiler dropped.
	require.Equal(t, 42, row{hidden: 42}.hidden)
}

func TestLoad_acceptsCSVSliceExpansion(t *testing.T) {
	type item struct {
		ID int
	}
	type row struct {
		Items []item `csv:"items" csv[]:"2"`
	}

	out, err := Load[[]row](strings.NewReader("items[0].ID,items[1].ID\n7,8\n"))
	require.NoError(t, err)
	require.Equal(t, []row{{Items: []item{{ID: 7}, {ID: 8}}}}, out)
}

func TestLoad_acceptsDirectCSVSliceExpansionForms(t *testing.T) {
	t.Run("slice of primitives", func(t *testing.T) {
		type row struct {
			Items []int `csv:"items" csv[]:"2"`
		}

		out, err := Load[[]row](strings.NewReader("items[0],items[1]\n7,8\n"))

		require.NoError(t, err)
		require.Equal(t, []row{{Items: []int{7, 8}}}, out)
	})

	t.Run("slice count larger than columns", func(t *testing.T) {
		type row struct {
			Items []int `csv:"items" csv[]:"5"`
		}

		out, err := Load[[]row](strings.NewReader("items[0],items[1]\n7,8\n"))

		require.NoError(t, err)
		require.Equal(t, []row{{Items: []int{7, 8}}}, out)
	})

	t.Run("array of primitives", func(t *testing.T) {
		type row struct {
			Items [2]int `csv:"items" csv[]:"2"`
		}

		out, err := Load[[]row](strings.NewReader("items[0],items[1]\n7,8\n"))

		require.NoError(t, err)
		require.Equal(t, []row{{Items: [2]int{7, 8}}}, out)
	})

	t.Run("array count smaller than length", func(t *testing.T) {
		type row struct {
			Items [3]int `csv:"items" csv[]:"2"`
		}

		out, err := Load[[]row](strings.NewReader("items[0],items[1]\n7,8\n"))

		require.NoError(t, err)
		require.Equal(t, []row{{Items: [3]int{7, 8}}}, out)
	})

	t.Run("array of structs", func(t *testing.T) {
		type item struct {
			ID int
		}
		type row struct {
			Items [2]item `csv:"items" csv[]:"2"`
		}

		out, err := Load[[]row](strings.NewReader("items[0].ID,items[1].ID\n7,8\n"))

		require.NoError(t, err)
		require.Equal(t, []row{{Items: [2]item{{ID: 7}, {ID: 8}}}}, out)
	})
}

func TestLoad_rejectsCSVSliceExpansionThroughPointer(t *testing.T) {
	type row struct {
		Items *[]int `csv:"Items" csv[]:"1"`
	}

	out, err := Load[[]row](strings.NewReader("Items[0]\n7\n"))
	assert.Nil(t, out)
	require.Error(t, err)
	assert.ErrorContains(t, err, "unsupported csv[] slice expansion")
	assert.ErrorContains(t, err, "Items")
}

func TestLoad_rejectsCSVSliceExpansionWithInvalidCount(t *testing.T) {
	t.Run("pointer-wrapped", func(t *testing.T) {
		type row struct {
			Items *[]int `csv:"items" csv[]:"2"`
		}

		out, err := Load[[]row](strings.NewReader("items[0],items[1]\n7,8\n"))

		require.Nil(t, out)
		require.ErrorContains(t, err, "unsupported csv[] slice expansion")
		require.ErrorContains(t, err, "pointer-wrapped")
		require.NotContains(t, err.Error(), "positive integer count")
	})

	tests := []struct {
		name  string
		count string
	}{
		{name: "non-numeric", count: "many"},
		{name: "zero", count: "0"},
		{name: "negative", count: "-1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field := reflect.StructOf([]reflect.StructField{{
				Name: "Items",
				Type: reflect.TypeFor[[]int](),
				Tag:  reflect.StructTag(`csv:"items" csv[]:"` + tt.count + `"`),
			}})

			err := validateCSVOutputType(field)

			require.Error(t, err)
			assert.ErrorContains(t, err, "unsupported csv[] slice expansion")
			assert.ErrorContains(t, err, "positive integer count")
			assert.NotContains(t, err.Error(), "pointer-wrapped")
		})
	}
}

func TestLoad_rejectsCSVSliceExpansionThroughOtherWrappers(t *testing.T) {
	t.Run("pointer to array", func(t *testing.T) {
		type row struct {
			Items *[1]int `csv:"Items" csv[]:"1"`
		}

		out, err := Load[[]row](strings.NewReader("Items[0]\n7\n"))
		assert.Nil(t, out)
		require.Error(t, err)
		assert.ErrorContains(t, err, "unsupported csv[] slice expansion")
		assert.ErrorContains(t, err, "Items")
	})

	t.Run("multiple pointers to slice", func(t *testing.T) {
		type row struct {
			Items **[]int `csv:"Items" csv[]:"1"`
		}

		out, err := Load[[]row](strings.NewReader("Items[0]\n7\n"))
		assert.Nil(t, out)
		require.Error(t, err)
		assert.ErrorContains(t, err, "unsupported csv[] slice expansion")
		assert.ErrorContains(t, err, "Items")
	})
}

func TestValidateCSVOutputType_rejectsCycles(t *testing.T) {
	err := validateCSVOutputType(reflect.TypeFor[PointerCycle]())
	require.Error(t, err)
	assert.ErrorContains(t, err, "cyclic CSV output type")

	err = validateCSVOutputType(reflect.TypeFor[SliceCycle]())
	require.Error(t, err)
	assert.ErrorContains(t, err, "cyclic CSV output type")
}

func TestLoad_memoizesCSVOutputTypeValidationByTypeAndTagName(t *testing.T) {
	t.Run("repeated load uses cached verdict", func(t *testing.T) {
		type row struct {
			Name string
		}

		key := csvOutputTypeValidationCacheKey{
			rowType: reflect.TypeFor[row](),
			tagName: gocsv.TagName,
		}
		csvOutputTypeValidationCache.Delete(key)
		defer csvOutputTypeValidationCache.Delete(key)

		out, err := Load[[]row](strings.NewReader("Name\nbob\n"))
		require.NoError(t, err)
		require.Equal(t, []row{{Name: "bob"}}, out)

		cached, ok := csvOutputTypeValidationCache.Load(key)
		require.True(t, ok)
		require.NoError(t, cached.(csvOutputTypeValidationCacheValue).err)

		cachedErr := errors.New("cached CSV output type verdict")
		csvOutputTypeValidationCache.Store(key, csvOutputTypeValidationCacheValue{err: cachedErr})

		out, err = Load[[]row](strings.NewReader("Name\nbob\n"))
		require.Nil(t, out)
		require.ErrorIs(t, err, cachedErr)
	})

	t.Run("tag name change re-evaluates verdict", func(t *testing.T) {
		type row struct {
			Items []int `csv:"items" csv[]:"0" alternate:"items" alternate[]:"2"`
		}

		originalTagName := gocsv.TagName
		defer func() { gocsv.TagName = originalTagName }()

		rowType := reflect.TypeFor[row]()
		csvKey := csvOutputTypeValidationCacheKey{rowType: rowType, tagName: "csv"}
		alternateKey := csvOutputTypeValidationCacheKey{rowType: rowType, tagName: "alternate"}
		csvOutputTypeValidationCache.Delete(csvKey)
		csvOutputTypeValidationCache.Delete(alternateKey)
		defer csvOutputTypeValidationCache.Delete(csvKey)
		defer csvOutputTypeValidationCache.Delete(alternateKey)

		gocsv.TagName = "csv"
		err := validateCSVOutputType(rowType)
		require.ErrorContains(t, err, "positive integer count")

		gocsv.TagName = "alternate"
		err = validateCSVOutputType(rowType)
		require.NoError(t, err)

		_, csvCached := csvOutputTypeValidationCache.Load(csvKey)
		_, alternateCached := csvOutputTypeValidationCache.Load(alternateKey)
		require.True(t, csvCached)
		require.True(t, alternateCached)
	})
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
