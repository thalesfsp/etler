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
