package elasticsearch

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/thalesfsp/etler/option"
	"github.com/thalesfsp/etler/pipeline"
	"github.com/thalesfsp/etler/processor"
)

type TestUser struct {
	Name      string    `json:"name"`
	Age       int       `json:"age,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

var buf = new(bytes.Buffer)

func TestNew(t *testing.T) {
	if os.Getenv("ENVIRONMENT") != "integration" {
		t.Skip("Skipping test. Set `ENVIRONMENT` to `integration` to run.")
	}

	//////
	// Processors.
	//////

	doubleAge := processor.New(
		"doubleAge",
		"doubles the age",
		func(ctx context.Context, in []TestUser) ([]TestUser, error) {
			out := make([]TestUser, len(in))

			for i, v := range in {
				out[i] = v
				out[i].Age = v.Age * 2
			}

			return out, nil
		},
	)

	squareAge := processor.New(
		"squareAge",
		"squares the age",
		func(ctx context.Context, in []TestUser) ([]TestUser, error) {
			out := make([]TestUser, len(in))

			for i, v := range in {
				out[i] = v
				out[i].Age = v.Age * v.Age
			}

			return out, nil
		},
	)

	upperCaserFirstChar := processor.New(
		"upperCaserFirstChar",
		"Upper case the first character of the name",
		func(ctx context.Context, in []TestUser) ([]TestUser, error) {
			out := make([]TestUser, len(in))

			for i, v := range in {
				out[i] = v
				out[i].Name = strings.ToUpper(string(v.Name[0])) + v.Name[1:]
			}

			return out, nil
		},
	)

	updatedAtPlus10Mins := processor.New(
		"updatedAtPlus10Mins",
		"Updates the updated_at field with the current time plus 10 minutes",
		func(ctx context.Context, in []TestUser) ([]TestUser, error) {
			out := make([]TestUser, len(in))

			for i, v := range in {
				out[i] = v
				out[i].UpdatedAt = time.Now().Add(10 * time.Minute)
			}

			return out, nil
		},
	)

	////
	// Setup pipeline.
	////

	// Create a new pipeline.
	p := pipeline.New(
		"User Enhancer",
		"Enhances user data",
		[]pipeline.Stage[TestUser]{
			{
				Concurrent: false,
				Processors: []processor.IProcessor[TestUser]{
					doubleAge,
					squareAge,
					upperCaserFirstChar,
					updatedAtPlus10Mins,
				},
			},
		},
	)

	//////
	// Setup adapter.
	//////

	ctx := context.Background()

	esAdapter, err := New[TestUser]("vendor-test", Config{
		APIKey:  os.Getenv("ELASTIC_ELASTICSEARCH_API_KEY"),
		CloudID: os.Getenv("ELASTIC_ELASTICSEARCH_CLOUD_ID"),
	})
	if err != nil {
		t.Fatal(err)
	}

	//////
	// Write data.
	//////

	if err := esAdapter.Upsert(
		ctx,
		[]TestUser{
			{Name: "jacek", Age: 26, CreatedAt: time.Now()},
			{Name: "john", Age: 18, CreatedAt: time.Now()},
		},
		option.WithFieldToUseForID("Age"),
	); err != nil {
		t.Fatal("write failed", err)
	}

	//////
	// Load data.
	//////

	records, err := esAdapter.Read(
		ctx,
		option.WithFields([]string{"name", "age", "created_at", "updated_at"}),
		option.WithFieldToUseForID("Name"),
		option.WithLimit(5),
		option.WithSort(map[string]string{"updated_at": "desc"}),
		option.WithQuery(`{ "query": { "bool": { "must": [ { "match": { "age": 26 } } ] } } }`))
	if err != nil {
		t.Fatal(err)
	}

	//////
	// Validate data.
	//////

	if len(records) != 1 {
		t.Fatalf("Unexpected number of records: expected != 0, got=%d", len(records))
	}

	//////
	// Run the pipeline.
	//////

	processedRecords, err := p.Run(ctx, records)
	if err != nil {
		t.Fatal("run failed", err)
	}

	//////
	// Write data.
	//////

	if err := esAdapter.Upsert(ctx, processedRecords); err != nil {
		t.Fatal("write failed", err)
	}

	//////
	// Should have changed the data.
	//////

	updatedRecords, err := esAdapter.Read(ctx,
		option.WithFields([]string{"name", "age", "created_at", "updated_at"}),
		option.WithLimit(100),
		option.WithSort(map[string]string{
			"updated_at": "desc",
		}),
		option.WithQuery(`
{
	"query": {
		"range": { "age": { "gt": 26 } }
	}
}`))
	if err != nil {
		t.Fatal(err)
	}

	////
	// Validates changes in `processedRecords`.
	////

	// Iterate over processed records and check if age is doubled.
	for _, uR := range updatedRecords {
		// Check if the first char of the name is uppercased.
		if uR.Name[0] != strings.ToUpper(string(uR.Name[0]))[0] {
			t.Fatalf("Name should be uppercased, got=%s", uR.Name)
		}

		// Check if age was processed.
		if uR.Age != 2704 {
			t.Fatalf("Age should be 2704, got=%d", uR.Age)
		}

		// Check if updated_at was set
		if uR.UpdatedAt.IsZero() {
			t.Fatal("UpdatedAt should not be nil")
		}
	}
}
