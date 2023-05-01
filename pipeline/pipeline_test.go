package pipeline

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/WreckingBallStudioLabs/etler/processor"
)

type TestUser struct {
	Age       int       `json:"age,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	Name      string    `json:"name"`
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
	p := New(
		"User Enhancer",
		"Enhances user data",
		[]Stage[TestUser, TestUser]{
			{
				Concurrent: false,
				Processors: []processor.IProcessor[TestUser, TestUser]{double, square},
			},
		},
	)

	//////
	// Run the pipeline.
	//////

	records := []TestUser{
		{
			Name: "jacek",
			Age:  26,
		},
		{
			Name: "john",
			Age:  34,
		},
	}

	// Context with timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	processedRecords, err := p.Run(ctx, records)
	if err != nil {
		t.Fatal(err)
	}

	//////
	// Validates changes in `processedRecords`.
	//////

	if len(processedRecords) != 2 {
		t.Fatalf("Unexpected number of out: expected=2, got=%d", len(processedRecords))
	}

	if processedRecords[0].Name != "jacek" || processedRecords[0].Age != 676 {
		t.Fatalf("Unexpected record data: expected=(jacek,676), got=(%s,%d)", processedRecords[0].Name, processedRecords[0].Age)
	}

	if processedRecords[1].Name != "john" || processedRecords[1].Age != 1156 {
		t.Fatalf("Unexpected record data: expected=(john,1156), got=(%s,%d,%v)", processedRecords[1].Name, processedRecords[1].Age, processedRecords[1].CreatedAt)
	}
}
