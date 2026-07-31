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

type csvField struct {
	key  string
	fold bool
}

//////
// Loader.
//////

// Load converts CSV data to a slice of structs. Untagged field names match
// case-insensitively through gocsv's per-unmarshaller header normalizer; gocsv
// resolves all field collisions in declaration order. Output types using csv[]
// slice expansion are unsupported.
//
// Numeric conversion uses gocsv's permissive, not range-safe semantics: 010
// decodes as 8, 1_0 as 10, and 127.9 into an integer as 127. Out-of-range
// values may wrap or become infinite. Callers requiring a numeric syntax or
// range guarantee should decode into a string field and parse it themselves.
// Other behavior comes from gocsv's Unmarshaller. Its package-level tag, header,
// and duplicate settings influence Load; its reader factory and
// FailIfUnmatchedStructTags do not.
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
	fields, err := inspectCSVFields(rowType, "", true, nil)
	if err != nil {
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
	source, err := io.ReadAll(reader)
	if err != nil {
		return *new(Out), err
	}
	if _, err := stdcsv.NewReader(bytes.NewReader(source)).ReadAll(); err != nil {
		return *new(Out), err
	}
	csvReader := stdcsv.NewReader(bytes.NewReader(source))
	prototype := reflect.New(rowType)
	if outType.Elem().Kind() != reflect.Pointer {
		prototype = prototype.Elem()
	}
	unmarshaller, err := gocsv.NewUnmarshaller(csvReader, prototype.Interface())
	if err != nil {
		return *new(Out), err
	}
	headers := normalizeCSVHeaders(unmarshaller.Headers, fields)
	if err := unmarshaller.RenormalizeHeaders(func([]string) []string {
		return headers
	}); err != nil {
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

func inspectCSVFields(rowType reflect.Type, parentKey string, parentFold bool, active map[reflect.Type]struct{}) ([]csvField, error) {
	if rowType.Kind() != reflect.Struct {
		return nil, nil
	}
	if active == nil {
		active = make(map[reflect.Type]struct{})
	}
	if _, exists := active[rowType]; exists {
		return nil, customerror.NewInvalidError(fmt.Sprintf("cyclic CSV output type %s", rowType))
	}
	active[rowType] = struct{}{}
	defer delete(active, rowType)
	fields := make([]csvField, 0, rowType.NumField())
	for index := range rowType.NumField() {
		field := rowType.Field(index)
		if !field.IsExported() {
			continue
		}
		key, fold, ignored := csvFieldName(field)
		if ignored {
			continue
		}
		if _, expands := field.Tag.Lookup(gocsv.TagName + "[]"); expands &&
			(field.Type.Kind() == reflect.Slice || field.Type.Kind() == reflect.Array) {
			return nil, customerror.NewInvalidError(fmt.Sprintf("CSV output type %s uses unsupported csv[] slice expansion on field %s", rowType, field.Name))
		}
		if field.Anonymous {
			key, fold = parentKey, parentFold
		} else {
			fold = parentFold && fold
			if fold && parentKey != "" {
				key = parentKey + gocsv.FieldsCombiner + key
			}
		}
		fieldType := field.Type
		if fieldType.Kind() == reflect.Pointer {
			fieldType = fieldType.Elem()
		}
		if fieldType.Kind() == reflect.Struct && !hasCustomUnmarshaller(fieldType) {
			nested, err := inspectCSVFields(fieldType, key, fold, active)
			if err != nil {
				return nil, err
			}
			fields = append(fields, nested...)
			continue
		}
		if (fieldType.Kind() == reflect.Slice || fieldType.Kind() == reflect.Array) &&
			fieldType.Elem().Kind() == reflect.Struct {
			if _, err := inspectCSVFields(fieldType.Elem(), "", true, active); err != nil {
				return nil, err
			}
		}
		if field.Anonymous {
			continue
		}
		fields = append(fields, csvField{key: key, fold: fold})
	}
	return fields, nil
}

func csvFieldName(field reflect.StructField) (string, bool, bool) {
	parts := strings.Split(field.Tag.Get(gocsv.TagName), gocsv.TagSeparator)
	keys := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		switch {
		case strings.HasPrefix(part, "partial"), part == "omitempty", strings.HasPrefix(part, "default="):
		default:
			keys = append(keys, part)
		}
	}
	if len(keys) == 1 && keys[0] == "-" {
		return "", false, true
	}
	if len(keys) == 0 || keys[0] == "" {
		return field.Name, true, false
	}
	return "", false, false
}

func hasCustomUnmarshaller(typ reflect.Type) bool {
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	return typ.Implements(csvUnmarshallerType) || typ.Implements(textUnmarshallerType) ||
		reflect.PointerTo(typ).Implements(csvUnmarshallerType) || reflect.PointerTo(typ).Implements(textUnmarshallerType)
}

func normalizeCSVHeaders(headers []string, fields []csvField) []string {
	normalized := append([]string(nil), headers...)
	for index, header := range normalized {
		for _, field := range fields {
			if field.fold && strings.EqualFold(header, field.key) {
				normalized[index] = field.key
				break
			}
		}
	}
	return normalized
}
