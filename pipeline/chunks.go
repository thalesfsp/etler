package pipeline

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/thalesfsp/customerror/v2"
	"github.com/thalesfsp/etler/v4/checkpoint"
	"github.com/thalesfsp/etler/v4/internal/shared"
	"github.com/thalesfsp/etler/v4/task"
)

//////
// Consts, vars and types.
//////

// ChunkOptions configures RunChunks. The zero value checkpoints in memory and
// stops at the first failed chunk.
type ChunkOptions[ProcessedData, ConvertedOut any] struct {
	// Store records each chunk's outcome. Chunks it holds as done are skipped,
	// which is how a rerun resumes. Nil uses a fresh in-memory store, so
	// nothing survives the call.
	Store checkpoint.Store

	// ContinueOnError records a failed chunk and runs the next one instead of
	// stopping the run.
	ContinueOnError bool

	// Key names a chunk within its run. Keys must be non-empty, unique, and
	// stable across reruns, or resume skips the wrong chunks. Nil uses the
	// chunk's zero-based index, which is stable only while the input is.
	Key func(index int, chunk []ProcessedData) string

	// Commit persists a chunk's output. It runs after the pipeline succeeds
	// and before the chunk is checkpointed as done; an error fails the chunk.
	// A crash between Commit and the checkpoint reruns the chunk, so Commit
	// must be idempotent.
	Commit func(ctx context.Context, key string, tasks []task.Task[ProcessedData, ConvertedOut]) error
}

// ChunkResult is one chunk's outcome in a RunChunks call.
type ChunkResult[ProcessedData, ConvertedOut any] struct {
	// Index is the chunk's position in the input.
	Index int

	// Key is the chunk's checkpoint key.
	Key string

	// Status is StatusDone, StatusFailed, or StatusSkipped.
	Status checkpoint.Status

	// Err is the chunk's failure, or the checkpoint write's failure; nil
	// otherwise.
	Err error

	// Tasks is the pipeline's output for a chunk that ran successfully; nil
	// for a skipped or failed chunk.
	Tasks []task.Task[ProcessedData, ConvertedOut]
}

//////
// Exported functionalities.
//////

// Chunk splits data into consecutive chunks of at most size items. The chunks
// share data's backing array but cannot append into each other.
func Chunk[T any](data []T, size int) ([][]T, error) {
	if size <= 0 {
		return nil, customerror.NewInvalidError("chunk size", customerror.WithField("size", size))
	}

	chunks := make([][]T, 0, (len(data)+size-1)/size)

	for start := 0; start < len(data); start += size {
		end := min(start+size, len(data))

		chunks = append(chunks, data[start:end:end])
	}

	return chunks, nil
}

// RunChunks runs p once per chunk, in order, and checkpoints every outcome
// under runID in opts.Store before moving on.
//
// Chunks the store already holds as done are skipped, so calling RunChunks
// again with the same runID and keys resumes: only failed and missing chunks
// run. A chunk that fails is recorded as failed; the run then stops, or with
// ContinueOnError runs the remaining chunks. A canceled context stops the run
// before the next chunk.
//
// It returns one result per chunk it reached, in input order, and the joined
// errors of every failed chunk. A store error stops the run: a Load error
// before any chunk runs, a Save error right after the chunk it was recording.
// Save calls ignore ctx's cancellation so an interrupted run still records
// what it finished.
func RunChunks[ProcessedData, ConvertedOut any](
	ctx context.Context,
	p IPipeline[ProcessedData, ConvertedOut],
	runID string,
	chunks [][]ProcessedData,
	opts ChunkOptions[ProcessedData, ConvertedOut],
) ([]ChunkResult[ProcessedData, ConvertedOut], error) {
	if shared.IsNil(p) {
		return nil, customerror.NewRequiredError("pipeline")
	}

	if runID == "" {
		return nil, customerror.NewRequiredError("runID")
	}

	keys, err := chunkKeys(chunks, opts.Key)
	if err != nil {
		return nil, err
	}

	store := opts.Store
	if shared.IsNil(store) {
		store = checkpoint.NewMemory()
	}

	records, err := store.Load(ctx, runID)
	if err != nil {
		return nil, customerror.NewFailedToError(
			"load checkpoints",
			customerror.WithError(err),
			customerror.WithField("runID", runID),
		)
	}

	saveCtx := context.WithoutCancel(ctx)

	results := make([]ChunkResult[ProcessedData, ConvertedOut], 0, len(chunks))

	var errs []error

	for index, chunk := range chunks {
		key := keys[index]
		previous := records[key]

		if previous.Status == checkpoint.StatusDone {
			results = append(results, ChunkResult[ProcessedData, ConvertedOut]{
				Index:  index,
				Key:    key,
				Status: checkpoint.StatusSkipped,
			})

			continue
		}

		if err := ctx.Err(); err != nil {
			errs = append(errs, err)

			break
		}

		result, record := runChunk(ctx, p, opts.Commit, index, key, chunk)
		record.Attempts = previous.Attempts + 1

		if err := store.Save(saveCtx, runID, record); err != nil {
			saveErr := customerror.NewFailedToError(
				"save checkpoint",
				customerror.WithError(err),
				customerror.WithField("runID", runID),
				customerror.WithField("chunk", key),
			)

			result.Err = errors.Join(result.Err, saveErr)
			results = append(results, result)
			errs = append(errs, result.Err)

			break
		}

		results = append(results, result)

		if result.Err != nil {
			errs = append(errs, result.Err)

			if !opts.ContinueOnError {
				break
			}
		}
	}

	return results, errors.Join(errs...)
}

//////
// Helpers.
//////

// chunkKeys resolves and validates every chunk's key before anything runs.
func chunkKeys[ProcessedData any](
	chunks [][]ProcessedData,
	keyOf func(index int, chunk []ProcessedData) string,
) ([]string, error) {
	keys := make([]string, len(chunks))
	seen := make(map[string]int, len(chunks))

	for index, chunk := range chunks {
		key := strconv.Itoa(index)
		if keyOf != nil {
			key = keyOf(index, chunk)
		}

		if key == "" {
			return nil, customerror.NewInvalidError("chunk key", customerror.WithField("index", index))
		}

		if first, ok := seen[key]; ok {
			return nil, customerror.NewInvalidError(
				"chunk key: duplicate",
				customerror.WithField("key", key),
				customerror.WithField("index", index),
				customerror.WithField("firstIndex", first),
			)
		}

		seen[key] = index
		keys[index] = key
	}

	return keys, nil
}

// runChunk runs and commits one chunk and builds its result and checkpoint
// record, leaving Attempts to the caller.
func runChunk[ProcessedData, ConvertedOut any](
	ctx context.Context,
	p IPipeline[ProcessedData, ConvertedOut],
	commit func(ctx context.Context, key string, tasks []task.Task[ProcessedData, ConvertedOut]) error,
	index int,
	key string,
	chunk []ProcessedData,
) (ChunkResult[ProcessedData, ConvertedOut], checkpoint.Record) {
	tasks, err := p.Run(ctx, chunk)
	if err == nil && commit != nil {
		err = commit(ctx, key, tasks)
	}

	result := ChunkResult[ProcessedData, ConvertedOut]{Index: index, Key: key}
	record := checkpoint.Record{Key: key, UpdatedAt: time.Now()}

	if err != nil {
		result.Status = checkpoint.StatusFailed
		result.Err = customerror.NewFailedToError(
			"run chunk",
			customerror.WithError(err),
			customerror.WithField("chunk", key),
		)

		record.Status = checkpoint.StatusFailed
		record.Error = err.Error()

		return result, record
	}

	result.Status = checkpoint.StatusDone
	result.Tasks = tasks
	record.Status = checkpoint.StatusDone

	return result, record
}
