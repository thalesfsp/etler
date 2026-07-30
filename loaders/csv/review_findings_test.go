package csv

import (
	"errors"
	"io"
	"math"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/gocarina/gocsv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_rejectsNumericOverflow(t *testing.T) {
	type testCase struct {
		name       string
		inside     string
		outside    string
		load       func(string) (any, error)
		insideWant any
	}

	testCases := []testCase{
		{
			name:       "int8",
			inside:     "127",
			outside:    "128",
			load:       func(in string) (any, error) { return Load[[]struct{ N int8 }](strings.NewReader(in)) },
			insideWant: int8(127),
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
			name:       "int64",
			inside:     "9223372036854775807",
			outside:    "9223372036854775808",
			load:       func(in string) (any, error) { return Load[[]struct{ N int64 }](strings.NewReader(in)) },
			insideWant: int64(math.MaxInt64),
		},
		{
			name:       "uint8",
			inside:     "255",
			outside:    "256",
			load:       func(in string) (any, error) { return Load[[]struct{ N uint8 }](strings.NewReader(in)) },
			insideWant: uint8(255),
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
			insideWant: uint32(math.MaxUint32),
		},
		{
			name:       "uint64",
			inside:     "18446744073709551615",
			outside:    "18446744073709551616",
			load:       func(in string) (any, error) { return Load[[]struct{ N uint64 }](strings.NewReader(in)) },
			insideWant: uint64(math.MaxUint64),
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
			assert.ErrorContains(t, err, "out of range")
		})
	}
}

func TestLoad_rejectsOverflowInTaggedNonFirstField(t *testing.T) {
	type row struct {
		Name  string
		Count uint8 `csv:"count"`
	}

	out, err := Load[[]row](strings.NewReader("Name,count\nalice,256\n"))
	assert.Nil(t, out)
	require.Error(t, err)
	assert.ErrorContains(t, err, "out of range")
}

func TestLoad_rejectsOverflowThroughPartialTag(t *testing.T) {
	type row struct {
		Count uint8 `csv:"number,partial"`
	}

	out, err := Load[[]row](strings.NewReader("total_number\n256\n"))
	assert.Nil(t, out)
	require.Error(t, err)
	assert.ErrorContains(t, err, "out of range")
}

func TestLoad_rejectsNonDecimalIntegerSyntax(t *testing.T) {
	t.Run("fraction", func(t *testing.T) {
		out, err := Load[[]struct{ N int8 }](strings.NewReader("N\n127.9\n"))
		assert.Nil(t, out)
		require.EqualError(t, err,
			`parse error on line 2, column 1: value "127.9" in column "N" does not use strict decimal syntax for int8`,
		)
	})

	t.Run("leading zero", func(t *testing.T) {
		out, err := Load[[]struct{ N int }](strings.NewReader("N\n010\n"))
		assert.Nil(t, out)
		require.EqualError(t, err,
			`parse error on line 2, column 1: value "010" in column "N" does not use strict decimal syntax for int`,
		)
	})

	t.Run("underscore", func(t *testing.T) {
		out, err := Load[[]struct{ N int }](strings.NewReader("N\n1_0\n"))
		assert.Nil(t, out)
		require.EqualError(t, err,
			`parse error on line 2, column 1: value "1_0" in column "N" does not use strict decimal syntax for int`,
		)
	})
}

func TestLoad_acceptsStrictDecimalNumericSyntax(t *testing.T) {
	type row struct {
		Integer int
		Float   float64
	}

	out, err := Load[[]row](strings.NewReader(
		"Integer,Float\n42,1.5\n-42,-1.5\n0,1e3\n,\n",
	))
	require.NoError(t, err)
	require.Equal(t, []row{
		{Integer: 42, Float: 1.5},
		{Integer: -42, Float: -1.5},
		{Float: 1000},
		{},
	}, out)
}

func TestLoad_rejectsOtherNonDecimalNumericSyntax(t *testing.T) {
	for _, value := range []string{
		"+42",
		"-01",
		"0x10",
		"0o10",
		"0b10",
		" 42",
		"42 ",
	} {
		t.Run("integer "+value, func(t *testing.T) {
			out, err := Load[[]struct{ N int }](strings.NewReader("N\n" + value + "\n"))
			assert.Nil(t, out)
			require.Error(t, err)
			assert.ErrorContains(t, err, "strict decimal syntax")
		})
	}

	for _, value := range []string{
		"+1.5",
		"1_0.5",
		" 1.5",
		"1.5 ",
		".5",
		"1.",
		"NaN",
		"Inf",
		"1e+3",
	} {
		t.Run("float "+value, func(t *testing.T) {
			out, err := Load[[]struct{ N float64 }](strings.NewReader("N\n" + value + "\n"))
			assert.Nil(t, out)
			require.Error(t, err)
			assert.ErrorContains(t, err, "strict decimal syntax")
		})
	}
}

func TestLoad_leavesNonNumericFieldsUntouchedByNumericValidation(t *testing.T) {
	type row struct {
		Text string
		N    int
	}

	out, err := Load[[]row](strings.NewReader("Text,N\n 1_0 ,42\n"))
	require.NoError(t, err)
	require.Equal(t, []row{{Text: " 1_0 ", N: 42}}, out)
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

	t.Run("tag remains case sensitive", func(t *testing.T) {
		type row struct {
			Name string `csv:"display_name"`
		}

		out, err := Load[[]row](strings.NewReader("name,display_name\nwrong,alice\n"))
		require.NoError(t, err)
		require.Equal(t, []row{{Name: "alice"}}, out)
	})
}

func TestLoad_resolvesHeaderMatchesDeterministically(t *testing.T) {
	t.Run("exact tag wins over folded field", func(t *testing.T) {
		type row struct {
			Tagged string `csv:"name"`
			Name   string
		}

		out, err := Load[[]row](strings.NewReader("name\nalice\n"))
		require.NoError(t, err)
		require.Equal(t, []row{{Tagged: "alice"}}, out)
	})

	t.Run("folded fields are ambiguous", func(t *testing.T) {
		type row struct {
			Foo string
			FOO string
		}

		out, err := Load[[]row](strings.NewReader("foo\nalice\n"))
		assert.Nil(t, out)
		require.Error(t, err)
		assert.ErrorContains(t, err, `ambiguous CSV header "foo"`)
		assert.ErrorContains(t, err, "Foo")
		assert.ErrorContains(t, err, "FOO")
	})

	t.Run("duplicate exact tags are ambiguous", func(t *testing.T) {
		type row struct {
			First  string `csv:"name"`
			Second string `csv:"name"`
		}

		out, err := Load[[]row](strings.NewReader("name\nalice\n"))
		assert.Nil(t, out)
		require.Error(t, err)
		assert.ErrorContains(t, err, `ambiguous CSV header "name"`)
		assert.ErrorContains(t, err, "First")
		assert.ErrorContains(t, err, "Second")
	})

	t.Run("identical tag and field key are ambiguous", func(t *testing.T) {
		type row struct {
			Name   string
			Tagged string `csv:"Name"`
		}

		out, err := Load[[]row](strings.NewReader("Name\nalice\n"))
		assert.Nil(t, out)
		require.Error(t, err)
		assert.ErrorContains(t, err, `ambiguous CSV header "Name"`)
		assert.ErrorContains(t, err, "Name")
		assert.ErrorContains(t, err, "Tagged")
	})
}

func TestLoad_delegatesTagOptionsToGocsvUnmarshaller(t *testing.T) {
	t.Run("default is not applied", func(t *testing.T) {
		type row struct {
			Tagged int `csv:"name,default=7"`
			Name   int
			Note   string
		}

		out, err := Load[[]row](strings.NewReader("name,Note\n,kept\n"))
		require.NoError(t, err)
		require.Equal(t, []row{{Note: "kept"}}, out)
	})

	t.Run("omitempty leaves pointer nil", func(t *testing.T) {
		type row struct {
			Tagged *int `csv:"name,omitempty"`
			Name   int
			Note   string
		}

		out, err := Load[[]row](strings.NewReader("name,Note\n,kept\n"))
		require.NoError(t, err)
		require.Equal(t, []row{{Note: "kept"}}, out)
	})
}

func TestLoad_nilReaderReturnsRequiredArgumentError(t *testing.T) {
	assert.NotPanics(t, func() {
		out, err := Load[[]struct{ Name string }](nil)
		assert.Nil(t, out)
		assert.EqualError(t, err, "CSV reader required")
	})
}

func TestLoad_gocsvPackageGlobals(t *testing.T) {
	t.Run("custom reader factory does not influence Load", func(t *testing.T) {
		injectedErr := errors.New("global gocsv reader factory used")
		gocsv.SetCSVReader(func(io.Reader) gocsv.CSVReader {
			return errorCSVReader{err: injectedErr}
		})
		defer gocsv.SetCSVReader(gocsv.DefaultCSVReader)

		out, err := Load[[]struct{ Name string }](strings.NewReader("Name\nalice\n"))
		require.NoError(t, err)
		require.Equal(t, []struct{ Name string }{{Name: "alice"}}, out)
	})

	t.Run("unmatched struct tag flag does not influence Load", func(t *testing.T) {
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

	t.Run("duplicate header flag influences Load", func(t *testing.T) {
		gocsv.FailIfDoubleHeaderNames = true
		defer func() {
			gocsv.FailIfDoubleHeaderNames = false
		}()

		out, err := Load[[]struct{ Name string }](strings.NewReader("Name,Name\nfirst,last\n"))
		assert.Nil(t, out)
		assert.ErrorIs(t, err, gocsv.ErrDoubleHeaderNames)
	})

	t.Run("duplicate header flag applies after folding", func(t *testing.T) {
		gocsv.FailIfDoubleHeaderNames = true
		defer func() {
			gocsv.FailIfDoubleHeaderNames = false
		}()

		out, err := Load[[]struct{ Name string }](strings.NewReader("Name,name\nfirst,last\n"))
		assert.Nil(t, out)
		assert.ErrorIs(t, err, gocsv.ErrDoubleHeaderNames)
	})
}

func TestLoad_rejectsCyclicOutputType(t *testing.T) {
	const helperEnvironment = "ETLER_CSV_CYCLE_HELPER"
	switch os.Getenv(helperEnvironment) {
	case "pointer":
		out, err := Load[[]PointerCycle](strings.NewReader("Name\nalice\n"))
		assert.Nil(t, out)
		require.Error(t, err)
		assert.ErrorContains(t, err, "cyclic CSV output type")

		return
	case "slice":
		out, err := Load[[]SliceCycle](strings.NewReader("Name\nalice\n"))
		assert.Nil(t, out)
		require.Error(t, err)
		assert.ErrorContains(t, err, "cyclic CSV output type")

		return
	}

	for _, cycle := range []string{"pointer", "slice"} {
		command := exec.Command(os.Args[0], "-test.run=^TestLoad_rejectsCyclicOutputType$")
		command.Env = append(os.Environ(), helperEnvironment+"="+cycle)
		require.NoError(t, command.Run(), "%s cycle crashed subprocess", cycle)
	}
}

type PointerCycle struct {
	*PointerCycle
}

type SliceCycle struct {
	Children []SliceCycle
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

type customInt int

func (value *customInt) UnmarshalCSV(string) error {
	*value = 42

	return nil
}
