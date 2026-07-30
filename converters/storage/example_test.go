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

// Demonstrates that retrying the same item uses the same deterministic ID.
func ExampleStorage_deterministicID() {
	store := &stubStorage{}
	converter, err := New[exampleItem](store, "items")
	if err != nil {
		panic(err)
	}

	item := exampleItem{Name: "notebook"}
	firstID, err := converter.Run(context.Background(), item)
	if err != nil {
		panic(err)
	}
	secondID, err := converter.Run(context.Background(), item)
	if err != nil {
		panic(err)
	}

	fmt.Println("same ID:", firstID == secondID)

	// Output:
	// same ID: true
}
