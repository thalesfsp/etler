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
	"strconv"
	"sync"

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
	csvOutputTypeValidationCache  sync.Map
)

type csvOutputTypeValidationCacheKey struct {
	rowType reflect.Type
	tagName string
}

type csvOutputTypeValidationCacheValue struct {
	err error
}

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

func validateCSVOutputType(rowType reflect.Type) error {
	return validateCSVOutputTypeWithTagName(rowType, nil, gocsv.TagName)
}

func validateCSVOutputTypeWithTagName(
	rowType reflect.Type,
	active map[reflect.Type]struct{},
	tagName string,
) (err error) {
	if rowType.Kind() != reflect.Struct {
		return nil
	}
	if active == nil {
		active = make(map[reflect.Type]struct{})
	}
	if _, exists := active[rowType]; exists {
		return customerror.NewInvalidError(fmt.Sprintf("cyclic CSV output type %s", rowType))
	}

	cacheKey := csvOutputTypeValidationCacheKey{rowType: rowType, tagName: tagName}
	if cached, exists := csvOutputTypeValidationCache.Load(cacheKey); exists {
		return cached.(csvOutputTypeValidationCacheValue).err
	}

	active[rowType] = struct{}{}
	defer delete(active, rowType)
	defer func() {
		csvOutputTypeValidationCache.Store(cacheKey, csvOutputTypeValidationCacheValue{err: err})
	}()

	for index := range rowType.NumField() {
		field := rowType.Field(index)
		if field.PkgPath != "" {
			continue
		}
		// This deliberately follows every pointer level even though gocsv's
		// traversal follows one: accepting an ignored csv[] tag loses columns.
		expansionType := field.Type
		for expansionType.Kind() == reflect.Pointer {
			expansionType = expansionType.Elem()
		}
		if expansionCount, expands := field.Tag.Lookup(tagName + "[]"); expands &&
			(expansionType.Kind() == reflect.Slice || expansionType.Kind() == reflect.Array) {
			if field.Type.Kind() != reflect.Slice && field.Type.Kind() != reflect.Array {
				return customerror.NewInvalidError(fmt.Sprintf(
					"CSV output type %s uses unsupported csv[] slice expansion on field %s: pointer-wrapped slice or array fields are ignored by gocsv; move the tag to a direct slice or array field",
					rowType,
					field.Name,
				))
			}
			count, err := strconv.Atoi(expansionCount)
			if err != nil || count <= 0 {
				return customerror.NewInvalidError(fmt.Sprintf(
					"CSV output type %s uses unsupported csv[] slice expansion on field %s: count %q is non-numeric or non-positive; use a positive integer count",
					rowType,
					field.Name,
					expansionCount,
				))
			}
		}

		structType := field.Type
		if structType.Kind() == reflect.Pointer {
			structType = structType.Elem()
		}
		if structType.Kind() == reflect.Struct && !gocsvCanMarshal(structType) {
			if err := validateCSVOutputTypeWithTagName(structType, active, tagName); err != nil {
				return err
			}
			continue
		}
		if !field.Anonymous &&
			(field.Type.Kind() == reflect.Slice || field.Type.Kind() == reflect.Array) &&
			field.Type.Elem().Kind() == reflect.Struct {
			if err := validateCSVOutputTypeWithTagName(field.Type.Elem(), active, tagName); err != nil {
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
