package pipeline

import (
	"context"
	"fmt"
	"log"
	"slices"
	"sort"

	"github.com/thalesfsp/etler/v4/converters/passthru"
	"github.com/thalesfsp/etler/v4/processor"
	"github.com/thalesfsp/etler/v4/stage"
	"github.com/thalesfsp/etler/v4/task"
)

// Demonstrates a two-stage sequential pipeline.
func ExampleNew() {
	increment, err := processor.New(
		"example-pipeline-increment",
		"increments every number",
		func(_ context.Context, in []int) ([]int, error) {
			out := make([]int, len(in))
			for i, value := range in {
				out[i] = value + 1
			}

			return out, nil
		},
	)
	if err != nil {
		log.Fatalln(err)
	}

	double, err := processor.New(
		"example-pipeline-double",
		"doubles every number",
		func(_ context.Context, in []int) ([]int, error) {
			out := make([]int, len(in))
			for i, value := range in {
				out[i] = value * 2
			}

			return out, nil
		},
	)
	if err != nil {
		log.Fatalln(err)
	}

	first, err := stage.New(
		"example-pipeline-first",
		"increments numbers",
		passthru.Must[int](),
		increment,
	)
	if err != nil {
		log.Fatalln(err)
	}

	second, err := stage.New(
		"example-pipeline-second",
		"doubles numbers",
		passthru.Must[int](),
		double,
	)
	if err != nil {
		log.Fatalln(err)
	}

	flow, err := New(
		"example-sequential-pipeline",
		"increments, then doubles",
		false,
		first,
		second,
	)
	if err != nil {
		log.Fatalln(err)
	}

	tasks, err := flow.Run(context.Background(), []int{1, 2, 3})
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Println(tasks[len(tasks)-1].ConvertedData)

	// Output:
	// [4 6 8]
}

// Demonstrates stages that independently process the same input concurrently.
func ExampleNew_concurrentStages() {
	addTen, err := processor.New(
		"example-concurrent-add-ten",
		"adds ten to every number",
		func(_ context.Context, in []int) ([]int, error) {
			out := make([]int, len(in))
			for i, value := range in {
				out[i] = value + 10
			}

			return out, nil
		},
	)
	if err != nil {
		log.Fatalln(err)
	}

	double, err := processor.New(
		"example-concurrent-double",
		"doubles every number",
		func(_ context.Context, in []int) ([]int, error) {
			out := make([]int, len(in))
			for i, value := range in {
				out[i] = value * 2
			}

			return out, nil
		},
	)
	if err != nil {
		log.Fatalln(err)
	}

	addStage, err := stage.New(
		"example-concurrent-add-stage",
		"adds ten",
		passthru.Must[int](),
		addTen,
	)
	if err != nil {
		log.Fatalln(err)
	}

	doubleStage, err := stage.New(
		"example-concurrent-double-stage",
		"doubles",
		passthru.Must[int](),
		double,
	)
	if err != nil {
		log.Fatalln(err)
	}

	flow, err := New(
		"example-concurrent-pipeline",
		"runs both stages independently",
		true,
		addStage,
		doubleStage,
	)
	if err != nil {
		log.Fatalln(err)
	}

	tasks, err := flow.Run(context.Background(), []int{1, 2, 3})
	if err != nil {
		log.Fatalln(err)
	}

	results := make([][]int, len(tasks))
	for i, result := range tasks {
		results[i] = slices.Clone(result.ConvertedData)
		slices.Sort(results[i])
	}
	sort.Slice(results, func(i, j int) bool {
		return slices.Compare(results[i], results[j]) < 0
	})

	fmt.Println(results)

	// Output:
	// [[2 4 6] [11 12 13]]
}

// Demonstrates observing every stage result after a pipeline finishes.
func ExamplePipeline_SetOnFinished() {
	increment, err := processor.New(
		"example-callback-increment",
		"increments every number",
		func(_ context.Context, in []int) ([]int, error) {
			out := make([]int, len(in))
			for i, value := range in {
				out[i] = value + 1
			}

			return out, nil
		},
	)
	if err != nil {
		log.Fatalln(err)
	}

	triple, err := processor.New(
		"example-callback-triple",
		"triples every number",
		func(_ context.Context, in []int) ([]int, error) {
			out := make([]int, len(in))
			for i, value := range in {
				out[i] = value * 3
			}

			return out, nil
		},
	)
	if err != nil {
		log.Fatalln(err)
	}

	first, err := stage.New(
		"example-callback-first",
		"increments numbers",
		passthru.Must[int](),
		increment,
	)
	if err != nil {
		log.Fatalln(err)
	}

	second, err := stage.New(
		"example-callback-second",
		"triples numbers",
		passthru.Must[int](),
		triple,
	)
	if err != nil {
		log.Fatalln(err)
	}

	flow, err := New(
		"example-callback-pipeline",
		"observes stage results",
		false,
		first,
		second,
	)
	if err != nil {
		log.Fatalln(err)
	}

	flow.SetOnFinished(func(
		_ context.Context,
		_ IPipeline[int, int],
		_ task.Task[int, int],
		tasks []task.Task[int, int],
	) {
		for i, result := range tasks {
			fmt.Printf("stage %d: %v\n", i+1, result.ConvertedData)
		}
	})

	if _, err := flow.Run(context.Background(), []int{1, 2}); err != nil {
		log.Fatalln(err)
	}

	// Output:
	// stage 1: [2 3]
	// stage 2: [6 9]
}
