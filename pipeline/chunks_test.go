package pipeline

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thalesfsp/etler/v4/checkpoint"
	"github.com/thalesfsp/etler/v4/converters/passthru"
	"github.com/thalesfsp/etler/v4/processor"
	"github.com/thalesfsp/etler/v4/stage"
	"github.com/thalesfsp/etler/v4/task"
)

// errChunk is the failure the test processor returns for a chunk set to fail.
var errChunk = errors.New("upstream timed out")

// chunkRecorder is a test processor that records which chunks ran and fails
// the chunks in failOn. Each chunk holds one int, its own index.
type chunkRecorder struct {
	mu     sync.Mutex
	ran    []int
	failOn map[int]bool
}

func (r *chunkRecorder) run(_ context.Context, in []int) ([]int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.ran = append(r.ran, in[0])

	if r.failOn[in[0]] {
		return nil, errChunk
	}

	out := make([]int, len(in))
	for i, v := range in {
		out[i] = v * 10
	}

	return out, nil
}

func (r *chunkRecorder) ranChunks() []int {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]int(nil), r.ran...)
}

// newChunkPipeline builds a one-stage pipeline around r.
func newChunkPipeline(t *testing.T, name string, r *chunkRecorder) IPipeline[int, int] {
	t.Helper()

	proc, err := processor.New(name+"-proc", "records chunks", r.run)
	require.NoError(t, err)

	stg, err := stage.New(name+"-stage", "records chunks", passthru.Must[int](), proc)
	require.NoError(t, err)

	p, err := New(name, "records chunks", false, stg)
	require.NoError(t, err)

	return p
}

// twelveChunks returns 12 one-item chunks; chunk i holds i.
func twelveChunks(t *testing.T) [][]int {
	t.Helper()

	data := make([]int, 12)
	for i := range data {
		data[i] = i
	}

	chunks, err := Chunk(data, 1)
	require.NoError(t, err)
	require.Len(t, chunks, 12)

	return chunks
}

func indexes(from, to int) []int {
	out := []int{}
	for i := from; i <= to; i++ {
		out = append(out, i)
	}

	return out
}

func statuses[P, C any](results []ChunkResult[P, C]) map[checkpoint.Status][]int {
	out := map[checkpoint.Status][]int{}
	for _, r := range results {
		out[r.Status] = append(out[r.Status], r.Index)
	}

	return out
}

func TestChunk(t *testing.T) {
	chunks, err := Chunk([]int{1, 2, 3, 4, 5}, 2)
	require.NoError(t, err)
	assert.Equal(t, [][]int{{1, 2}, {3, 4}, {5}}, chunks)

	exact, err := Chunk([]int{1, 2, 3, 4}, 2)
	require.NoError(t, err)
	assert.Equal(t, [][]int{{1, 2}, {3, 4}}, exact)

	larger, err := Chunk([]int{1, 2}, 500)
	require.NoError(t, err)
	assert.Equal(t, [][]int{{1, 2}}, larger)

	empty, err := Chunk([]int{}, 3)
	require.NoError(t, err)
	assert.Empty(t, empty)

	for _, size := range []int{0, -1} {
		_, err := Chunk([]int{1}, size)
		require.Error(t, err, "size %d", size)
		assert.Contains(t, err.Error(), "chunk size")
	}

	// A size larger than any length must not overflow.
	huge, err := Chunk([]int{1, 2}, math.MaxInt)
	require.NoError(t, err)
	assert.Equal(t, [][]int{{1, 2}}, huge)

	// Appending to one chunk must not overwrite the next one's items.
	chunks[0] = append(chunks[0], 99)
	assert.Equal(t, []int{3, 4}, chunks[1])
}

func TestRunChunks_allSucceed(t *testing.T) {
	r := &chunkRecorder{}
	p := newChunkPipeline(t, "chunks-all-succeed", r)
	store := checkpoint.NewMemory()

	var committed []string

	results, err := RunChunks(context.Background(), p, "run-1", twelveChunks(t), ChunkOptions[int, int]{
		Store: store,
		Commit: func(_ context.Context, key string, tasks []task.Task[int, int]) error {
			require.Len(t, tasks, 1)
			committed = append(committed, key)

			return nil
		},
	})
	require.NoError(t, err)

	assert.Equal(t, indexes(0, 11), r.ranChunks())
	assert.Equal(t, map[checkpoint.Status][]int{checkpoint.StatusDone: indexes(0, 11)}, statuses(results))
	assert.Len(t, committed, 12)
	assert.Equal(t, []int{70}, results[7].Tasks[0].ProcessingData)

	records, err := store.Load(context.Background(), "run-1")
	require.NoError(t, err)
	require.Len(t, records, 12)

	for key, rec := range records {
		assert.Equal(t, checkpoint.StatusDone, rec.Status, key)
		assert.Equal(t, 1, rec.Attempts, key)
		assert.Empty(t, rec.Error, key)
		assert.False(t, rec.UpdatedAt.IsZero(), key)
	}
}

// The BrainOS case: chunk 7 of 12 fails. With ContinueOnError the other 11
// commit, and the failure is recorded.
func TestRunChunks_middleFails_continueOnError(t *testing.T) {
	r := &chunkRecorder{failOn: map[int]bool{6: true}}
	p := newChunkPipeline(t, "chunks-middle-fails", r)
	store := checkpoint.NewMemory()

	var committed []string

	results, err := RunChunks(context.Background(), p, "run-1", twelveChunks(t), ChunkOptions[int, int]{
		Store:           store,
		ContinueOnError: true,
		Commit: func(_ context.Context, key string, _ []task.Task[int, int]) error {
			committed = append(committed, key)

			return nil
		},
	})
	require.Error(t, err)
	require.ErrorIs(t, err, errChunk)
	assert.Contains(t, err.Error(), "run chunk")

	assert.Equal(t, indexes(0, 11), r.ranChunks(), "chunks after the failure must still run")
	assert.Equal(t, []string{"0", "1", "2", "3", "4", "5", "7", "8", "9", "10", "11"}, committed)

	got := statuses(results)
	assert.Equal(t, []int{6}, got[checkpoint.StatusFailed])
	assert.Len(t, got[checkpoint.StatusDone], 11)
	require.ErrorIs(t, results[6].Err, errChunk)
	assert.Nil(t, results[6].Tasks)

	records, err := store.Load(context.Background(), "run-1")
	require.NoError(t, err)
	require.Len(t, records, 12)
	assert.Equal(t, checkpoint.StatusFailed, records["6"].Status)
	assert.Contains(t, records["6"].Error, errChunk.Error())
	assert.Equal(t, 1, records["6"].Attempts)
}

// Default options keep today's semantics: the run stops at the first failure.
// What finished before it is still checkpointed.
func TestRunChunks_defaultStopsAtFirstFailure(t *testing.T) {
	r := &chunkRecorder{failOn: map[int]bool{6: true}}
	p := newChunkPipeline(t, "chunks-default-stop", r)
	store := checkpoint.NewMemory()

	results, err := RunChunks(context.Background(), p, "run-1", twelveChunks(t), ChunkOptions[int, int]{Store: store})
	require.ErrorIs(t, err, errChunk)

	assert.Equal(t, indexes(0, 6), r.ranChunks(), "chunks after the failure must not run")
	require.Len(t, results, 7)
	assert.Equal(t, []int{6}, statuses(results)[checkpoint.StatusFailed])

	records, err := store.Load(context.Background(), "run-1")
	require.NoError(t, err)
	assert.Len(t, records, 7)
	assert.Equal(t, checkpoint.StatusFailed, records["6"].Status)
}

func TestRunChunks_resumeRunsOnlyFailedChunk(t *testing.T) {
	store := checkpoint.NewMemory()
	chunks := twelveChunks(t)

	first := &chunkRecorder{failOn: map[int]bool{6: true}}
	_, err := RunChunks(context.Background(), newChunkPipeline(t, "chunks-resume-1", first), "run-1", chunks,
		ChunkOptions[int, int]{Store: store, ContinueOnError: true})
	require.ErrorIs(t, err, errChunk)

	second := &chunkRecorder{}
	results, err := RunChunks(context.Background(), newChunkPipeline(t, "chunks-resume-2", second), "run-1", chunks,
		ChunkOptions[int, int]{Store: store, ContinueOnError: true})
	require.NoError(t, err)

	assert.Equal(t, []int{6}, second.ranChunks())
	got := statuses(results)
	assert.Equal(t, []int{6}, got[checkpoint.StatusDone])
	assert.Len(t, got[checkpoint.StatusSkipped], 11)

	records, err := store.Load(context.Background(), "run-1")
	require.NoError(t, err)
	assert.Equal(t, checkpoint.StatusDone, records["6"].Status)
	assert.Equal(t, 2, records["6"].Attempts)
	assert.Empty(t, records["6"].Error)
	assert.Equal(t, 1, records["5"].Attempts, "a skipped chunk is not attempted again")

	// A different run ID shares nothing with run-1.
	third := &chunkRecorder{}
	_, err = RunChunks(context.Background(), newChunkPipeline(t, "chunks-resume-3", third), "run-2", chunks,
		ChunkOptions[int, int]{Store: store})
	require.NoError(t, err)
	assert.Equal(t, indexes(0, 11), third.ranChunks())
}

func TestRunChunks_resumeRunsFailedAndMissingChunks(t *testing.T) {
	store := checkpoint.NewMemory()
	chunks := twelveChunks(t)

	// Default mode stops at chunk 6, leaving 7-11 missing.
	first := &chunkRecorder{failOn: map[int]bool{6: true}}
	_, err := RunChunks(context.Background(), newChunkPipeline(t, "chunks-missing-1", first), "run-1", chunks,
		ChunkOptions[int, int]{Store: store})
	require.Error(t, err)

	second := &chunkRecorder{}
	_, err = RunChunks(context.Background(), newChunkPipeline(t, "chunks-missing-2", second), "run-1", chunks,
		ChunkOptions[int, int]{Store: store})
	require.NoError(t, err)

	assert.Equal(t, indexes(6, 11), second.ranChunks())
}

// A crash after a chunk's pipeline ran but before it was checkpointed — here
// a panic in Commit — loses only that chunk.
func TestRunChunks_crashMidRunThenResume(t *testing.T) {
	store := checkpoint.NewMemory()
	chunks := twelveChunks(t)

	var committed []string

	commit := func(crashAt string) func(context.Context, string, []task.Task[int, int]) error {
		return func(_ context.Context, key string, _ []task.Task[int, int]) error {
			if key == crashAt {
				panic("process killed")
			}

			committed = append(committed, key)

			return nil
		}
	}

	first := &chunkRecorder{}
	require.Panics(t, func() {
		_, _ = RunChunks(context.Background(), newChunkPipeline(t, "chunks-crash-1", first), "run-1", chunks,
			ChunkOptions[int, int]{Store: store, Commit: commit("4")})
	})
	assert.Equal(t, indexes(0, 4), first.ranChunks())

	records, err := store.Load(context.Background(), "run-1")
	require.NoError(t, err)
	assert.Len(t, records, 4, "chunks 0-3 are checkpointed; the crashed chunk is not")

	second := &chunkRecorder{}
	results, err := RunChunks(context.Background(), newChunkPipeline(t, "chunks-crash-2", second), "run-1", chunks,
		ChunkOptions[int, int]{Store: store, Commit: commit("")})
	require.NoError(t, err)

	assert.Equal(t, indexes(4, 11), second.ranChunks())
	assert.Len(t, statuses(results)[checkpoint.StatusSkipped], 4)
	assert.Equal(t, []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11"}, committed)
}

func TestRunChunks_commitFailureFailsChunk(t *testing.T) {
	store := checkpoint.NewMemory()
	errCommit := errors.New("disk full")

	results, err := RunChunks(context.Background(), newChunkPipeline(t, "chunks-commit-fails", &chunkRecorder{}), "run-1",
		twelveChunks(t)[:3], ChunkOptions[int, int]{
			Store:           store,
			ContinueOnError: true,
			Commit: func(_ context.Context, key string, _ []task.Task[int, int]) error {
				if key == "1" {
					return errCommit
				}

				return nil
			},
		})
	require.ErrorIs(t, err, errCommit)

	assert.Equal(t, map[checkpoint.Status][]int{
		checkpoint.StatusDone:   {0, 2},
		checkpoint.StatusFailed: {1},
	}, statuses(results))
	assert.Nil(t, results[1].Tasks)

	records, err := store.Load(context.Background(), "run-1")
	require.NoError(t, err)
	assert.Equal(t, checkpoint.StatusFailed, records["1"].Status)
	assert.Equal(t, errCommit.Error(), records["1"].Error)
}

func TestRunChunks_customKey(t *testing.T) {
	store := checkpoint.NewMemory()
	chunks := twelveChunks(t)[:3]
	key := func(_ int, chunk []int) string { return fmt.Sprintf("line-%d", chunk[0]*500) }

	results, err := RunChunks(context.Background(), newChunkPipeline(t, "chunks-custom-key", &chunkRecorder{}), "run-1", chunks,
		ChunkOptions[int, int]{Store: store, Key: key})
	require.NoError(t, err)
	assert.Equal(t, "line-1000", results[2].Key)

	records, err := store.Load(context.Background(), "run-1")
	require.NoError(t, err)
	assert.Contains(t, records, "line-0")
	assert.Contains(t, records, "line-500")
	assert.Contains(t, records, "line-1000")

	// The same keys in a different order still resume by key, not position.
	again := &chunkRecorder{}
	reordered := [][]int{chunks[2], chunks[0], chunks[1]}
	_, err = RunChunks(context.Background(), newChunkPipeline(t, "chunks-custom-key-2", again), "run-1", reordered,
		ChunkOptions[int, int]{Store: store, Key: key})
	require.NoError(t, err)
	assert.Empty(t, again.ranChunks())
}

func TestRunChunks_invalidInput(t *testing.T) {
	p := newChunkPipeline(t, "chunks-invalid", &chunkRecorder{})
	chunks := twelveChunks(t)[:2]

	var typedNil *Pipeline[int, int]

	tests := []struct {
		name string
		p    IPipeline[int, int]
		run  string
		key  func(int, []int) string
		want string
	}{
		{name: "nil pipeline", p: nil, run: "run", want: "pipeline"},
		{name: "typed nil pipeline", p: typedNil, run: "run", want: "pipeline"},
		{name: "empty run ID", p: p, run: "", want: "runID"},
		{name: "empty key", p: p, run: "run", key: func(int, []int) string { return "" }, want: "chunk key"},
		{name: "duplicate key", p: p, run: "run", key: func(int, []int) string { return "same" }, want: "duplicate"},
		{name: "invalid UTF-8 run ID", p: p, run: "run-\xff", want: "runID"},
		{name: "invalid UTF-8 key", p: p, run: "run", key: func(i int, _ []int) string { return []string{"\uFFFD", "\xff"}[i] }, want: "chunk key"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := checkpoint.NewMemory()

			results, err := RunChunks(context.Background(), tt.p, tt.run, chunks, ChunkOptions[int, int]{Store: store, Key: tt.key})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
			assert.Nil(t, results)

			records, err := store.Load(context.Background(), tt.run)
			require.NoError(t, err)
			assert.Empty(t, records, "nothing may run or be recorded")
		})
	}
}

func TestRunChunks_noChunks(t *testing.T) {
	results, err := RunChunks(context.Background(), newChunkPipeline(t, "chunks-none", &chunkRecorder{}), "run-1",
		nil, ChunkOptions[int, int]{})
	require.NoError(t, err)
	assert.Empty(t, results)
}

// The zero value of ChunkOptions — no Store — still checkpoints within the
// call, so ContinueOnError works without persistence. A typed-nil Store is
// treated the same way.
func TestRunChunks_defaultStore(t *testing.T) {
	var typedNil *checkpoint.Memory

	for name, store := range map[string]checkpoint.Store{"nil": nil, "typed nil": typedNil} {
		t.Run(name, func(t *testing.T) {
			r := &chunkRecorder{failOn: map[int]bool{1: true}}

			results, err := RunChunks(context.Background(), newChunkPipeline(t, "chunks-default-store-"+name, r), "run-1",
				twelveChunks(t)[:3], ChunkOptions[int, int]{Store: store, ContinueOnError: true})
			require.ErrorIs(t, err, errChunk)
			assert.Equal(t, []int{0, 1, 2}, r.ranChunks())
			assert.Len(t, results, 3)
		})
	}
}

// fakeStore fails on demand and records the contexts Save was given.
type fakeStore struct {
	*checkpoint.Memory

	loadErr   error
	saveErr   error
	saveErrAt string
	saveCtxs  []error
}

func (f *fakeStore) Load(ctx context.Context, runID string) (map[string]checkpoint.Record, error) {
	if f.loadErr != nil {
		return nil, f.loadErr
	}

	return f.Memory.Load(ctx, runID)
}

func (f *fakeStore) Save(ctx context.Context, runID string, record checkpoint.Record) error {
	f.saveCtxs = append(f.saveCtxs, ctx.Err())

	if f.saveErr != nil && record.Key == f.saveErrAt {
		return f.saveErr
	}

	return f.Memory.Save(ctx, runID, record)
}

func TestRunChunks_loadErrorRunsNothing(t *testing.T) {
	errLoad := errors.New("db down")
	r := &chunkRecorder{}

	results, err := RunChunks(context.Background(), newChunkPipeline(t, "chunks-load-err", r), "run-1",
		twelveChunks(t), ChunkOptions[int, int]{Store: &fakeStore{Memory: checkpoint.NewMemory(), loadErr: errLoad}})
	require.ErrorIs(t, err, errLoad)
	assert.Contains(t, err.Error(), "load checkpoints")
	assert.Nil(t, results)
	assert.Empty(t, r.ranChunks())
}

// A chunk that cannot be checkpointed stops the run even with
// ContinueOnError: going on would do work no rerun can account for.
func TestRunChunks_saveErrorStops(t *testing.T) {
	errSave := errors.New("db down")
	store := &fakeStore{Memory: checkpoint.NewMemory(), saveErr: errSave, saveErrAt: "2"}
	r := &chunkRecorder{}

	results, err := RunChunks(context.Background(), newChunkPipeline(t, "chunks-save-err", r), "run-1",
		twelveChunks(t), ChunkOptions[int, int]{Store: store, ContinueOnError: true})
	require.ErrorIs(t, err, errSave)
	assert.Contains(t, err.Error(), "save checkpoint")

	assert.Equal(t, []int{0, 1, 2}, r.ranChunks())
	require.Len(t, results, 3)
	assert.Equal(t, checkpoint.StatusDone, results[2].Status, "the chunk itself succeeded")
	require.ErrorIs(t, results[2].Err, errSave)

	records, err := store.Load(context.Background(), "run-1")
	require.NoError(t, err)
	assert.Len(t, records, 2)
}

func TestRunChunks_saveErrorOnFailedChunkKeepsBoth(t *testing.T) {
	errSave := errors.New("db down")
	store := &fakeStore{Memory: checkpoint.NewMemory(), saveErr: errSave, saveErrAt: "1"}

	results, err := RunChunks(context.Background(), newChunkPipeline(t, "chunks-save-err-failed", &chunkRecorder{failOn: map[int]bool{1: true}}),
		"run-1", twelveChunks(t), ChunkOptions[int, int]{Store: store, ContinueOnError: true})
	require.ErrorIs(t, err, errSave)
	require.ErrorIs(t, err, errChunk)
	require.Len(t, results, 2)
	assert.Equal(t, checkpoint.StatusFailed, results[1].Status)
}

func TestRunChunks_canceledBeforeStart(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	r := &chunkRecorder{}

	results, err := RunChunks(ctx, newChunkPipeline(t, "chunks-canceled-start", r), "run-1", twelveChunks(t), ChunkOptions[int, int]{})
	require.ErrorIs(t, err, context.Canceled)
	assert.Empty(t, results)
	assert.Empty(t, r.ranChunks())
}

// Canceling mid-run still checkpoints the chunk that finished, then stops
// before the next one, even with ContinueOnError.
func TestRunChunks_canceledMidRun(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store := &fakeStore{Memory: checkpoint.NewMemory()}
	r := &chunkRecorder{}

	results, err := RunChunks(ctx, newChunkPipeline(t, "chunks-canceled-mid", r), "run-1", twelveChunks(t),
		ChunkOptions[int, int]{
			Store:           store,
			ContinueOnError: true,
			Commit: func(_ context.Context, key string, _ []task.Task[int, int]) error {
				if key == "2" {
					cancel()
				}

				return nil
			},
		})
	require.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, []int{0, 1, 2}, r.ranChunks())
	assert.Len(t, results, 3)

	records, err := store.Load(context.Background(), "run-1")
	require.NoError(t, err)
	assert.Equal(t, checkpoint.StatusDone, records["2"].Status)

	for i, saveCtxErr := range store.saveCtxs {
		assert.NoError(t, saveCtxErr, "save %d must not see the cancellation", i)
	}
}
