package pipeline

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/thalesfsp/etler/v4/checkpoint"
	"github.com/thalesfsp/etler/v4/converters/passthru"
	"github.com/thalesfsp/etler/v4/processor"
	"github.com/thalesfsp/etler/v4/stage"
	"github.com/thalesfsp/etler/v4/task"
)

// Demonstrates a chunked run that survives a failed chunk and resumes it.
func ExampleRunChunks() {
	failing := true

	call, err := processor.New(
		"example-chunks-call",
		"calls an upstream service per chunk",
		func(_ context.Context, in []int) ([]int, error) {
			if failing && in[0] == 3 {
				return nil, errors.New("upstream timed out")
			}

			return in, nil
		},
	)
	if err != nil {
		log.Fatalln(err)
	}

	only, err := stage.New("example-chunks-stage", "calls upstream", passthru.Must[int](), call)
	if err != nil {
		log.Fatalln(err)
	}

	flow, err := New("example-chunks-pipeline", "chunked upstream calls", false, only)
	if err != nil {
		log.Fatalln(err)
	}

	chunks, err := Chunk([]int{1, 2, 3, 4, 5, 6}, 2)
	if err != nil {
		log.Fatalln(err)
	}

	// Use checkpoint.NewFile, or a database-backed Store, to resume across
	// processes.
	opts := ChunkOptions[int, int]{
		Store:           checkpoint.NewMemory(),
		ContinueOnError: true,
		Commit: func(_ context.Context, key string, tasks []task.Task[int, int]) error {
			fmt.Printf("commit chunk %s: %v\n", key, tasks[0].ConvertedData)

			return nil
		},
	}

	if _, err := RunChunks(context.Background(), flow, "nightly", chunks, opts); err != nil {
		fmt.Println("first run failed a chunk")
	}

	failing = false

	results, err := RunChunks(context.Background(), flow, "nightly", chunks, opts)
	if err != nil {
		log.Fatalln(err)
	}

	for _, result := range results {
		fmt.Printf("chunk %s: %s\n", result.Key, result.Status)
	}

	// Output:
	// commit chunk 0: [1 2]
	// commit chunk 2: [5 6]
	// first run failed a chunk
	// commit chunk 1: [3 4]
	// chunk 0: skipped
	// chunk 1: done
	// chunk 2: skipped
}
