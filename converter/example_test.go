package converter

import (
	"context"
	"fmt"
	"log"
)

// Demonstrates a converter that changes a value's type.
func ExampleNew() {
	type person struct {
		Name string
		Age  int
	}

	format, err := New(
		"example-person-string",
		"formats a person as text",
		func(_ context.Context, in person) (string, error) {
			return fmt.Sprintf("%s (%d)", in.Name, in.Age), nil
		},
	)
	if err != nil {
		log.Fatalln(err)
	}

	out, err := format.Run(context.Background(), person{Name: "Ada", Age: 37})
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Println(out)

	// Output:
	// Ada (37)
}
