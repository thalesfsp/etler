package csv

import (
	"bufio"
	"bytes"
	"encoding"
	stdcsv "encoding/csv"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/gocarina/gocsv"
	"github.com/thalesfsp/customerror/v2"
)

//////
// Consts, vars and types.
//////

var (
	csvMarshallerType             = reflect.TypeFor[gocsv.TypeMarshaller]()
	textMarshallerType            = reflect.TypeFor[encoding.TextMarshaler]()
	csvUnmarshallerType           = reflect.TypeFor[gocsv.TypeUnmarshaller]()
	csvWithFieldsUnmarshallerType = reflect.TypeFor[gocsv.TypeUnmarshalCSVWithFields]()
)

//////
// Loader.
//////

// Load converts CSV data to a slice. It rejects nil readers and non-slice output
// types. Two row shapes are rejected up front: cycles, which make gocsv recurse
// until the process dies of an unrecoverable stack overflow, and csv[]
// expansion, which can silently discard columns. Other type incompatibilities
// are delegated to gocsv; any panic is returned as an error naming the output
// type and recovered value. That guard also covers a panic raised by a
// caller-supplied UnmarshalCSV or UnmarshalText, which is reported the same way.
//
// Cycle rejection mirrors gocsv's own traversal, so a shape gocsv collapses to a
// single value is accepted: a field type implementing MarshalCSV, MarshalText,
// UnmarshalCSV, or UnmarshalCSVWithFields is a leaf, a csv:"-" tag on a named
// field skips it, and only one pointer level is followed. A type implementing
// only encoding.TextUnmarshaler is not a leaf, because gocsv descends into it.
// The csv[] check is deliberately stricter than gocsv and follows every pointer
// level, because gocsv ignores that tag on a *[]T field and loses the columns.
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
	if err := validateCSVOutputType(rowType, nil); err != nil {
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

func validateCSVOutputType(rowType reflect.Type, active map[reflect.Type]struct{}) error {
	if rowType.Kind() != reflect.Struct {
		return nil
	}
	if active == nil {
		active = make(map[reflect.Type]struct{})
	}
	if _, exists := active[rowType]; exists {
		return customerror.NewInvalidError(fmt.Sprintf("cyclic CSV output type %s", rowType))
	}
	active[rowType] = struct{}{}
	defer delete(active, rowType)
	for index := range rowType.NumField() {
		field := rowType.Field(index)
		if field.PkgPath != "" {
			continue
		}
		if !field.Anonymous && gocsvSkipsField(field) {
			continue
		}

		// This deliberately follows every pointer level even though gocsv's
		// traversal follows one: accepting an ignored csv[] tag loses columns.
		expansionType := field.Type
		for expansionType.Kind() == reflect.Pointer {
			expansionType = expansionType.Elem()
		}
		if _, expands := field.Tag.Lookup(gocsv.TagName + "[]"); expands &&
			(expansionType.Kind() == reflect.Slice || expansionType.Kind() == reflect.Array) {
			return customerror.NewInvalidError(fmt.Sprintf("CSV output type %s uses unsupported csv[] slice expansion on field %s", rowType, field.Name))
		}

		structType := field.Type
		if structType.Kind() == reflect.Pointer {
			structType = structType.Elem()
		}
		if structType.Kind() == reflect.Struct && !gocsvCanMarshal(structType) {
			if err := validateCSVOutputType(structType, active); err != nil {
				return err
			}
			continue
		}
		if (field.Type.Kind() == reflect.Slice || field.Type.Kind() == reflect.Array) &&
			field.Type.Elem().Kind() == reflect.Struct {
			if err := validateCSVOutputType(field.Type.Elem(), active); err != nil {
				return err
			}
		}
	}
	return nil
}

func callGocsv[Result any](outType reflect.Type, call func() (Result, error)) (result Result, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("gocsv panic for CSV output type %s: %v", outType, recovered)
		}
	}()

	return call()
}

func gocsvSkipsField(field reflect.StructField) bool {
	filteredTags := make([]string, 0, 1)
	for _, tag := range strings.Split(field.Tag.Get(gocsv.TagName), gocsv.TagSeparator) {
		tag = strings.TrimSpace(tag)
		if tag == "omitempty" || strings.HasPrefix(tag, "partial") || strings.HasPrefix(tag, "default=") {
			continue
		}
		filteredTags = append(filteredTags, tag)
	}

	return len(filteredTags) == 1 && filteredTags[0] == "-"
}

func gocsvCanMarshal(typ reflect.Type) bool {
	if typ.Implements(csvMarshallerType) ||
		typ.Implements(textMarshallerType) ||
		typ.Implements(csvUnmarshallerType) ||
		typ.Implements(csvWithFieldsUnmarshallerType) {
		return true
	}

	pointerType := reflect.PointerTo(typ)
	return pointerType.Implements(csvMarshallerType) ||
		pointerType.Implements(textMarshallerType) ||
		pointerType.Implements(csvUnmarshallerType) ||
		pointerType.Implements(csvWithFieldsUnmarshallerType)
}
