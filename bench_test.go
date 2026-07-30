package etler_test

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/thalesfsp/etler/v3/converters/passthru"
	csvloader "github.com/thalesfsp/etler/v3/loaders/csv"
	"github.com/thalesfsp/etler/v3/pipeline"
	"github.com/thalesfsp/etler/v3/processor"
	"github.com/thalesfsp/etler/v3/stage"
	"github.com/thalesfsp/etler/v3/task"
)

type benchmarkCSVRow struct {
	Name string `csv:"name"`
	Age  int    `csv:"age"`
}

var (
	benchmarkCSVRowsResult   []benchmarkCSVRow
	benchmarkStageResult     task.Task[int, int]
	benchmarkPipelineResult  []task.Task[int, int]
	benchmarkProcessorResult []int
	benchmarkConverterResult int
)

func newBenchmarkProcessor(b *testing.B, name string) processor.IProcessor[int] {
	b.Helper()

	p, err := processor.New(
		name,
		"increments each item",
		func(_ context.Context, in []int) ([]int, error) {
			out := make([]int, len(in))
			for i, value := range in {
				out[i] = value + 1
			}

			return out, nil
		},
	)
	if err != nil {
		b.Fatal(err)
	}

	return p
}

func newBenchmarkStage(b *testing.B, name string) stage.IStage[int, int] {
	b.Helper()

	conv, err := passthru.New[int]()
	if err != nil {
		b.Fatal(err)
	}

	stg, err := stage.New(
		name,
		"increments and passes through each item",
		conv,
		newBenchmarkProcessor(b, name+"-processor"),
	)
	if err != nil {
		b.Fatal(err)
	}

	return stg
}

func BenchmarkLoadersCSV_Load_1kRows(b *testing.B) {
	b.ReportAllocs()

	var builder strings.Builder
	builder.WriteString("name,age\n")
	for i := 0; i < 1000; i++ {
		builder.WriteString("name")
		builder.WriteString(strconv.Itoa(i))
		builder.WriteByte(',')
		builder.WriteString(strconv.Itoa(i % 100))
		builder.WriteByte('\n')
	}

	data := builder.String()
	reader := strings.NewReader(data)
	var result []benchmarkCSVRow

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader.Reset(data)

		var err error
		result, err = csvloader.Load[[]benchmarkCSVRow](reader)
		if err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()

	benchmarkCSVRowsResult = result
}

func BenchmarkStage_Run_1kItems(b *testing.B) {
	b.ReportAllocs()

	ctx := context.Background()
	items := make([]int, 1000)
	for i := range items {
		items[i] = i
	}

	stg := newBenchmarkStage(b, "benchmark-stage-1k")
	tsk, err := task.New[int, int](items)
	if err != nil {
		b.Fatal(err)
	}

	var result task.Task[int, int]

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, err = stg.Run(ctx, tsk)
		if err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()

	benchmarkStageResult = result
}

func BenchmarkStage_Run_1Item(b *testing.B) {
	b.ReportAllocs()

	ctx := context.Background()
	stg := newBenchmarkStage(b, "benchmark-stage-1")
	tsk, err := task.New[int, int]([]int{1})
	if err != nil {
		b.Fatal(err)
	}

	var result task.Task[int, int]

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, err = stg.Run(ctx, tsk)
		if err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()

	benchmarkStageResult = result
}

func BenchmarkPipeline_Run_2Stages_1kItems(b *testing.B) {
	b.ReportAllocs()

	ctx := context.Background()
	items := make([]int, 1000)
	for i := range items {
		items[i] = i
	}

	p, err := pipeline.New(
		"benchmark-pipeline-2-stage-1k",
		"runs two sequential stages",
		false,
		newBenchmarkStage(b, "benchmark-pipeline-stage-1"),
		newBenchmarkStage(b, "benchmark-pipeline-stage-2"),
	)
	if err != nil {
		b.Fatal(err)
	}

	var result []task.Task[int, int]

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, err = p.Run(ctx, items)
		if err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()

	benchmarkPipelineResult = result
}

func BenchmarkProcessor_Run_1kItems(b *testing.B) {
	b.ReportAllocs()

	ctx := context.Background()
	items := make([]int, 1000)
	for i := range items {
		items[i] = i
	}

	p := newBenchmarkProcessor(b, "benchmark-processor-1k")
	var result []int
	var err error

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, err = p.Run(ctx, items)
		if err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()

	benchmarkProcessorResult = result
}

func BenchmarkConverter_Run_1Item(b *testing.B) {
	b.ReportAllocs()

	ctx := context.Background()
	conv, err := passthru.New[int]()
	if err != nil {
		b.Fatal(err)
	}

	var result int

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, err = conv.Run(ctx, 1)
		if err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()

	benchmarkConverterResult = result
}
