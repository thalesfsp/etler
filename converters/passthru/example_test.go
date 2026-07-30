package passthru

import (
	"context"
	"fmt"
	"log"
)

// Demonstrates a converter that returns its input unchanged.
func ExampleMust() {
	out, err := Must[string]().Run(context.Background(), "unchanged")
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Println(out)

	// Output:
	// unchanged
}
