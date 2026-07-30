package processor

import (
	"context"
	"fmt"
	"log"
)

// Demonstrates a processor that transforms every item in a slice.
func ExampleNew() {
	double, err := New(
		"example-double",
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

	out, err := double.Run(context.Background(), []int{1, 2, 3})
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Println(out)

	// Output:
	// [2 4 6]
}

// Demonstrates setting a callback after constructing a processor.
func ExampleProcessor_SetOnFinished() {
	double, err := New(
		"example-double-callback",
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

	double.SetOnFinished(func(_ context.Context, p IProcessor[int], in, out []int) {
		fmt.Printf("%s: %v -> %v\n", p.GetName(), in, out)
	})

	if _, err := double.Run(context.Background(), []int{2, 4}); err != nil {
		log.Fatalln(err)
	}

	// Output:
	// example-double-callback: [2 4] -> [4 8]
}
