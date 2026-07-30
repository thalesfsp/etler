package loader

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"
)

// Demonstrates a loader that reads a source into typed items.
func ExampleNew() {
	numbers, err := New(
		"example-number-loader",
		"reads one integer per line",
		func(_ context.Context, source io.Reader) ([]int, error) {
			var out []int
			scanner := bufio.NewScanner(source)
			for scanner.Scan() {
				value, err := strconv.Atoi(scanner.Text())
				if err != nil {
					return nil, err
				}

				out = append(out, value)
			}

			return out, scanner.Err()
		},
	)
	if err != nil {
		log.Fatalln(err)
	}

	out, err := numbers.Run(context.Background(), strings.NewReader("10\n20\n30\n"))
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Println(out)

	// Output:
	// [10 20 30]
}
