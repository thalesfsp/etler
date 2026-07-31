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
	csvUnmarshallerType  = reflect.TypeFor[gocsv.TypeUnmarshaller]()
	textUnmarshallerType = reflect.TypeFor[encoding.TextUnmarshaler]()
)

//////
// Loader.
//////

// Load converts CSV data to a slice of structs. By default gocsv resolves field
// keys case-sensitively, but header matching is not byte-exact: Name also
// matches a header with surrounding whitespace or U+200B, U+200C, U+200D, and
// U+FEFF zero-width characters removed. A process-wide gocsv header normalizer
// can change that rule; use explicit csv tags when accepted spelling must be
// unambiguous. Load rejects nil readers and output types that are not slices,
// whose elements are not structs or pointers to structs, that are cyclic, that
// use csv[] slice expansion, or that contain anonymous non-struct fields or
// interface fields (including pointers to interfaces) not tagged csv:"-".
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
	if rowType.Kind() != reflect.Struct {
		return *new(Out), customerror.NewInvalidError(fmt.Sprintf(
			"CSV output type %s uses unsupported element type %s, which must be a struct or pointer to a struct",
			outType,
			outType.Elem(),
		))
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
	unmarshaller, err := gocsv.NewUnmarshaller(stdcsv.NewReader(reader), prototype.Interface())
	if err != nil {
		return *new(Out), err
	}
	var out Out
	outValue := reflect.ValueOf(&out).Elem()
	for {
		row, err := unmarshaller.Read()
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
		if !field.IsExported() {
			continue
		}
		tagParts := strings.Split(field.Tag.Get(gocsv.TagName), gocsv.TagSeparator)
		if len(tagParts) == 1 && strings.TrimSpace(tagParts[0]) == "-" {
			continue
		}
		fieldType := field.Type
		if fieldType.Kind() == reflect.Pointer {
			fieldType = fieldType.Elem()
		}
		if field.Anonymous && fieldType.Kind() != reflect.Struct {
			return customerror.NewInvalidError(fmt.Sprintf(
				"CSV output type %s uses unsupported anonymous field %s with non-struct type %s",
				rowType,
				field.Name,
				field.Type,
			))
		}
		if fieldType.Kind() == reflect.Interface {
			return customerror.NewInvalidError(fmt.Sprintf(
				"CSV output type %s uses unsupported interface field %s with type %s",
				rowType,
				field.Name,
				field.Type,
			))
		}
		if _, expands := field.Tag.Lookup(gocsv.TagName + "[]"); expands &&
			(field.Type.Kind() == reflect.Slice || field.Type.Kind() == reflect.Array) {
			return customerror.NewInvalidError(fmt.Sprintf("CSV output type %s uses unsupported csv[] slice expansion on field %s", rowType, field.Name))
		}
		if fieldType.Kind() == reflect.Struct && !hasCustomUnmarshaller(fieldType) {
			if err := validateCSVOutputType(fieldType, active); err != nil {
				return err
			}
			continue
		}
		if (fieldType.Kind() == reflect.Slice || fieldType.Kind() == reflect.Array) &&
			fieldType.Elem().Kind() == reflect.Struct {
			if err := validateCSVOutputType(fieldType.Elem(), active); err != nil {
				return err
			}
		}
	}
	return nil
}

func hasCustomUnmarshaller(typ reflect.Type) bool {
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	return typ.Implements(csvUnmarshallerType) || typ.Implements(textUnmarshallerType) ||
		reflect.PointerTo(typ).Implements(csvUnmarshallerType) || reflect.PointerTo(typ).Implements(textUnmarshallerType)
}
