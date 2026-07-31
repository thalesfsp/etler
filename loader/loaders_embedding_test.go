package loader

import (
	"context"
	"os"
	"os/exec"
	"runtime/debug"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const loaderCycleChild = "ETLER_LOADER_CYCLE_CHILD"

type LoaderNilImplementation struct {
	ILoader[string, []int]
}

type LoaderTwoLevelNilWrapper struct {
	LoaderNilImplementation
}

type LoaderThreeLevelNilWrapper struct {
	LoaderTwoLevelNilWrapper
}

type loaderUnexportedNilImplementation struct {
	ILoader[string, []int]
}

type LoaderUnexportedNilWrapper struct {
	loaderUnexportedNilImplementation
}

type LoaderCycleNilWrapper struct {
	*LoaderCycleNilWrapper
	LoaderNilImplementation
}

type LoaderValidWrapper struct {
	ILoader[string, []int]
}

type LoaderValidOuter struct {
	LoaderValidWrapper
}

func TestNew_rejectsNilEmbeddedLoaderAtAnyDepth(t *testing.T) {
	tests := []struct {
		name  string
		value ILoader[string, []int]
	}{
		{
			name:  "two-level",
			value: &LoaderTwoLevelNilWrapper{},
		},
		{
			name:  "three-level",
			value: &LoaderThreeLevelNilWrapper{},
		},
		{
			name:  "unexported embedded type",
			value: &LoaderUnexportedNilWrapper{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				loader ILoader[string, []int]
				err    error
			)

			assert.NotPanics(t, func() {
				loader, err = New(
					"loader-nested-nil-"+tt.name,
					"loader nested nil",
					func(_ context.Context, in string) ([]int, error) {
						return []int{len(in)}, nil
					},
					func(ILoader[string, []int]) ILoader[string, []int] {
						return tt.value
					},
				)
			})

			assert.Nil(t, loader)
			require.EqualError(t, err, "loader option result required")
		})
	}
}

func TestNew_rejectsNilEmbeddedLoaderThroughCycle(t *testing.T) {
	if os.Getenv(loaderCycleChild) == "1" {
		debug.SetMaxStack(1 << 20)

		value := &LoaderCycleNilWrapper{}
		value.LoaderCycleNilWrapper = value

		var (
			loader ILoader[string, []int]
			err    error
		)

		assert.NotPanics(t, func() {
			loader, err = New(
				"loader-cycle-nil-child",
				"loader cycle nil",
				func(_ context.Context, in string) ([]int, error) {
					return []int{len(in)}, nil
				},
				func(ILoader[string, []int]) ILoader[string, []int] {
					return value
				},
			)
		})

		assert.Nil(t, loader)
		require.EqualError(t, err, "loader option result required")

		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestNew_rejectsNilEmbeddedLoaderThroughCycle$")
	cmd.Env = append(os.Environ(),
		loaderCycleChild+"=1",
		"GOTRACEBACK=none",
	)

	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "cyclic loader subprocess failed:\n%s", output)
}

func TestNew_acceptsValidLoaderEmbeddingDepths(t *testing.T) {
	tests := []struct {
		name string
		wrap func(ILoader[string, []int]) ILoader[string, []int]
	}{
		{
			name: "direct",
			wrap: func(loader ILoader[string, []int]) ILoader[string, []int] {
				return loader
			},
		},
		{
			name: "one-level",
			wrap: func(loader ILoader[string, []int]) ILoader[string, []int] {
				return &LoaderValidWrapper{ILoader: loader}
			},
		},
		{
			name: "two-level",
			wrap: func(loader ILoader[string, []int]) ILoader[string, []int] {
				return &LoaderValidOuter{
					LoaderValidWrapper: LoaderValidWrapper{ILoader: loader},
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loader, err := New(
				"loader-valid-embedding-"+tt.name,
				"loader valid embedding",
				func(_ context.Context, in string) ([]int, error) {
					return []int{len(in)}, nil
				},
				func(loader ILoader[string, []int]) ILoader[string, []int] {
					return tt.wrap(loader)
				},
			)

			require.NoError(t, err)
			assert.NotNil(t, loader)
		})
	}
}
