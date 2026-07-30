package csv

import (
	"errors"
	"io"
	"math"
	"os"
	"os/exec"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gocarina/gocsv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_rejectsNumericOverflow(t *testing.T) {
	type testCase struct {
		name         string
		inside       string
		outside      string
		lowerInside  string
		lowerOutside string
		load         func(string) (any, error)
		insideWant   any
		lowerWant    any
	}

	testCases := []testCase{
		{
			name:       "int",
			inside:     strconv.FormatInt(int64(maxInt()), 10),
			outside:    strconv.FormatUint(uint64(maxInt())+1, 10),
			load:       func(in string) (any, error) { return Load[[]struct{ N int }](strings.NewReader(in)) },
			insideWant: maxInt(),
		},
		{
			name:         "int8",
			inside:       "127",
			outside:      "128",
			lowerInside:  "-128",
			lowerOutside: "-129",
			load:         func(in string) (any, error) { return Load[[]struct{ N int8 }](strings.NewReader(in)) },
			insideWant:   int8(127),
			lowerWant:    int8(-128),
		},
		{
			name:       "int16",
			inside:     "32767",
			outside:    "32768",
			load:       func(in string) (any, error) { return Load[[]struct{ N int16 }](strings.NewReader(in)) },
			insideWant: int16(32767),
		},
		{
			name:       "int32",
			inside:     "2147483647",
			outside:    "2147483648",
			load:       func(in string) (any, error) { return Load[[]struct{ N int32 }](strings.NewReader(in)) },
			insideWant: int32(2147483647),
		},
		{
			name:         "int64",
			inside:       "9223372036854775807",
			outside:      "9223372036854775808",
			lowerInside:  "-9223372036854775808",
			lowerOutside: "-9223372036854775809",
			load:         func(in string) (any, error) { return Load[[]struct{ N int64 }](strings.NewReader(in)) },
			insideWant:   int64(9223372036854775807),
			lowerWant:    int64(-9223372036854775808),
		},
		{
			name:       "uint",
			inside:     strconv.FormatUint(uint64(maxUint()), 10),
			outside:    uintOverflowValue(),
			load:       func(in string) (any, error) { return Load[[]struct{ N uint }](strings.NewReader(in)) },
			insideWant: maxUint(),
		},
		{
			name:         "uint8",
			inside:       "255",
			outside:      "256",
			lowerInside:  "0",
			lowerOutside: "-1",
			load:         func(in string) (any, error) { return Load[[]struct{ N uint8 }](strings.NewReader(in)) },
			insideWant:   uint8(255),
			lowerWant:    uint8(0),
		},
		{
			name:       "uint16",
			inside:     "65535",
			outside:    "65536",
			load:       func(in string) (any, error) { return Load[[]struct{ N uint16 }](strings.NewReader(in)) },
			insideWant: uint16(65535),
		},
		{
			name:       "uint32",
			inside:     "4294967295",
			outside:    "4294967296",
			load:       func(in string) (any, error) { return Load[[]struct{ N uint32 }](strings.NewReader(in)) },
			insideWant: uint32(4294967295),
		},
		{
			name:       "uint64",
			inside:     "18446744073709551615",
			outside:    "18446744073709551616",
			load:       func(in string) (any, error) { return Load[[]struct{ N uint64 }](strings.NewReader(in)) },
			insideWant: uint64(18446744073709551615),
		},
		{
			name:       "float32",
			inside:     "3.4028235e38",
			outside:    "3.4028236e38",
			load:       func(in string) (any, error) { return Load[[]struct{ N float32 }](strings.NewReader(in)) },
			insideWant: float32(math.MaxFloat32),
		},
		{
			name:       "float64",
			inside:     "1.7976931348623157e308",
			outside:    "1.7976931348623159e308",
			load:       func(in string) (any, error) { return Load[[]struct{ N float64 }](strings.NewReader(in)) },
			insideWant: float64(math.MaxFloat64),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			out, err := testCase.load("N\n" + testCase.inside + "\n")
			require.NoError(t, err)
			require.Len(t, out, 1)
			assert.Equal(t, testCase.insideWant, firstField(out))

			out, err = testCase.load("N\n" + testCase.outside + "\n")
			assert.Nil(t, out)
			require.Error(t, err)
			assert.ErrorContains(t, err, "field N")
			assert.ErrorContains(t, err, testCase.outside)

			if testCase.lowerInside != "" {
				out, err = testCase.load("N\n" + testCase.lowerInside + "\n")
				require.NoError(t, err)
				require.Len(t, out, 1)
				assert.Equal(t, testCase.lowerWant, firstField(out))

				out, err = testCase.load("N\n" + testCase.lowerOutside + "\n")
				assert.Nil(t, out)
				require.Error(t, err)
				assert.ErrorContains(t, err, "field N")
				assert.ErrorContains(t, err, testCase.lowerOutside)
			}
		})
	}
}

func TestLoad_matchesUntaggedFieldsCaseInsensitively(t *testing.T) {
	t.Run("lowercase header", func(t *testing.T) {
		type row struct {
			Name string
		}

		out, err := Load[[]row](strings.NewReader("name\nalice\n"))
		require.NoError(t, err)
		require.Equal(t, []row{{Name: "alice"}}, out)
	})

	t.Run("explicit tag takes precedence", func(t *testing.T) {
		type row struct {
			Name string `csv:"display_name"`
		}

		out, err := Load[[]row](strings.NewReader("name,display_name\nwrong,alice\n"))
		require.NoError(t, err)
		require.Equal(t, []row{{Name: "alice"}}, out)
	})
}

func TestLoad_nilReaderReturnsRequiredArgumentError(t *testing.T) {
	assert.NotPanics(t, func() {
		out, err := Load[[]struct{ Name string }](nil)
		assert.Nil(t, out)
		assert.EqualError(t, err, "CSV reader required")
	})
}

func TestLoad_isolatesConfigurableGocsvGlobals(t *testing.T) {
	t.Run("reader factory", func(t *testing.T) {
		injectedErr := errors.New("global gocsv reader factory used")
		gocsv.SetCSVReader(func(io.Reader) gocsv.CSVReader {
			return errorCSVReader{err: injectedErr}
		})
		defer gocsv.SetCSVReader(gocsv.DefaultCSVReader)

		out, err := Load[[]struct{ Name string }](strings.NewReader("Name\nalice\n"))
		require.NoError(t, err)
		require.Equal(t, []struct{ Name string }{{Name: "alice"}}, out)
	})

	t.Run("unmatched struct tags", func(t *testing.T) {
		gocsv.FailIfUnmatchedStructTags = true
		defer func() {
			gocsv.FailIfUnmatchedStructTags = false
		}()

		type row struct {
			Name string
			Note string
		}

		out, err := Load[[]row](strings.NewReader("Name\nalice\n"))
		require.NoError(t, err)
		require.Equal(t, []row{{Name: "alice"}}, out)
	})
}

func TestLoad_rejectsSelfReferentialOutputType(t *testing.T) {
	const helperEnvironment = "ETLER_CSV_SELF_REFERENCE_HELPER"
	if os.Getenv(helperEnvironment) == "1" {
		out, err := Load[[]RecursiveRow](strings.NewReader("Name\nalice\n"))
		assert.Nil(t, out)
		require.Error(t, err)
		require.ErrorContains(t, err, "cyclic CSV output type")

		return
	}

	command := exec.Command(os.Args[0], "-test.run=^TestLoad_rejectsSelfReferentialOutputType$")
	command.Env = append(os.Environ(), helperEnvironment+"=1")
	require.NoError(t, command.Run(), "self-referential output crashed subprocess")
}

type RecursiveRow struct {
	*RecursiveRow
}

func TestCSVHelpers(t *testing.T) {
	t.Run("nil-like values", func(t *testing.T) {
		var pointer *strings.Reader
		var channel chan int
		var function func()
		var interfaceValue any
		var mapValue map[string]string
		var slice []string

		assert.True(t, isNil(pointer))
		assert.True(t, isNil(channel))
		assert.True(t, isNil(function))
		assert.True(t, isNil(interfaceValue))
		assert.True(t, isNil(mapValue))
		assert.True(t, isNil(slice))
		assert.False(t, isNil(1))
	})

	t.Run("field metadata", func(t *testing.T) {
		type embedded struct {
			Value int
		}
		type row struct {
			embedded
			Visible int `csv:"number,partial,default=7,omitempty"`
			Ignored int `csv:"-"`
			hidden  int // unexported: must be skipped by field discovery
			Nested  struct {
				Count int
			}
			NestedPointer *struct {
				Total uint8
			}
			When  time.Time
			Items []struct {
				ID int
			}
		}

		fields, err := getFieldInfo(reflect.TypeFor[row](), nil, nil)
		require.NoError(t, err)
		require.NotEmpty(t, fields)

		tagged, ignored := newFieldInfo(reflect.TypeFor[row]().Field(1), []string{"parent"})
		require.False(t, ignored)
		require.Equal(t, []string{"parent.number"}, tagged.keys)
		require.Equal(t, "7", tagged.defaultValue)
		assert.True(t, tagged.tagged)
		assert.True(t, tagged.partial)

		anonymous, ignored := newFieldInfo(reflect.TypeFor[row]().Field(0), nil)
		assert.Nil(t, anonymous)
		assert.False(t, ignored)

		ignoredField, ignored := newFieldInfo(reflect.TypeFor[row]().Field(2), nil)
		assert.Nil(t, ignoredField)
		assert.True(t, ignored)

		// An unexported field must never be discovered: it cannot be set, so
		// including it would panic at assignment time.
		hiddenField := reflect.TypeFor[row]().Field(3)
		require.Equal(t, "hidden", hiddenField.Name)
		require.False(t, hiddenField.IsExported())
		require.Equal(t, 42, row{hidden: 42}.hidden, "the field is real and settable in-package")

		// The exported check lives in getFieldInfo, not newFieldInfo, so it is
		// the discovered set that must exclude it. An unexported field cannot
		// be set through reflection, so discovering one would panic on assign.
		for _, f := range fields {
			assert.NotContains(t, f.keys, "hidden")
			assert.NotEqual(t, "hidden", f.name)
		}

		assert.True(t, isCSVScalar(reflect.TypeFor[time.Time]()))
		assert.False(t, isCSVScalar(reflect.TypeFor[struct{ Value int }]()))
	})

	t.Run("number syntax", func(t *testing.T) {
		require.NoError(t, validateNumber("", reflect.TypeFor[int8]()))
		require.NoError(t, validateNumber("1.9", reflect.TypeFor[int8]()))
		require.NoError(t, validateNumber("255.0", reflect.TypeFor[uint8]()))
		require.ErrorIs(t, validateNumber("-1.0", reflect.TypeFor[uint8]()), strconv.ErrRange)
		require.Error(t, validateNumber("bad.0", reflect.TypeFor[uint8]()))
		require.Error(t, validateNumber("bad", reflect.TypeFor[float32]()))
		require.NoError(t, validateNumber("anything", reflect.TypeFor[string]()))
		require.Equal(
			t,
			"number",
			removeZeroWidthCharacters("\u200Bnum\u200Dber"),
		)
	})
}

func TestLoad_pointerRowsAndConversionErrors(t *testing.T) {
	type row struct {
		Active bool
	}

	out, err := Load[[]*row](strings.NewReader("Active\ntrue\n"))
	require.NoError(t, err)
	require.Equal(t, []*row{{Active: true}}, out)

	out, err = Load[[]*row](strings.NewReader("Active\nnot-a-bool\n"))
	assert.Nil(t, out)
	require.Error(t, err)
	assert.ErrorContains(t, err, `strconv.ParseBool: parsing "not-a-bool": invalid syntax`)

	invalidOut, err := Load[[]string](strings.NewReader("Value\nx\n"))
	assert.Nil(t, invalidOut)
	require.Error(t, err)

	customOut, err := Load[[]struct{ N customInt }](strings.NewReader("N\nword\n"))
	require.NoError(t, err)
	require.Equal(t, []struct{ N customInt }{{N: 42}}, customOut)
}

type errorCSVReader struct {
	err error
}

func (r errorCSVReader) Read() ([]string, error) {
	return nil, r.err
}

func (r errorCSVReader) ReadAll() ([][]string, error) {
	return nil, r.err
}

func firstField(value any) any {
	slice := reflect.ValueOf(value)

	return slice.Index(0).Field(0).Interface()
}

func maxInt() int {
	return int(^uint(0) >> 1)
}

func maxUint() uint {
	return ^uint(0)
}

func uintOverflowValue() string {
	if strconv.IntSize == 64 {
		return "18446744073709551616"
	}

	return "4294967296"
}

type customInt int

func (value *customInt) UnmarshalCSV(string) error {
	*value = 42

	return nil
}
