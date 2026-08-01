package stage

import (
	"context"
	"fmt"
	"log"
	"slices"
	"strconv"

	"github.com/thalesfsp/etler/v4/converter"
	"github.com/thalesfsp/etler/v4/processor"
	"github.com/thalesfsp/etler/v4/task"
)

// Demonstrates a stage that processes values, then changes their type.
func ExampleNew() {
	square, err := processor.New(
		"example-stage-square",
		"squares every number",
		func(_ context.Context, in []int) ([]int, error) {
			out := make([]int, len(in))
			for i, value := range in {
				out[i] = value * value
			}

			return out, nil
		},
	)
	if err != nil {
		log.Fatalln(err)
	}

	format, err := converter.New(
		"example-stage-format",
		"formats a number",
		func(_ context.Context, in int) (string, error) {
			return "value=" + strconv.Itoa(in), nil
		},
	)
	if err != nil {
		log.Fatalln(err)
	}

	numbers, err := New("example-stage", "squares and formats numbers", format, square)
	if err != nil {
		log.Fatalln(err)
	}

	out, err := numbers.Run(context.Background(), task.Task[int, string]{
		ProcessingData: []int{2, 3},
	})
	if err != nil {
		log.Fatalln(err)
	}

	slices.Sort(out.ConvertedData)
	fmt.Println(out.ConvertedData)

	// Output:
	// [value=4 value=9]
}
