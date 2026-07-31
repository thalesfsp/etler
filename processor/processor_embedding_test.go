package processor

import (
	"context"
	"os"
	"os/exec"
	"runtime/debug"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const processorCycleChild = "ETLER_PROCESSOR_CYCLE_CHILD"

type ProcessorNilImplementation struct {
	IProcessor[int]
}

type ProcessorTwoLevelNilWrapper struct {
	ProcessorNilImplementation
}

type ProcessorThreeLevelNilWrapper struct {
	ProcessorTwoLevelNilWrapper
}

type processorUnexportedNilImplementation struct {
	IProcessor[int]
}

type ProcessorUnexportedNilWrapper struct {
	processorUnexportedNilImplementation
}

type ProcessorCycleNilWrapper struct {
	*ProcessorCycleNilWrapper
	ProcessorNilImplementation
}

type ProcessorValidWrapper struct {
	IProcessor[int]
}

type ProcessorValidOuter struct {
	ProcessorValidWrapper
}

func TestNew_rejectsNilEmbeddedProcessorAtAnyDepth(t *testing.T) {
	tests := []struct {
		name  string
		value IProcessor[int]
	}{
		{
			name:  "two-level",
			value: &ProcessorTwoLevelNilWrapper{},
		},
		{
			name:  "three-level",
			value: &ProcessorThreeLevelNilWrapper{},
		},
		{
			name:  "unexported embedded type",
			value: &ProcessorUnexportedNilWrapper{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				processor IProcessor[int]
				err       error
			)

			assert.NotPanics(t, func() {
				processor, err = New(
					"processor-nested-nil-"+tt.name,
					"processor nested nil",
					func(_ context.Context, data []int) ([]int, error) {
						return data, nil
					},
					func(IProcessor[int]) IProcessor[int] {
						return tt.value
					},
				)
			})

			assert.Nil(t, processor)
			require.EqualError(t, err, "processor option result required")
		})
	}
}

func TestNew_rejectsNilEmbeddedProcessorThroughCycle(t *testing.T) {
	if os.Getenv(processorCycleChild) == "1" {
		debug.SetMaxStack(1 << 20)

		value := &ProcessorCycleNilWrapper{}
		value.ProcessorCycleNilWrapper = value

		var (
			processor IProcessor[int]
			err       error
		)

		assert.NotPanics(t, func() {
			processor, err = New(
				"processor-cycle-nil-child",
				"processor cycle nil",
				func(_ context.Context, data []int) ([]int, error) {
					return data, nil
				},
				func(IProcessor[int]) IProcessor[int] {
					return value
				},
			)
		})

		assert.Nil(t, processor)
		require.EqualError(t, err, "processor option result required")

		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestNew_rejectsNilEmbeddedProcessorThroughCycle$")
	cmd.Env = append(os.Environ(),
		processorCycleChild+"=1",
		"GOTRACEBACK=none",
	)

	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "cyclic processor subprocess failed:\n%s", output)
}

func TestNew_acceptsValidProcessorEmbeddingDepths(t *testing.T) {
	tests := []struct {
		name string
		wrap func(IProcessor[int]) IProcessor[int]
	}{
		{
			name: "direct",
			wrap: func(processor IProcessor[int]) IProcessor[int] {
				return processor
			},
		},
		{
			name: "one-level",
			wrap: func(processor IProcessor[int]) IProcessor[int] {
				return &ProcessorValidWrapper{IProcessor: processor}
			},
		},
		{
			name: "two-level",
			wrap: func(processor IProcessor[int]) IProcessor[int] {
				return &ProcessorValidOuter{
					ProcessorValidWrapper: ProcessorValidWrapper{IProcessor: processor},
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor, err := New(
				"processor-valid-embedding-"+tt.name,
				"processor valid embedding",
				func(_ context.Context, data []int) ([]int, error) {
					return data, nil
				},
				func(processor IProcessor[int]) IProcessor[int] {
					return tt.wrap(processor)
				},
			)

			require.NoError(t, err)
			assert.NotNil(t, processor)
		})
	}
}
