package etler_test

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thalesfsp/etler/v4/checkpoint"
	"github.com/thalesfsp/etler/v4/converters/passthru"
	"github.com/thalesfsp/etler/v4/pipeline"
	"github.com/thalesfsp/etler/v4/processor"
	"github.com/thalesfsp/etler/v4/stage"
	"github.com/thalesfsp/etler/v4/task"
)

// Environment variables that switch TestE2E_chunks_processCrashThenResume
// into its child-process role.
const (
	crashHelperEnv = "ETLER_E2E_CHUNK_CRASH_DIR"
	crashExitCode  = 3
)

// e2eLines builds n input lines, "line-0" ... "line-<n-1>".
func e2eLines(n int) []string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = "line-" + strconv.Itoa(i)
	}

	return lines
}

// chunkNumber returns the 0-based chunk a line belongs to for chunks of size.
func chunkNumber(line string, size int) int {
	n, _ := strconv.Atoi(strings.TrimPrefix(line, "line-"))

	return n / size
}

// newE2EChunkPipeline builds a CSV-free pipeline that uppercases lines and
// calls onChunk with the chunk number before doing so.
func newE2EChunkPipeline(t *testing.T, name string, size int, onChunk func(chunk int) error) pipeline.IPipeline[string, string] {
	t.Helper()

	upper, err := processor.New(name+"-upper", "uppercases lines", func(_ context.Context, in []string) ([]string, error) {
		if err := onChunk(chunkNumber(in[0], size)); err != nil {
			return nil, err
		}

		out := make([]string, len(in))
		for i, v := range in {
			out[i] = strings.ToUpper(v)
		}

		return out, nil
	})
	require.NoError(t, err)

	stg, err := stage.New(name+"-stage", "uppercases lines", passthru.Must[string](), upper)
	require.NoError(t, err)

	p, err := pipeline.New(name, "uppercases lines", false, stg)
	require.NoError(t, err)

	return p
}

// appendCommit commits a chunk by appending its key and items to sink, the
// stand-in for BrainOS filing cards.
func appendCommit(sink string) func(context.Context, string, []task.Task[string, string]) error {
	return func(_ context.Context, key string, tasks []task.Task[string, string]) error {
		f, err := os.OpenFile(sink, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(f, "%s %d %s\n", key, len(tasks[0].ProcessingData), tasks[0].ProcessingData[0])

		return errors.Join(err, f.Close())
	}
}

// readSink returns the committed chunk keys in commit order.
func readSink(t *testing.T, sink string) []string {
	t.Helper()

	f, err := os.Open(sink)
	require.NoError(t, err)

	defer func() { require.NoError(t, f.Close()) }()

	var keys []string

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		keys = append(keys, strings.Fields(scanner.Text())[0])
	}

	require.NoError(t, scanner.Err())

	return keys
}

func keyRange(from, to int) []string {
	out := []string{}
	for i := from; i <= to; i++ {
		out = append(out, strconv.Itoa(i))
	}

	return out
}

// The BrainOS nightly case end to end: 6,000 lines in 500-line chunks, the
// upstream call times out on chunk 7 of 12. The other 11 commit and are
// checkpointed in a file; a rerun in a new process (a new store and pipeline
// on the same file) asks only chunk 7.
func TestE2E_chunks_timeoutOnChunk7ThenResume(t *testing.T) {
	const size = 500

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	dir := t.TempDir()
	path := filepath.Join(dir, "checkpoints.json")
	sink := filepath.Join(dir, "committed.txt")

	chunks, err := pipeline.Chunk(e2eLines(12*size), size)
	require.NoError(t, err)
	require.Len(t, chunks, 12)

	store, err := checkpoint.NewFile(path)
	require.NoError(t, err)

	var firstRan []int

	first := newE2EChunkPipeline(t, "e2e-chunks-timeout-1", size, func(chunk int) error {
		firstRan = append(firstRan, chunk)

		if chunk == 6 {
			return context.DeadlineExceeded
		}

		return nil
	})

	results, err := pipeline.RunChunks(ctx, first, "nightly-2026-11-16", chunks, pipeline.ChunkOptions[string, string]{
		Store:           store,
		ContinueOnError: true,
		Commit:          appendCommit(sink),
	})
	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Len(t, firstRan, 12, "chunks 8-12 must run after chunk 7 fails")
	assert.Equal(t, checkpoint.StatusFailed, results[6].Status)
	assert.Equal(t, append(keyRange(0, 5), keyRange(7, 11)...), readSink(t, sink))

	// The "next night": a fresh store instance on the same file.
	reopened, err := checkpoint.NewFile(path)
	require.NoError(t, err)

	var secondRan []int

	second := newE2EChunkPipeline(t, "e2e-chunks-timeout-2", size, func(chunk int) error {
		secondRan = append(secondRan, chunk)

		return nil
	})

	results, err = pipeline.RunChunks(ctx, second, "nightly-2026-11-16", chunks, pipeline.ChunkOptions[string, string]{
		Store:           reopened,
		ContinueOnError: true,
		Commit:          appendCommit(sink),
	})
	require.NoError(t, err)
	assert.Equal(t, []int{6}, secondRan, "the rerun asks only chunk 7")
	assert.Equal(t, checkpoint.StatusDone, results[6].Status)
	assert.Equal(t, []string{"LINE-3000"}, results[6].Tasks[0].ProcessingData[:1])

	committed := readSink(t, sink)
	assert.Len(t, committed, 12, "every chunk is committed exactly once")
	assert.Equal(t, "6", committed[11])

	records, err := reopened.Load(ctx, "nightly-2026-11-16")
	require.NoError(t, err)
	assert.Equal(t, 2, records["6"].Attempts)
}

// A real process crash: a child process dies (os.Exit) inside chunk 5 of 12.
// The parent then resumes from the child's checkpoint file and runs only
// chunks 5-12; no chunk is committed twice.
func TestE2E_chunks_processCrashThenResume(t *testing.T) {
	const (
		size    = 500
		crashAt = 4
	)

	if dir := os.Getenv(crashHelperEnv); dir != "" {
		runCrashingChild(t, dir, size, crashAt)

		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	dir := t.TempDir()

	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestE2E_chunks_processCrashThenResume$", "-test.count=1")
	cmd.Env = append(os.Environ(), crashHelperEnv+"="+dir)

	out, err := cmd.CombinedOutput()

	var exitErr *exec.ExitError
	require.True(t, errors.As(err, &exitErr), "child must die, got %v: %s", err, out)
	require.Equal(t, crashExitCode, exitErr.ExitCode(), "child output: %s", out)

	sink := filepath.Join(dir, "committed.txt")
	assert.Equal(t, keyRange(0, crashAt-1), readSink(t, sink), "chunks before the crash are committed")

	store, err := checkpoint.NewFile(filepath.Join(dir, "checkpoints.json"))
	require.NoError(t, err)

	records, err := store.Load(ctx, "nightly")
	require.NoError(t, err)
	assert.Len(t, records, crashAt, "the crashed chunk has no checkpoint")

	chunks, err := pipeline.Chunk(e2eLines(12*size), size)
	require.NoError(t, err)

	var ran []int

	resumed := newE2EChunkPipeline(t, "e2e-chunks-crash-resume", size, func(chunk int) error {
		ran = append(ran, chunk)

		return nil
	})

	results, err := pipeline.RunChunks(ctx, resumed, "nightly", chunks, pipeline.ChunkOptions[string, string]{
		Store:  store,
		Commit: appendCommit(sink),
	})
	require.NoError(t, err)

	want := []int{}
	for i := crashAt; i < 12; i++ {
		want = append(want, i)
	}

	assert.Equal(t, want, ran)
	assert.Equal(t, checkpoint.StatusSkipped, results[0].Status)
	assert.Equal(t, keyRange(0, 11), readSink(t, sink), "every chunk committed exactly once, in order")
}

// runCrashingChild is the child half of the crash test: it exits the process
// from inside chunk crashAt's pipeline run.
func runCrashingChild(t *testing.T, dir string, size, crashAt int) {
	store, err := checkpoint.NewFile(filepath.Join(dir, "checkpoints.json"))
	require.NoError(t, err)

	chunks, err := pipeline.Chunk(e2eLines(12*size), size)
	require.NoError(t, err)

	p := newE2EChunkPipeline(t, "e2e-chunks-crash-child", size, func(chunk int) error {
		if chunk == crashAt {
			os.Exit(crashExitCode)
		}

		return nil
	})

	_, _ = pipeline.RunChunks(context.Background(), p, "nightly", chunks, pipeline.ChunkOptions[string, string]{
		Store:  store,
		Commit: appendCommit(filepath.Join(dir, "committed.txt")),
	})

	t.Fatal("the child must exit inside the crashing chunk")
}
