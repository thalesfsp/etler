package csv

import (
	"encoding/csv"
	"io"
	"regexp"

	"github.com/thalesfsp/etler/internal/shared"
)

// Compile the regular expression
var pattern = regexp.MustCompile(`\s+`)

// LoadCSV loads a CSV file and converts it to JSON.
func LoadCSV[T any](r io.Reader) (T, error) {
	// Create a new CSV reader
	reader := csv.NewReader(r)

	// Read all the records
	records, err := reader.ReadAll()
	if err != nil {
		return *new(T), err
	}

	// Define a slice to hold the JSON objects
	var jsonData []map[string]string

	// Get the header row to use as keys
	header := records[0]

	// Process each record (except the header row)
	for _, record := range records[1:] {
		obj := make(map[string]string)

		// Create key-value pairs using the header and record values
		for i, value := range record {
			// Remove whitespace characters from the string using the compiled regex
			obj[header[i]] = pattern.ReplaceAllString(value, "")
		}

		// Append the object to the jsonData slice
		jsonData = append(jsonData, obj)
	}

	// Convert the JSON data to a byte slice
	jsonBytes, err := shared.Marshal(jsonData)
	if err != nil {
		return *new(T), err
	}

	// Unmarshal jsonBytes against T.
	var t T

	if err := shared.Unmarshal(jsonBytes, &t); err != nil {
		return *new(T), err
	}

	// Print the JSON data as a string
	return t, nil
}
