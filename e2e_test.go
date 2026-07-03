package etler_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thalesfsp/dal/v2/memory"
	"github.com/thalesfsp/etler/v3/converter"
	"github.com/thalesfsp/etler/v3/converters/passthru"
	loadercsv "github.com/thalesfsp/etler/v3/loaders/csv"
	"github.com/thalesfsp/etler/v3/pipeline"
	"github.com/thalesfsp/etler/v3/processor"
	storageproc "github.com/thalesfsp/etler/v3/processors/storage"
	"github.com/thalesfsp/etler/v3/stage"
	"github.com/thalesfsp/params/v2/list"
)

type person struct {
	Name string `json:"name"`
	Age  int    `json:"age,string"`
}

// E2E happy path: CSV text -> loader -> two sequential stages (uppercase,
// then suffix) -> final converted data.
func TestE2E_csvToPipeline_sequential(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	//////
	// Load: CSV -> []person.
	//////

	csvLoader, err := loadercsv.New[person]()
	require.NoError(t, err)

	records, err := csvLoader.Run(ctx, strings.NewReader("name,age\nalice,30\nbob,0\n"))
	require.NoError(t, err)
	require.Len(t, records, 2)
	require.Equal(t, person{Name: "alice", Age: 30}, records[0])
	require.Equal(t, person{Name: "bob", Age: 0}, records[1])

	//////
	// Transform: two stages.
	//////

	uppercase, err := processor.New(
		"uppercase-e2e",
		"uppercases names",
		func(ctx context.Context, processingData []person) ([]person, error) {
			out := make([]person, len(processingData))

			for i, v := range processingData {
				out[i] = v
				out[i].Name = strings.ToUpper(v.Name)
			}

			return out, nil
		},
	)
	require.NoError(t, err)

	suffix, err := processor.New(
		"suffix-e2e",
		"appends a marker to names",
		func(ctx context.Context, processingData []person) ([]person, error) {
			out := make([]person, len(processingData))

			for i, v := range processingData {
				out[i] = v
				out[i].Name = v.Name + "-ok"
			}

			return out, nil
		},
	)
	require.NoError(t, err)

	identityConv := converter.MustDefault(
		func(ctx context.Context, in person) (person, error) {
			return in, nil
		},
	)

	stg1, err := stage.New("stage-uppercase-e2e", "uppercase stage", identityConv, uppercase)
	require.NoError(t, err)

	stg2, err := stage.New("stage-suffix-e2e", "suffix stage", identityConv, suffix)
	require.NoError(t, err)

	p, err := pipeline.New("e2e-pipeline", "csv to enriched people", false, stg1, stg2)
	require.NoError(t, err)

	out, err := p.Run(ctx, records)
	require.NoError(t, err)

	// v3: one task per stage, in stage order; the final task is last.
	require.Len(t, out, 2)

	final := out[len(out)-1]

	// Sequential pipelines retro-feed ProcessingData stage to stage.
	assert.Equal(t, []person{
		{Name: "ALICE-ok", Age: 30},
		{Name: "BOB-ok", Age: 0},
	}, final.ConvertedData)

	// The intermediate stage's results are exposed too.
	assert.Equal(t, []person{
		{Name: "ALICE", Age: 30},
		{Name: "BOB", Age: 0},
	}, out[0].ConvertedData)

	assert.NotEmpty(t, final.ID)
	assert.NotEmpty(t, final.CreatedAt)
}

// E2E happy path: pipeline with a storage processor persisting into the DAL
// memory storage; the stored documents must be listable afterwards.
func TestE2E_pipelineWithStorageProcessor(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	memoryStorage, err := memory.New(ctx)
	require.NoError(t, err)

	store, err := storageproc.New[person](memoryStorage, 2, "e2e-audit-")
	require.NoError(t, err)

	stg, err := stage.New(
		"stage-storage-e2e",
		"persists people",
		passthru.Must[person](),
		store,
	)
	require.NoError(t, err)

	p, err := pipeline.New("e2e-storage-pipeline", "persists people", false, stg)
	require.NoError(t, err)

	people := []person{
		{Name: "alice", Age: 30},
		{Name: "bob", Age: 0},
		{Name: "carol", Age: 41},
	}

	out, err := p.Run(ctx, people)
	require.NoError(t, err)
	require.Len(t, out, 1)

	// The storage processor passes data through unchanged.
	assert.Equal(t, people, out[0].ConvertedData)

	var stored memory.ResponseList[person]
	require.NoError(t, memoryStorage.List(ctx, "etl", &stored, &list.List{}))
	assert.Len(t, stored.Items, len(people))
}

// E2E bad path: a CSV whose values cannot be mapped to the target struct must
// fail at load time with the cause preserved.
func TestE2E_csvLoader_typeMismatch_failsLoad(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	csvLoader, err := loadercsv.New[person]()
	require.NoError(t, err)

	out, err := csvLoader.Run(ctx, strings.NewReader("name,age\nalice,not-a-number\n"))
	assert.Nil(t, out)
	assert.Error(t, err)
}
