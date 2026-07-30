package csv

import (
	"context"
	"fmt"
	"log"
	"strings"
)

// Demonstrates decoding CSV directly into plain typed struct fields.
func ExampleNew() {
	type person struct {
		Name   string
		Age    int
		Active bool
	}

	people, err := New[person]()
	if err != nil {
		log.Fatalln(err)
	}

	out, err := people.Run(
		context.Background(),
		strings.NewReader("Name,Age,Active\nAda,37,true\nGrace,40,false\n"),
	)
	if err != nil {
		log.Fatalln(err)
	}

	for _, person := range out {
		fmt.Printf("%s: age=%d active=%t\n", person.Name, person.Age, person.Active)
	}

	// Output:
	// Ada: age=37 active=true
	// Grace: age=40 active=false
}
