package csv

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"reflect"

	"github.com/gocarina/gocsv"
	"github.com/thalesfsp/customerror/v2"
)

//////
// Consts, vars and types.
//////

var utf8BOM = []byte{0xef, 0xbb, 0xbf}

//////
// Helpers.
//////

// Load a CSV file and converts it to target struct (Out).
func Load[Out any](in io.Reader) (Out, error) {
	outType := reflect.TypeOf((*Out)(nil)).Elem()
	if outType.Kind() != reflect.Slice {
		return *new(Out), customerror.NewInvalidError(
			fmt.Sprintf("CSV output type %s, it must be a slice", outType),
		)
	}

	reader := bufio.NewReader(in)
	prefix, _ := reader.Peek(len(utf8BOM))
	if bytes.Equal(prefix, utf8BOM) {
		_, _ = reader.Discard(len(utf8BOM))
	}

	if _, err := reader.Peek(1); err != nil {
		if errors.Is(err, io.EOF) {
			return *new(Out), customerror.NewMissingError("CSV header row")
		}

		return *new(Out), err
	}

	var out Out
	if err := gocsv.Unmarshal(reader, &out); err != nil {
		return *new(Out), err
	}

	return out, nil
}
