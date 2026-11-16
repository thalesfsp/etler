package checkpoint_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/thalesfsp/etler/v4/checkpoint"
)

// Demonstrates a file-backed store surviving a restart.
func ExampleNewFile() {
	dir, err := os.MkdirTemp("", "checkpoint-example")
	if err != nil {
		log.Fatalln(err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	path := filepath.Join(dir, "checkpoints.json")

	before, err := checkpoint.NewFile(path)
	if err != nil {
		log.Fatalln(err)
	}

	if err := before.Save(context.Background(), "nightly-2026-11-16", checkpoint.Record{
		Key:      "6",
		Status:   checkpoint.StatusFailed,
		Error:    "timeout",
		Attempts: 1,
	}); err != nil {
		log.Fatalln(err)
	}

	// A new process opens the same file.
	after, err := checkpoint.NewFile(path)
	if err != nil {
		log.Fatalln(err)
	}

	records, err := after.Load(context.Background(), "nightly-2026-11-16")
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Println(records["6"].Status, records["6"].Attempts, records["6"].Error)

	// Output:
	// failed 1 timeout
}
