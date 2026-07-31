package csv

import (
	stdcsv "encoding/csv"
	"errors"
	"io"
	"math"
	"os"
	"os/exec"
	"reflect"
	"strconv"
	"strings"
	"testing"

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
			name:         "int",
			inside:       strconv.FormatInt(int64(maxInt()), 10),
			outside:      strconv.FormatUint(uint64(maxInt())+1, 10),
			lowerInside:  strconv.FormatInt(int64(minInt()), 10),
			lowerOutside: intUnderflowValue(),
			load:         func(in string) (any, error) { return Load[[]struct{ N int }](strings.NewReader(in)) },
			insideWant:   maxInt(),
			lowerWant:    minInt(),
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
			name:         "int16",
			inside:       "32767",
			outside:      "32768",
			lowerInside:  "-32768",
			lowerOutside: "-32769",
			load:         func(in string) (any, error) { return Load[[]struct{ N int16 }](strings.NewReader(in)) },
			insideWant:   int16(32767),
			lowerWant:    int16(-32768),
		},
		{
			name:         "int32",
			inside:       "2147483647",
			outside:      "2147483648",
			lowerInside:  "-2147483648",
			lowerOutside: "-2147483649",
			load:         func(in string) (any, error) { return Load[[]struct{ N int32 }](strings.NewReader(in)) },
			insideWant:   int32(2147483647),
			lowerWant:    int32(-2147483648),
		},
		{
			name:         "int64",
			inside:       "9223372036854775807",
			outside:      "9223372036854775808",
			lowerInside:  "-9223372036854775808",
			lowerOutside: "-9223372036854775809",
			load:         func(in string) (any, error) { return Load[[]struct{ N int64 }](strings.NewReader(in)) },
			insideWant:   int64(math.MaxInt64),
			lowerWant:    int64(math.MinInt64),
		},
		{
			name:         "uint",
			inside:       strconv.FormatUint(uint64(maxUint()), 10),
			outside:      uintOverflowValue(),
			lowerInside:  "0",
			lowerOutside: "-1",
			load:         func(in string) (any, error) { return Load[[]struct{ N uint }](strings.NewReader(in)) },
			insideWant:   maxUint(),
			lowerWant:    uint(0),
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
			name:         "uint16",
			inside:       "65535",
			outside:      "65536",
			lowerInside:  "0",
			lowerOutside: "-1",
			load:         func(in string) (any, error) { return Load[[]struct{ N uint16 }](strings.NewReader(in)) },
			insideWant:   uint16(65535),
			lowerWant:    uint16(0),
		},
		{
			name:         "uint32",
			inside:       "4294967295",
			outside:      "4294967296",
			lowerInside:  "0",
			lowerOutside: "-1",
			load:         func(in string) (any, error) { return Load[[]struct{ N uint32 }](strings.NewReader(in)) },
			insideWant:   uint32(math.MaxUint32),
			lowerWant:    uint32(0),
		},
		{
			name:         "uint64",
			inside:       "18446744073709551615",
			outside:      "18446744073709551616",
			lowerInside:  "0",
			lowerOutside: "-1",
			load:         func(in string) (any, error) { return Load[[]struct{ N uint64 }](strings.NewReader(in)) },
			insideWant:   uint64(math.MaxUint64),
			lowerWant:    uint64(0),
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

			if testCase.lowerInside != "" {
				out, err = testCase.load("N\n" + testCase.lowerInside + "\n")
				require.NoError(t, err)
				require.Len(t, out, 1)
				assert.Equal(t, testCase.lowerWant, firstField(out))

				out, err = testCase.load("N\n" + testCase.lowerOutside + "\n")
				assert.Nil(t, out)
				require.Error(t, err)
				assert.ErrorContains(t, err, testCase.lowerOutside)
			}
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
			`value "127.9" in column "N" is not valid decimal input for int8`,
		)
		var parseErr *stdcsv.ParseError
		assert.False(t, errors.As(err, &parseErr))
	})

	t.Run("leading zero", func(t *testing.T) {
		out, err := Load[[]struct{ N int }](strings.NewReader("N\n010\n"))
		assert.Nil(t, out)
		require.EqualError(t, err,
			`value "010" in column "N" is not valid decimal input for int`,
		)
	})

	t.Run("underscore", func(t *testing.T) {
		out, err := Load[[]struct{ N int }](strings.NewReader("N\n1_0\n"))
		assert.Nil(t, out)
		require.EqualError(t, err,
			`value "1_0" in column "N" is not valid decimal input for int`,
		)
	})
}

func TestLoad_acceptsOrdinaryDecimalNumericSyntax(t *testing.T) {
	type row struct {
		Integer int
		Float   float64
	}

	out, err := Load[[]row](strings.NewReader(
		"Integer,Float\n+42,.5\n 42 ,1.\n-42,1e+3\n0, +1.5 \n,\n",
	))
	require.NoError(t, err)
	require.Equal(t, []row{
		{Integer: 42, Float: 0.5},
		{Integer: 42, Float: 1},
		{Integer: -42, Float: 1000},
		{Float: 1.5},
		{},
	}, out)
}

func TestLoad_rejectsOtherNonDecimalNumericSyntax(t *testing.T) {
	for _, value := range []string{
		"-01",
		"0x10",
		"0o10",
		"0b10",
	} {
		t.Run("integer "+value, func(t *testing.T) {
			out, err := Load[[]struct{ N int }](strings.NewReader("N\n" + value + "\n"))
			assert.Nil(t, out)
			require.Error(t, err)
			assert.ErrorContains(t, err, "not valid decimal input")
		})
	}

	for _, value := range []string{
		"1_0.5",
		"NaN",
		"Inf",
	} {
		t.Run("float "+value, func(t *testing.T) {
			out, err := Load[[]struct{ N float64 }](strings.NewReader("N\n" + value + "\n"))
			assert.Nil(t, out)
			require.Error(t, err)
			assert.ErrorContains(t, err, "not valid decimal input")
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

func TestLoad_delegatesHeaderResolutionToGocsv(t *testing.T) {
	t.Run("exact tag declared first wins over identical field key", func(t *testing.T) {
		type row struct {
			Tagged string `csv:"Name"`
			Name   string
		}

		out, err := Load[[]row](strings.NewReader("Name\nalice\n"))
		require.NoError(t, err)
		require.Equal(t, []row{{Tagged: "alice"}}, out)
	})

	t.Run("folded collision follows gocsv field order", func(t *testing.T) {
		type row struct {
			Foo string
			FOO string
		}

		out, err := Load[[]row](strings.NewReader("foo\nalice\n"))
		require.NoError(t, err)
		require.Equal(t, []row{{Foo: "alice"}}, out)
	})

	t.Run("duplicate exact tags follow gocsv field order", func(t *testing.T) {
		type row struct {
			First  string `csv:"name"`
			Second string `csv:"name"`
		}

		out, err := Load[[]row](strings.NewReader("name\nalice\n"))
		require.NoError(t, err)
		require.Equal(t, []row{{First: "alice"}}, out)
	})

	t.Run("untagged key declared first resolves without ambiguity", func(t *testing.T) {
		type row struct {
			Name   string
			Tagged string `csv:"Name"`
		}

		out, err := Load[[]row](strings.NewReader("Name\nalice\n"))
		require.NoError(t, err)
		require.Equal(t, []row{{Name: "alice"}}, out)
	})

	t.Run("partial tag declared first wins over later exact tag", func(t *testing.T) {
		type row struct {
			Partial string `csv:"id,partial"`
			Exact   string `csv:"user_id"`
		}

		out, err := Load[[]row](strings.NewReader("user_id\nalice\n"))
		require.NoError(t, err)
		require.Equal(t, []row{{Partial: "alice"}}, out)
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

func maxInt() int {
	return int(^uint(0) >> 1)
}

func minInt() int {
	return -maxInt() - 1
}

func maxUint() uint {
	return ^uint(0)
}

func intUnderflowValue() string {
	if strconv.IntSize == 64 {
		return "-9223372036854775809"
	}

	return "-2147483649"
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
