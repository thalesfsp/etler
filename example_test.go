package etler_test

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/thalesfsp/etler/v4/converter"
	loadercsv "github.com/thalesfsp/etler/v4/loaders/csv"
	"github.com/thalesfsp/etler/v4/pipeline"
	"github.com/thalesfsp/etler/v4/processor"
	"github.com/thalesfsp/etler/v4/stage"
)

// Demonstrates a complete typed CSV-to-pipeline flow.
func Example() {
	type record struct {
		Name   string
		Points int
		Active bool
	}

	type summary struct {
		Name   string
		Points int
		Active bool
	}

	source, err := loadercsv.New[record]()
	if err != nil {
		log.Fatalln(err)
	}

	records, err := source.Run(
		context.Background(),
		strings.NewReader("Name,Points,Active\nalice,2,true\nbob,3,false\n"),
	)
	if err != nil {
		log.Fatalln(err)
	}

	uppercase, err := processor.New(
		"example-root-uppercase",
		"uppercases names",
		func(_ context.Context, in []record) ([]record, error) {
			out := make([]record, len(in))
			for i, value := range in {
				out[i] = value
				out[i].Name = strings.ToUpper(value.Name)
			}

			return out, nil
		},
	)
	if err != nil {
		log.Fatalln(err)
	}

	double, err := processor.New(
		"example-root-double",
		"doubles points",
		func(_ context.Context, in []record) ([]record, error) {
			out := make([]record, len(in))
			for i, value := range in {
				out[i] = value
				out[i].Points = value.Points * 2
			}

			return out, nil
		},
	)
	if err != nil {
		log.Fatalln(err)
	}

	summarize, err := converter.New(
		"example-root-summary",
		"converts records to summaries",
		func(_ context.Context, in record) (summary, error) {
			return summary(in), nil
		},
	)
	if err != nil {
		log.Fatalln(err)
	}

	first, err := stage.New(
		"example-root-first",
		"normalizes names",
		summarize,
		uppercase,
	)
	if err != nil {
		log.Fatalln(err)
	}

	second, err := stage.New(
		"example-root-second",
		"scores records",
		summarize,
		double,
	)
	if err != nil {
		log.Fatalln(err)
	}

	flow, err := pipeline.New(
		"example-root-pipeline",
		"normalizes and scores CSV records",
		false,
		first,
		second,
	)
	if err != nil {
		log.Fatalln(err)
	}

	tasks, err := flow.Run(context.Background(), records)
	if err != nil {
		log.Fatalln(err)
	}

	for _, value := range tasks[len(tasks)-1].ConvertedData {
		fmt.Printf("%s: points=%d active=%t\n", value.Name, value.Points, value.Active)
	}

	// Output:
	// ALICE: points=4 active=true
	// BOB: points=6 active=false
}
