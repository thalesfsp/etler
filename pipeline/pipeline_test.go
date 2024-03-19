package pipeline

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/thalesfsp/status"

	"github.com/thalesfsp/etler/v2/processor"
	"github.com/thalesfsp/etler/v2/stage"
)

// Number is a simple struct to be used ProcessedData the tests.
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

func TestPipeline_syncro(t *testing.T) {
	// Context with timeout. It controls the pipeline execution.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	//////
	// Setup processors.
	//////

	double, err := processor.New(
		"double",
		"doubles the input",
		func(ctx context.Context, ProcessedData []TestUser) ([]TestUser, error) {
			ConvertedOut := make([]TestUser, len(ProcessedData))

			for i, v := range ProcessedData {
				ConvertedOut[i] = v
				ConvertedOut[i].Name = v.Name + "-double"
				ConvertedOut[i].Age = v.Age * 2
			}

			return ConvertedOut, nil
		},
		processor.WithOnFinished(func(ctx context.Context, p processor.IProcessor[TestUser], originalIn []TestUser, processedOut []TestUser) {
			fmt.Println(p.GetName(), "finished")
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	square, err := processor.New(
		"square",
		"squares the input",
		func(ctx context.Context, ProcessedData []TestUser) ([]TestUser, error) {
			ConvertedOut := make([]TestUser, len(ProcessedData))

			for i, v := range ProcessedData {
				ConvertedOut[i] = v
				ConvertedOut[i].Name = v.Name + "-square"
				ConvertedOut[i].Age = v.Age * v.Age
			}

			return ConvertedOut, nil
		},
		processor.WithOnFinished(func(ctx context.Context, p processor.IProcessor[TestUser], originalIn []TestUser, processedOut []TestUser) {
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
		"double stage",
		func(ctx context.Context, tu TestUser) (TestUserUpdate, error) {
			return TestUserUpdate{
				Age:       tu.Age,
				Code:      fmt.Sprintf("%s-%d", tu.Name, tu.Age),
				CreatedAt: tu.CreatedAt,
				Name:      tu.Name,
			}, nil
		},
		// Add as many as you want.
		double,
	)
	if err != nil {
		t.Fatal(err)
	}

	stg2, err := stage.New(
		"stage-2",
		"square stage",
		func(ctx context.Context, tu TestUser) (TestUserUpdate, error) {
			return TestUserUpdate{
				Age:       tu.Age,
				Code:      fmt.Sprintf("%s-%d", tu.Name, tu.Age),
				CreatedAt: tu.CreatedAt,
				Name:      tu.Name,
			}, nil
		},
		// Add as many as you want.
		square,
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
		stg1, stg2,
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

	outputTasks, err := p.Run(ctx, records)
	if err != nil {
		t.Fatal(err)
	}

	// Validates processors metrics.
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

	// Validates stages metrics.
	assert.Equal(t, int64(1), stg1.GetCounterCreated().Value())
	assert.Equal(t, int64(1), stg1.GetCounterRunning().Value())
	assert.Equal(t, int64(0), stg1.GetCounterFailed().Value())
	assert.Equal(t, int64(1), stg1.GetCounterDone().Value())
	assert.Equal(t, status.Done.String(), stg1.GetStatus().Value())

	assert.Equal(t, int64(1), stg2.GetCounterCreated().Value())
	assert.Equal(t, int64(1), stg2.GetCounterRunning().Value())
	assert.Equal(t, int64(0), stg2.GetCounterFailed().Value())
	assert.Equal(t, int64(1), stg2.GetCounterDone().Value())
	assert.Equal(t, status.Done.String(), stg2.GetStatus().Value())

	// Validates pipeline metrics.
	assert.Equal(t, int64(1), p.GetCounterCreated().Value())
	assert.Equal(t, int64(1), p.GetCounterRunning().Value())
	assert.Equal(t, int64(0), p.GetCounterFailed().Value())
	assert.Equal(t, int64(1), p.GetCounterDone().Value())
	assert.Equal(t, status.Done.String(), p.GetStatus().Value())

	// Validates processed data.
	assert.Len(t, outputTasks, 1)

	assert.Equal(t, "jack-double-square", outputTasks[0].ConvertedData[0].Name)
	assert.Equal(t, "john-double-square", outputTasks[0].ConvertedData[1].Name)
	assert.Equal(t, 2704, outputTasks[0].ConvertedData[0].Age)
	assert.Equal(t, 4624, outputTasks[0].ConvertedData[1].Age)
}

func TestPipeline_concurrent(t *testing.T) {
	// Context with timeout. It controls the pipeline execution.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	//////
	// Setup processors.
	//////

	double, err := processor.New(
		"double",
		"doubles the input",
		func(ctx context.Context, ProcessedData []TestUser) ([]TestUser, error) {
			ConvertedOut := make([]TestUser, len(ProcessedData))

			for i, v := range ProcessedData {
				ConvertedOut[i] = v
				ConvertedOut[i].Name = v.Name + "-double"
				ConvertedOut[i].Age = v.Age * 2
			}

			return ConvertedOut, nil
		},
		processor.WithOnFinished(func(ctx context.Context, p processor.IProcessor[TestUser], originalIn []TestUser, processedOut []TestUser) {
			fmt.Println(p.GetName(), "finished")
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	square, err := processor.New(
		"square",
		"squares the input",
		func(ctx context.Context, ProcessedData []TestUser) ([]TestUser, error) {
			ConvertedOut := make([]TestUser, len(ProcessedData))

			for i, v := range ProcessedData {
				ConvertedOut[i] = v
				ConvertedOut[i].Name = v.Name + "-square"
				ConvertedOut[i].Age = v.Age * v.Age
			}

			return ConvertedOut, nil
		},
		processor.WithOnFinished(func(ctx context.Context, p processor.IProcessor[TestUser], originalIn []TestUser, processedOut []TestUser) {
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
		"double stage",
		func(ctx context.Context, tu TestUser) (TestUserUpdate, error) {
			return TestUserUpdate{
				Age:       tu.Age,
				Code:      fmt.Sprintf("%s-%d", tu.Name, tu.Age),
				CreatedAt: tu.CreatedAt,
				Name:      tu.Name,
			}, nil
		},
		// Add as many as you want.
		double,
	)
	if err != nil {
		t.Fatal(err)
	}

	stg2, err := stage.New(
		"stage-2",
		"square stage",
		func(ctx context.Context, tu TestUser) (TestUserUpdate, error) {
			return TestUserUpdate{
				Age:       tu.Age,
				Code:      fmt.Sprintf("%s-%d", tu.Name, tu.Age),
				CreatedAt: tu.CreatedAt,
				Name:      tu.Name,
			}, nil
		},
		// Add as many as you want.
		square,
	)
	if err != nil {
		t.Fatal(err)
	}

	//////
	// Setup pipeline.
	//////

	// Create a new pipeline.
	p, err := New("User Enhancer", "Enhances user data", true,
		// Add as many as you want.
		stg1, stg2,
	)
	if err != nil {
		t.Fatal(err)
	}

	//////
	// Run the pipeline.
	//////

	records := []TestUser{
		{
			Name:      "jack",
			Age:       26,
			CreatedAt: time.Now(),
		},
		{
			Name:      "john",
			Age:       34,
			CreatedAt: time.Now(),
		},
	}

	outputTasks, err := p.Run(ctx, records)
	if err != nil {
		t.Fatal(err)
	}

	// Validates processors metrics.
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

	// Validates stages metrics.
	assert.Equal(t, int64(1), stg1.GetCounterCreated().Value())
	assert.Equal(t, int64(1), stg1.GetCounterRunning().Value())
	assert.Equal(t, int64(0), stg1.GetCounterFailed().Value())
	assert.Equal(t, int64(1), stg1.GetCounterDone().Value())
	assert.Equal(t, status.Done.String(), stg1.GetStatus().Value())

	assert.Equal(t, int64(1), stg2.GetCounterCreated().Value())
	assert.Equal(t, int64(1), stg2.GetCounterRunning().Value())
	assert.Equal(t, int64(0), stg2.GetCounterFailed().Value())
	assert.Equal(t, int64(1), stg2.GetCounterDone().Value())
	assert.Equal(t, status.Done.String(), stg2.GetStatus().Value())

	// Validates pipeline metrics.
	assert.Equal(t, int64(1), p.GetCounterCreated().Value())
	assert.Equal(t, int64(1), p.GetCounterRunning().Value())
	assert.Equal(t, int64(0), p.GetCounterFailed().Value())
	assert.Equal(t, int64(1), p.GetCounterDone().Value())
	assert.Equal(t, status.Done.String(), p.GetStatus().Value())

	// Validates processed data.
	assert.Len(t, outputTasks, 2)

	assert.Equal(t, "jack-double", outputTasks[0].ConvertedData[0].Name)
	assert.Equal(t, "john-double", outputTasks[0].ConvertedData[1].Name)
	assert.Equal(t, 52, outputTasks[0].ConvertedData[0].Age)
	assert.Equal(t, 68, outputTasks[0].ConvertedData[1].Age)

	assert.Equal(t, "jack-square", outputTasks[1].ConvertedData[0].Name)
	assert.Equal(t, "john-square", outputTasks[1].ConvertedData[1].Name)
	assert.Equal(t, 676, outputTasks[1].ConvertedData[0].Age)
	assert.Equal(t, 1156, outputTasks[1].ConvertedData[1].Age)
}
