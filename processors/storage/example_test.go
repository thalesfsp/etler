package storage

import (
	"context"
	"fmt"
)

type exampleItem struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

// Demonstrates storing a slice while passing its items through unchanged.
func ExampleNew() {
	store := &stubStorage{}
	processor, err := New[exampleItem](store, 1, "item-")
	if err != nil {
		panic(err)
	}

	items := []exampleItem{{Name: "pencil"}, {Name: "notebook"}}
	out, err := processor.Run(context.Background(), items)
	if err != nil {
		panic(err)
	}

	stored := store.createdValues()

	fmt.Println("items:", out)
	fmt.Println("written:", store.created.Load())
	fmt.Println(
		"values stored:",
		len(stored) == len(items) &&
			stored[0] == items[0] &&
			stored[1] == items[1],
	)

	// Output:
	// items: [{ pencil} { notebook}]
	// written: 2
	// values stored: true
}

// Demonstrates selecting the item field used as the storage key.
func ExampleStorage_idFieldName() {
	store := &stubStorage{}
	processor, err := New[exampleItem](
		store,
		1,
		"item-",
		WithIDFieldName[exampleItem]("Key"),
	)
	if err != nil {
		panic(err)
	}

	_, err = processor.Run(
		context.Background(),
		[]exampleItem{{Key: "7", Name: "notebook"}},
	)
	if err != nil {
		panic(err)
	}

	fmt.Println("selected key:", store.createdIDs()[0] == "item-7")

	// Output:
	// selected key: true
}
