package storage

import (
	"context"
	"fmt"
)

type exampleItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Demonstrates storing an item under the ID derived from the item.
func ExampleNew() {
	store := &stubStorage{}
	converter, err := New[exampleItem](store, "items")
	if err != nil {
		panic(err)
	}

	item := exampleItem{ID: "item-1", Name: "notebook"}
	id, err := converter.Run(context.Background(), item)
	if err != nil {
		panic(err)
	}

	fmt.Println("value written:", store.createdValues()[0] == item)
	fmt.Println("key from item:", id == item.ID && store.createdIDs()[0] == item.ID)

	// Output:
	// value written: true
	// key from item: true
}

// Demonstrates that equal items share a deterministic ID while different
// items do not.
func ExampleStorage_deterministicID() {
	store := &stubStorage{}
	converter, err := New[exampleItem](store, "items")
	if err != nil {
		panic(err)
	}

	firstID, err := converter.Run(
		context.Background(),
		exampleItem{Name: "notebook"},
	)
	if err != nil {
		panic(err)
	}
	equalID, err := converter.Run(
		context.Background(),
		exampleItem{Name: "notebook"},
	)
	if err != nil {
		panic(err)
	}
	differentID, err := converter.Run(
		context.Background(),
		exampleItem{Name: "pencil"},
	)
	if err != nil {
		panic(err)
	}

	fmt.Println("equal items:", firstID == equalID)
	fmt.Println("different items:", firstID != differentID)

	// Output:
	// equal items: true
	// different items: true
}
