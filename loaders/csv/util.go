package csv

import (
	"bufio"
	"bytes"
	stdcsv "encoding/csv"
	"errors"
	"fmt"
	"io"
	"reflect"

	"github.com/gocarina/gocsv"
	"github.com/thalesfsp/customerror/v2"
	"github.com/thalesfsp/etler/v4/internal/csvshape"
)

//////
// Consts, vars and types.
//////

var validateCSVOutputType = csvshape.ValidateOutput

//////
// Loader.
//////

// Load converts CSV data to a slice. It rejects nil readers and non-slice output
// types. Two row shapes are rejected up front: cycles, which make gocsv recurse
// until the process dies of an unrecoverable stack overflow, and lossy csv[]
// expansion forms. Direct slice or array fields with a positive csv[] count are
// supported; pointer-wrapped fields and invalid counts are rejected because
// gocsv silently discards their columns. Other type incompatibilities are
// delegated to gocsv; any panic is returned as an error naming the output type
// and recovered value. That guard also covers a panic raised by a caller-supplied
// UnmarshalCSV or UnmarshalText, which is reported the same way.
//
// Cycle rejection walks a safe superset of gocsv's traversal. A csv:"-" field
// does not exempt its type from the cycle check: gocsv applies its process-wide
// header normalizer before deciding whether to skip the field, and that
// normalizer cannot be read, so the skip cannot be mirrored safely. A field type
// implementing MarshalCSV, MarshalText, UnmarshalCSV, or
// UnmarshalCSVWithFields is a leaf, and only one pointer level is followed. A
// type implementing only encoding.TextUnmarshaler is not a leaf, because gocsv
// descends into it. Anonymous struct fields are traversed, while anonymous slice
// and array fields are not, matching gocsv's unconditional handling of those
// forms. The csv[] check follows every pointer level to find ignored tags.
//
// By default gocsv resolves field keys case-sensitively, but header matching is
// not byte-exact: Name also matches a header with surrounding whitespace or
// U+200B, U+200C, U+200D, and U+FEFF zero-width characters removed. A
// process-wide gocsv header normalizer can change that rule; use explicit csv
// tags when accepted spelling must be unambiguous. Two collisions resolve
// silently: a header matching a field shadowed by an outer field of the same
// name fills the shadowed embedded field, and when two fields share a csv tag
// only the first is filled.
//
// Numeric conversion uses gocsv's permissive, not range-safe semantics: 010
// decodes as 8, 1_0 as 10, and 127.9 into an integer as 127. Out-of-range
// values may wrap or become infinite. Callers requiring a numeric syntax or
// range guarantee should decode into a string field and parse it themselves.
// Load strips one leading UTF-8 BOM and constructs encoding/csv.NewReader
// directly. Other decoding behavior comes from gocsv's Unmarshaller. Its
// package-level tag, header, and duplicate settings influence Load; its reader
// factory and FailIfUnmatchedStructTags do not.
func Load[Out any](in io.Reader) (Out, error) {
	outType := reflect.TypeOf((*Out)(nil)).Elem()
	if outType.Kind() != reflect.Slice {
		return *new(Out), customerror.NewInvalidError(
			fmt.Sprintf("CSV output type %s, it must be a slice", outType),
		)
	}
	rowType := outType.Elem()
	if rowType.Kind() == reflect.Pointer {
		rowType = rowType.Elem()
	}
	if err := validateCSVOutputType(rowType); err != nil {
		return *new(Out), err
	}
	if isNil(in) {
		return *new(Out), customerror.NewRequiredError("CSV reader")
	}
	reader := bufio.NewReader(in)
	bom := []byte{0xef, 0xbb, 0xbf}
	prefix, _ := reader.Peek(len(bom))
	if bytes.Equal(prefix, bom) {
		_, _ = reader.Discard(len(bom))
	}
	if _, err := reader.Peek(1); err != nil {
		if errors.Is(err, io.EOF) {
			return *new(Out), customerror.NewMissingError("CSV header row")
		}
		return *new(Out), err
	}
	prototype := reflect.New(rowType)
	if outType.Elem().Kind() != reflect.Pointer {
		prototype = prototype.Elem()
	}
	prototypeValue := prototype.Interface()
	unmarshaller, err := callGocsv(outType, func() (*gocsv.Unmarshaller, error) {
		return gocsv.NewUnmarshaller(stdcsv.NewReader(reader), prototypeValue)
	})
	if err != nil {
		return *new(Out), err
	}
	var out Out
	outValue := reflect.ValueOf(&out).Elem()
	for {
		row, err := callGocsv(outType, unmarshaller.Read)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return *new(Out), err
		}
		outValue.Set(reflect.Append(outValue, reflect.ValueOf(row)))
	}
	return out, nil
}

//////
// Helpers.
//////

func isNil(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

func callGocsv[Result any](outType reflect.Type, call func() (Result, error)) (result Result, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("gocsv panic for CSV output type %s: %v", outType, recovered)
		}
	}()

	return call()
}
