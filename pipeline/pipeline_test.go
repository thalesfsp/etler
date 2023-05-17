package pipeline

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/thalesfsp/etler/processor"
	"github.com/thalesfsp/etler/stage"
	"github.com/thalesfsp/status"
)

// Number is a simple struct to be used in the tests.
type Number struct {
	// Numbers to be processed.
	Numbers []int `json:"numbers"`
}

type TestUser struct {
	Age       int       `json:"age,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	Name      string    `json:"name"`
}

type TestUserUpdate struct {
	Age       int       `json:"age,omitempty"`
	Code      string    `json:"code"`
	CreatedAt time.Time `json:"created_at"`
	Name      string    `json:"name"`
}

func TestCSVFileAdapter_Read(t *testing.T) {
	// Context with timeout. It controls the pipeline execution.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	//////
	// Setup processors.
	//////

	double, err := processor.New(
		"double",
		"doubles the input",
		func(ctx context.Context, in []TestUser) ([]TestUser, error) {
			out := make([]TestUser, len(in))

			for i, v := range in {
				out[i] = v
				out[i].Name = v.Name + "-double"
				out[i].Age = v.Age * 2
			}

			return out, nil
		},
		processor.WithOnFinished(func(ctx context.Context, p processor.IProcessor[TestUser], originalIn []TestUser, processedIn []TestUser) {
			fmt.Println(p.GetName(), "finished")
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	square, err := processor.New(
		"square",
		"squares the input",
		func(ctx context.Context, in []TestUser) ([]TestUser, error) {
			out := make([]TestUser, len(in))

			for i, v := range in {
				out[i] = v
				out[i].Name = v.Name + "-square"
				out[i].Age = v.Age * v.Age
			}

			return out, nil
		},
		processor.WithOnFinished(func(ctx context.Context, p processor.IProcessor[TestUser], originalIn []TestUser, processedIn []TestUser) {
			fmt.Println(p.GetName(), "finished")
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	//////
	// Setup stage.
	//////

	stg1, err := stage.New(
		"stage-1",
		func(ctx context.Context, tu TestUser) (TestUserUpdate, error) {
			return TestUserUpdate{
				Age:       tu.Age,
				Code:      fmt.Sprintf("%s-%d", tu.Name, tu.Age),
				CreatedAt: tu.CreatedAt,
				Name:      tu.Name,
			}, nil
		},
		// Add as many as you want.
		double, square,
	)
	if err != nil {
		t.Fatal(err)
	}

	//////
	// Setup pipeline.
	//////

	// Create a new pipeline.
	p, err := New("User Enhancer", "Enhances user data", false,
		// Add as many as you want.
		stg1,
	)
	if err != nil {
		t.Fatal(err)
	}

	//////
	// Run the pipeline.
	//////

	records := []TestUser{
		{
			Name: "jack",
			Age:  26,
		},
		{
			Name: "john",
			Age:  34,
		},
	}

	processedRecords, err := p.Run(ctx, records)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, int64(1), double.GetCounterCreated().Value())
	assert.Equal(t, int64(1), double.GetCounterRunning().Value())
	assert.Equal(t, int64(0), double.GetCounterFailed().Value())
	assert.Equal(t, int64(1), double.GetCounterDone().Value())
	assert.Equal(t, status.Done.String(), double.GetStatus().Value())

	assert.Equal(t, int64(1), square.GetCounterCreated().Value())
	assert.Equal(t, int64(1), square.GetCounterRunning().Value())
	assert.Equal(t, int64(0), square.GetCounterFailed().Value())
	assert.Equal(t, int64(1), square.GetCounterDone().Value())
	assert.Equal(t, status.Done.String(), square.GetStatus().Value())

	assert.Equal(t, int64(1), stg1.GetCounterCreated().Value())
	assert.Equal(t, int64(1), stg1.GetCounterRunning().Value())
	assert.Equal(t, int64(0), stg1.GetCounterFailed().Value())
	assert.Equal(t, int64(1), stg1.GetCounterDone().Value())
	assert.Equal(t, status.Done.String(), stg1.GetStatus().Value())

	assert.Equal(t, int64(1), p.GetCounterCreated().Value())
	assert.Equal(t, int64(1), p.GetCounterRunning().Value())
	assert.Equal(t, int64(0), p.GetCounterFailed().Value())
	assert.Equal(t, int64(1), p.GetCounterDone().Value())
	assert.Equal(t, status.Done.String(), p.GetStatus().Value())

	//////
	// Validates changes in `processedRecords`.
	//////

	if len(processedRecords) != 2 {
		t.Fatalf("Unexpected number of out: expected=2, got=%d", len(processedRecords))
	}
}
