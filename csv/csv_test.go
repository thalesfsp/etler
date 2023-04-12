package csv

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/thalesfsp/etler/pipeline"
	"github.com/thalesfsp/etler/processor"
)

type TestUser struct {
	Name      string `csv:"name"`
	Age       int    `csv:"age,omitempty"`
	CreatedAt time.Time
}

var buf = new(bytes.Buffer)

func TestCSVFileAdapter_Read(t *testing.T) {
	//////
	// Processors.
	//////

	double := processor.New(
		"double",
		"doubles the input",
		func(ctx context.Context, in []TestUser) ([]TestUser, error) {
			out := make([]TestUser, len(in))

			for i, v := range in {
				out[i] = v
				out[i].Age = v.Age * 2
			}

			return out, nil
		},
	)

	square := processor.New(
		"square",
		"squares the input",
		func(ctx context.Context, in []TestUser) ([]TestUser, error) {
			out := make([]TestUser, len(in))

			for i, v := range in {
				out[i] = v
				out[i].Age = v.Age * v.Age
			}

			return out, nil
		},
	)

	//////
	// Setup pipeline.
	//////

	// Create a new pipeline.
	p := pipeline.New(
		"User Enhancer",
		"Enhances user data",
		[]pipeline.Stage[TestUser]{
			{
				Concurrent: false,
				Processors: []processor.IProcessor[TestUser]{double, square},
			},
		},
	)

	//////
	// Setup adapter.
	//////

	ctx := context.Background()

	csvAdapter, err := New[TestUser]([]byte(`
name,age,CreatedAt
jacek,26,2012-04-01T15:00:00Z
john,,0001-01-01T00:00:00Z`,
	))
	if err != nil {
		t.Fatal(err)
	}

	//////
	// Load data.
	//////

	records, err := csvAdapter.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}

	//////
	// Validate data.
	//////

	if len(records) != 2 {
		t.Fatalf("Unexpected number of records: expected=2, got=%d", len(records))
	}

	if records[0].Name != "jacek" || records[0].Age != 26 || records[0].CreatedAt == (time.Time{}) {
		t.Fatalf("Unexpected record data: expected=(jacek), got=(%s,%d)", records[0].Name, records[0].Age)
	}

	if records[1].Name != "john" || records[1].Age != 0 || records[1].CreatedAt != (time.Time{}) {
		t.Fatalf("Unexpected record data: expected=(john), got=(%s,%d,%v)", records[1].Name, records[1].Age, records[1].CreatedAt)
	}

	//////
	// Run the pipeline.
	//////

	processedRecords, err := p.Run(ctx, records)
	if err != nil {
		fmt.Println("run failed", err)
		return
	}

	//////
	// Validates changes in `processedRecords`.
	//////

	if len(processedRecords) != 2 {
		t.Fatalf("Unexpected number of out: expected=2, got=%d", len(processedRecords))
	}

	if processedRecords[0].Name != "jacek" || processedRecords[0].Age != 2704 || processedRecords[0].CreatedAt == (time.Time{}) {
		t.Fatalf("Unexpected record data: expected=(jacek), got=(%s,%d)", processedRecords[0].Name, processedRecords[0].Age)
	}

	if processedRecords[1].Name != "john" || processedRecords[1].Age != 0 || processedRecords[1].CreatedAt != (time.Time{}) {
		t.Fatalf("Unexpected record data: expected=(john), got=(%s,%d,%v)", processedRecords[1].Name, processedRecords[1].Age, processedRecords[1].CreatedAt)
	}

	//////
	// Should not change the original data.
	//////

	originalRecords, err := csvAdapter.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(originalRecords) != 2 {
		t.Fatalf("Unexpected number of originalRecords: expected=2, got=%d", len(originalRecords))
	}

	if originalRecords[0].Name != "jacek" || originalRecords[0].Age != 26 || originalRecords[0].CreatedAt == (time.Time{}) {
		t.Fatalf("Unexpected record data: expected=(jacek), got=(%s,%d)", originalRecords[0].Name, originalRecords[0].Age)
	}

	if originalRecords[1].Name != "john" || originalRecords[1].Age != 0 || originalRecords[1].CreatedAt != (time.Time{}) {
		t.Fatalf("Unexpected record data: expected=(john), got=(%s,%d,%v)", originalRecords[1].Name, originalRecords[1].Age, originalRecords[1].CreatedAt)
	}

	//////
	// Write data.
	//////

	if err := csvAdapter.Upsert(ctx, processedRecords); err != nil {
		fmt.Println("write failed", err)
		return
	}

	//////
	// Should have changed the data.
	//////

	updatedRecords, err := csvAdapter.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(updatedRecords) != 2 {
		t.Fatalf("Unexpected number of updatedRecords: expected=2, got=%d", len(updatedRecords))
	}

	if updatedRecords[0].Name != "jacek" || updatedRecords[0].Age != 2704 || updatedRecords[0].CreatedAt == (time.Time{}) {
		t.Fatalf("Unexpected record data: expected=(jacek), got=(%s,%d)", updatedRecords[0].Name, updatedRecords[0].Age)
	}

	if updatedRecords[1].Name != "john" || updatedRecords[1].Age != 0 || updatedRecords[1].CreatedAt != (time.Time{}) {
		t.Fatalf("Unexpected record data: expected=(john), got=(%s,%d,%v)", updatedRecords[1].Name, updatedRecords[1].Age, updatedRecords[1].CreatedAt)
	}
}
