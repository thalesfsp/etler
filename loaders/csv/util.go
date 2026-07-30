package csv

import (
	"encoding/csv"
	"io"
	"strings"

	"github.com/thalesfsp/customerror/v2"
	"github.com/thalesfsp/etler/v3/internal/shared"
)

//////
// Helpers.
//////

// Load a CSV file and converts it to target struct (Out).
func Load[Out any](in io.Reader) (Out, error) {
	// Create a new CSV reader
	reader := csv.NewReader(in)

	// Read all the records
	records, err := reader.ReadAll()
	if err != nil {
		return *new(Out), err
	}

	// A header row is required to map values to fields.
	if len(records) == 0 {
		return *new(Out), customerror.NewMissingError("CSV header row")
	}

	// Define a slice to hold the JSON objects
	var jsonData []map[string]string
	if len(records) > 1 {
		jsonData = make([]map[string]string, 0, len(records)-1)
	}

	// Get the header row to use as keys
	header := records[0]

	// Process each record (except the header row)
	for _, record := range records[1:] {
		obj := make(map[string]string, len(header))

		// Create key-value pairs using the header and record values
		for i, value := range record {
			// Remove tab characters from the string.
			obj[header[i]] = strings.ReplaceAll(value, "\t", "")
		}

		// Append the object to the jsonData slice
		jsonData = append(jsonData, obj)
	}

	// Convert the JSON data to a byte slice
	jsonBytes, err := shared.Marshal(jsonData)
	if err != nil {
		return *new(Out), err
	}

	// Unmarshal jsonBytes against Out.
	var out Out

	if err := shared.Unmarshal(jsonBytes, &out); err != nil {
		return *new(Out), err
	}

	// Print the JSON data as a string
	return out, nil
}
