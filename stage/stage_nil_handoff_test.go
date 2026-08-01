package stage

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thalesfsp/dal/v2/memory"
	"github.com/thalesfsp/etler/v4/converters/passthru"
	"github.com/thalesfsp/etler/v4/processor"
	"github.com/thalesfsp/etler/v4/processors/storage"
	"github.com/thalesfsp/etler/v4/task"
)

func TestStage_nilProcessorOutput_normalizedBeforeNextProcessor(t *testing.T) {
	filterAll, err := processor.New(
		"filter-all-nil-handoff",
		"filters out every item",
		func(ctx context.Context, processingData []int) ([]int, error) {
			var out []int

			for _, item := range processingData {
				if item < 0 {
					out = append(out, item)
				}
			}

			return out, nil
		},
	)
	require.NoError(t, err)

	var observed []int

	observe, err := processor.New(
		"observe-nil-handoff",
		"records its input",
		func(ctx context.Context, processingData []int) ([]int, error) {
			observed = processingData

			return processingData, nil
		},
	)
	require.NoError(t, err)

	stg, err := New(
		"stage-nil-handoff",
		"normalizes nil output before the next processor",
		identityConverter(),
		filterAll, observe,
	)
	require.NoError(t, err)

	_, err = stg.Run(context.Background(), task.MustNew[int, int]([]int{1, 2, 3}))
	require.NoError(t, err)
	assert.NotNil(t, observed, "the second processor must observe an empty non-nil slice")
	assert.Empty(t, observed)
}

type nilHandoffRow struct {
	ID string `json:"id"`
}

func TestStage_filterAllBeforeStorage_succeeds(t *testing.T) {
	t.Run("empty non-nil input", func(t *testing.T) {
		memoryStorage, err := memory.New(context.Background())
		require.NoError(t, err)

		store, err := storage.New[nilHandoffRow](memoryStorage, 1, "nil-handoff-control-")
		require.NoError(t, err)

		stg, err := New(
			"stage-empty-storage-control",
			"storage accepts empty non-nil input",
			passthru.Must[nilHandoffRow](),
			store,
		)
		require.NoError(t, err)

		_, err = stg.Run(
			context.Background(),
			task.MustNew[nilHandoffRow, nilHandoffRow]([]nilHandoffRow{}),
		)
		require.NoError(t, err)
	})

	t.Run("nil filter output", func(t *testing.T) {
		filterAll, err := processor.New(
			"filter-all-before-storage",
			"filters out every item before storage",
			func(ctx context.Context, processingData []nilHandoffRow) ([]nilHandoffRow, error) {
				var out []nilHandoffRow

				for _, item := range processingData {
					if item.ID == "keep" {
						out = append(out, item)
					}
				}

				return out, nil
			},
		)
		require.NoError(t, err)

		memoryStorage, err := memory.New(context.Background())
		require.NoError(t, err)

		store, err := storage.New[nilHandoffRow](memoryStorage, 1, "nil-handoff-")
		require.NoError(t, err)

		stg, err := New(
			"stage-filter-all-storage",
			"storage accepts normalized filter output",
			passthru.Must[nilHandoffRow](),
			filterAll, store,
		)
		require.NoError(t, err)

		_, err = stg.Run(
			context.Background(),
			task.MustNew[nilHandoffRow, nilHandoffRow]([]nilHandoffRow{{ID: "discard"}}),
		)
		require.NoError(t, err)
	})
}

func TestStage_middleNilProcessorOutput_normalizedForEntireChain(t *testing.T) {
	seenNonNil := make([]bool, 0, 3)

	first, err := processor.New(
		"first-nil-handoff-chain",
		"records and forwards its input",
		func(ctx context.Context, processingData []int) ([]int, error) {
			seenNonNil = append(seenNonNil, processingData != nil)

			return processingData, nil
		},
	)
	require.NoError(t, err)

	middle, err := processor.New(
		"middle-nil-handoff-chain",
		"records its input and returns nil",
		func(ctx context.Context, processingData []int) ([]int, error) {
			seenNonNil = append(seenNonNil, processingData != nil)

			return nil, nil
		},
	)
	require.NoError(t, err)

	last, err := processor.New(
		"last-nil-handoff-chain",
		"records and forwards its input",
		func(ctx context.Context, processingData []int) ([]int, error) {
			seenNonNil = append(seenNonNil, processingData != nil)

			return processingData, nil
		},
	)
	require.NoError(t, err)

	stg, err := New(
		"stage-middle-nil-handoff-chain",
		"normalizes nil output throughout the processor chain",
		identityConverter(),
		first, middle, last,
	)
	require.NoError(t, err)

	_, err = stg.Run(context.Background(), task.MustNew[int, int]([]int{1}))
	require.NoError(t, err)
	assert.Equal(t, []bool{true, true, true}, seenNonNil,
		"every processor must observe a non-nil slice")
}
