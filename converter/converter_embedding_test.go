package converter

import (
	"context"
	"os"
	"os/exec"
	"runtime/debug"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const converterCycleChild = "ETLER_CONVERTER_CYCLE_CHILD"

type ConverterNilImplementation struct {
	IConverter[int, int]
}

type ConverterTwoLevelNilWrapper struct {
	ConverterNilImplementation
}

type ConverterThreeLevelNilWrapper struct {
	ConverterTwoLevelNilWrapper
}

type converterUnexportedNilImplementation struct {
	IConverter[int, int]
}

type ConverterUnexportedNilWrapper struct {
	converterUnexportedNilImplementation
}

type ConverterCycleNilWrapper struct {
	*ConverterCycleNilWrapper
	ConverterNilImplementation
}

type ConverterValidWrapper struct {
	IConverter[int, int]
}

type ConverterValidOuter struct {
	ConverterValidWrapper
}

func TestNew_rejectsNilEmbeddedConverterAtAnyDepth(t *testing.T) {
	tests := []struct {
		name  string
		value IConverter[int, int]
	}{
		{
			name:  "two-level",
			value: &ConverterTwoLevelNilWrapper{},
		},
		{
			name:  "three-level",
			value: &ConverterThreeLevelNilWrapper{},
		},
		{
			name:  "unexported embedded type",
			value: &ConverterUnexportedNilWrapper{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				converter IConverter[int, int]
				err       error
			)

			assert.NotPanics(t, func() {
				converter, err = New(
					"converter-nested-nil-"+tt.name,
					"converter nested nil",
					func(_ context.Context, in int) (int, error) {
						return in, nil
					},
					func(IConverter[int, int]) IConverter[int, int] {
						return tt.value
					},
				)
			})

			assert.Nil(t, converter)
			require.EqualError(t, err, "converter option result required")
		})
	}
}

func TestNew_rejectsNilEmbeddedConverterThroughCycle(t *testing.T) {
	if os.Getenv(converterCycleChild) == "1" {
		debug.SetMaxStack(1 << 20)

		value := &ConverterCycleNilWrapper{}
		value.ConverterCycleNilWrapper = value

		var (
			converter IConverter[int, int]
			err       error
		)

		assert.NotPanics(t, func() {
			converter, err = New(
				"converter-cycle-nil-child",
				"converter cycle nil",
				func(_ context.Context, in int) (int, error) {
					return in, nil
				},
				func(IConverter[int, int]) IConverter[int, int] {
					return value
				},
			)
		})

		assert.Nil(t, converter)
		require.EqualError(t, err, "converter option result required")

		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestNew_rejectsNilEmbeddedConverterThroughCycle$")
	cmd.Env = append(os.Environ(),
		converterCycleChild+"=1",
		"GOTRACEBACK=none",
	)

	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "cyclic converter subprocess failed:\n%s", output)
}

func TestNew_acceptsValidConverterEmbeddingDepths(t *testing.T) {
	tests := []struct {
		name string
		wrap func(IConverter[int, int]) IConverter[int, int]
	}{
		{
			name: "direct",
			wrap: func(converter IConverter[int, int]) IConverter[int, int] {
				return converter
			},
		},
		{
			name: "one-level",
			wrap: func(converter IConverter[int, int]) IConverter[int, int] {
				return &ConverterValidWrapper{IConverter: converter}
			},
		},
		{
			name: "two-level",
			wrap: func(converter IConverter[int, int]) IConverter[int, int] {
				return &ConverterValidOuter{
					ConverterValidWrapper: ConverterValidWrapper{IConverter: converter},
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converter, err := New(
				"converter-valid-embedding-"+tt.name,
				"converter valid embedding",
				func(_ context.Context, in int) (int, error) {
					return in, nil
				},
				func(converter IConverter[int, int]) IConverter[int, int] {
					return tt.wrap(converter)
				},
			)

			require.NoError(t, err)
			assert.NotNil(t, converter)
		})
	}
}
