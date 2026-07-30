package csv

import (
	"bufio"
	"bytes"
	"encoding"
	stdcsv "encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"strconv"
	"strings"

	"github.com/gocarina/gocsv"
	"github.com/thalesfsp/customerror/v2"
)

var csvUnmarshallerType, textUnmarshallerType = reflect.TypeFor[gocsv.TypeUnmarshaller](), reflect.TypeFor[encoding.TextUnmarshaler]()

type csvField struct {
	keys    []string
	name    string
	typ     reflect.Type
	tagged  bool
	partial bool
}

// Load converts CSV data to a slice of structs. Untagged field names match
// case-insensitively. Exact csv tags win; folded collisions and duplicate exact
// tags are errors.
//
// Other behavior comes from gocsv's streaming Unmarshaller: default= is not
// applied; omitempty preserves nil pointers; duplicate headers assign in order;
// unmatched columns are ignored; and missing columns stay zero. TagName,
// TagSeparator, FieldsCombiner, the header normalizer, FailIfDoubleHeaderNames,
// and ShouldAlignDuplicateHeadersWithStructFieldOrder influence Load. The CSV
// reader factory and FailIfUnmatchedStructTags do not.
// Native numeric syntax and conversion otherwise follow gocsv.
func Load[Out any](in io.Reader) (Out, error) {
	if isNil(in) {
		return *new(Out), customerror.NewRequiredError("CSV reader")
	}
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
	fields, err := inspectCSVFields(rowType, nil, false, nil)
	if err != nil {
		return *new(Out), err
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
	var source bytes.Buffer
	csvReader := stdcsv.NewReader(io.TeeReader(reader, &source))
	prototype := reflect.New(rowType)
	if outType.Elem().Kind() != reflect.Pointer {
		prototype = prototype.Elem()
	}
	unmarshaller, err := gocsv.NewUnmarshaller(csvReader, prototype.Interface())
	if err != nil {
		return *new(Out), err
	}
	headers, err := normalizeCSVHeaders(unmarshaller.Headers, fields)
	if err != nil {
		return *new(Out), err
	}
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
	if err := checkNarrowNumericRanges(source.Bytes(), headers, fields); err != nil {
		return *new(Out), err
	}
	return out, nil
}

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

func inspectCSVFields(rowType reflect.Type, parentKeys []string, parentTagged bool, active map[reflect.Type]struct{}) ([]csvField, error) {
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
		keys, tagged, partial, ignored := csvFieldKeys(field)
		if ignored {
			continue
		}
		if field.Anonymous {
			keys = parentKeys
			tagged = parentTagged
		} else if len(parentKeys) > 0 {
			var combined []string
			for _, parent := range parentKeys {
				for _, key := range keys {
					combined = append(combined, parent+gocsv.FieldsCombiner+key)
				}
			}
			keys = combined
			tagged = parentTagged || tagged
		}
		fieldType := field.Type
		if fieldType.Kind() == reflect.Pointer {
			fieldType = fieldType.Elem()
		}
		if fieldType.Kind() == reflect.Struct && !hasCustomUnmarshaller(fieldType) {
			nested, err := inspectCSVFields(fieldType, keys, tagged, active)
			if err != nil {
				return nil, err
			}
			fields = append(fields, nested...)
			continue
		}
		if (fieldType.Kind() == reflect.Slice || fieldType.Kind() == reflect.Array) &&
			fieldType.Elem().Kind() == reflect.Struct {
			if _, err := inspectCSVFields(fieldType.Elem(), nil, false, active); err != nil {
				return nil, err
			}
		}
		if field.Anonymous {
			continue
		}
		fields = append(fields, csvField{keys, field.Name, field.Type, tagged, partial})
	}
	return fields, nil
}

func csvFieldKeys(field reflect.StructField) ([]string, bool, bool, bool) {
	parts := strings.Split(field.Tag.Get(gocsv.TagName), gocsv.TagSeparator)
	keys := make([]string, 0, len(parts))
	partial := false
	for _, part := range parts {
		part = strings.TrimSpace(part)
		switch {
		case strings.HasPrefix(part, "partial"):
			partial = true
		case part == "omitempty", strings.HasPrefix(part, "default="):
		default:
			keys = append(keys, part)
		}
	}
	if len(keys) == 1 && keys[0] == "-" {
		return nil, false, false, true
	}
	if len(keys) == 0 || keys[0] == "" {
		return []string{field.Name}, false, partial, false
	}
	return keys, true, partial, false
}

func hasCustomUnmarshaller(typ reflect.Type) bool {
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	return typ.Implements(csvUnmarshallerType) ||
		typ.Implements(textUnmarshallerType) ||
		reflect.PointerTo(typ).Implements(csvUnmarshallerType) ||
		reflect.PointerTo(typ).Implements(textUnmarshallerType)
}

func normalizeCSVHeaders(headers []string, fields []csvField) ([]string, error) {
	normalized := append([]string(nil), headers...)
	for index, header := range headers {
		var exact, folded []csvField
		for _, field := range fields {
			for _, key := range field.keys {
				if field.tagged && header == key {
					exact = append(exact, field)
					break
				}
				if !field.tagged && strings.EqualFold(header, key) {
					folded = append(folded, field)
					break
				}
			}
		}
		matches := folded
		if len(exact) > 0 {
			matches = exact
			for _, field := range folded {
				for _, key := range field.keys {
					if header == key {
						matches = append(matches, field)
						break
					}
				}
			}
		}
		if len(matches) > 1 {
			return nil, ambiguousHeaderError(header, matches)
		}
		if len(matches) == 1 {
			for _, key := range matches[0].keys {
				if header == key || strings.EqualFold(header, key) {
					normalized[index] = key
					break
				}
			}
		}
	}
	return normalized, nil
}

func ambiguousHeaderError(header string, fields []csvField) error {
	names := make([]string, len(fields))
	for index := range fields {
		names[index] = fields[index].name
	}
	return fmt.Errorf("ambiguous CSV header %q matches fields %s", header, strings.Join(names, ", "))
}

func checkNarrowNumericRanges(source []byte, headers []string, fields []csvField) error {
	records, err := stdcsv.NewReader(bytes.NewReader(source)).ReadAll()
	if err != nil || len(records) < 2 {
		return err
	}
	headerFields := make([]*csvField, len(headers))
	for column, header := range headers {
		for tagged := true; ; tagged = false {
			for index := range fields {
				if fields[index].tagged == tagged && matchesGocsvHeader(header, fields[index]) {
					headerFields[column] = &fields[index]
					break
				}
			}
			if headerFields[column] != nil || !tagged {
				break
			}
		}
	}
	for rowIndex, record := range records[1:] {
		for column, value := range record {
			if column >= len(headerFields) || headerFields[column] == nil {
				continue
			}
			field := headerFields[column]
			if hasCustomUnmarshaller(field.typ) || !numericRangeError(value, field.typ) {
				continue
			}
			return &stdcsv.ParseError{
				Line:   rowIndex + 2,
				Column: column + 1,
				Err:    fmt.Errorf("value %q for field %s is out of range for %s", value, field.name, field.typ),
			}
		}
	}
	return nil
}

func matchesGocsvHeader(header string, field csvField) bool {
	withoutZeroWidth := strings.Map(func(character rune) rune {
		if strings.ContainsRune("\u200B\uFEFF\u200D\u200C", character) {
			return -1
		}
		return character
	}, header)
	for _, key := range field.keys {
		if header == key || strings.TrimSpace(header) == key ||
			field.partial && strings.Contains(header, key) || withoutZeroWidth == key {
			return true
		}
	}
	return false
}

func numericRangeError(value string, typ reflect.Type) bool {
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	switch typ.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if typ.Bits() == 64 {
			return false
		}
		integer, _, _ := strings.Cut(value, ".")
		_, err := strconv.ParseInt(integer, 0, typ.Bits())
		return errors.Is(err, strconv.ErrRange)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if typ.Bits() == 64 {
			return false
		}
		if strings.Contains(value, ".") {
			number, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return false
			}
			truncated := math.Trunc(number)
			maximum := math.Pow(2, float64(typ.Bits())) - 1
			return truncated < 0 || truncated > maximum
		}
		_, err := strconv.ParseUint(value, 0, typ.Bits())
		return errors.Is(err, strconv.ErrRange)
	case reflect.Float32:
		_, err := strconv.ParseFloat(strings.ReplaceAll(value, ",", "."), 32)
		return errors.Is(err, strconv.ErrRange)
	default:
		return false
	}
}
