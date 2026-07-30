package csv

import (
	"context"
	"fmt"
	"log"
)

// Demonstrates marshaling a slice of structs as CSV text.
func ExampleNew() {
	type product struct {
		Name  string `csv:"name"`
		Price int    `csv:"price"`
	}

	products, err := New[product]()
	if err != nil {
		log.Fatalln(err)
	}

	out, err := products.Run(context.Background(), []product{
		{Name: "tea", Price: 4},
		{Name: "coffee", Price: 6},
	})
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Print(out)

	// Output:
	// name,price
	// tea,4
	// coffee,6
}
