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

//////
// Consts, vars and types.
//////

var utf8BOM = []byte{0xef, 0xbb, 0xbf}

var (
	csvMarshallerType         = reflect.TypeOf((*gocsv.TypeMarshaller)(nil)).Elem()
	csvUnmarshallerType       = reflect.TypeOf((*gocsv.TypeUnmarshaller)(nil)).Elem()
	csvFieldsUnmarshallerType = reflect.TypeOf((*gocsv.TypeUnmarshalCSVWithFields)(nil)).Elem()
	textMarshallerType        = reflect.TypeOf((*encoding.TextMarshaler)(nil)).Elem()
	textUnmarshallerType      = reflect.TypeOf((*encoding.TextUnmarshaler)(nil)).Elem()
)

type fieldInfo struct {
	keys         []string
	name         string
	valueType    reflect.Type
	defaultValue string
	tagged       bool
	custom       bool
	partial      bool
}

//////
// Helpers.
//////

// Load converts a CSV file to the target struct slice (Out).
//
// Load uses gocsv's per-unmarshaller header-renormalization hook and a local
// encoding/csv reader. However, gocsv has no per-unmarshaller settings for its
// TagName, TagSeparator, FieldsCombiner, FailIfDoubleHeaderNames,
// ShouldAlignDuplicateHeadersWithStructFieldOrder, or struct-name normalizer.
// Changes to those process-wide settings can still affect decoding.
func Load[Out any](in io.Reader) (Out, error) {
	outType := reflect.TypeOf((*Out)(nil)).Elem()
	if outType.Kind() != reflect.Slice {
		return *new(Out), customerror.NewInvalidError(
			fmt.Sprintf("CSV output type %s, it must be a slice", outType),
		)
	}

	if isNil(in) {
		return *new(Out), customerror.NewRequiredError("CSV reader")
	}

	rowType := outType.Elem()
	if rowType.Kind() == reflect.Ptr {
		rowType = rowType.Elem()
	}

	fields, err := getFieldInfo(rowType, nil, nil)
	if err != nil {
		return *new(Out), err
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

	content, err := io.ReadAll(reader)
	if err != nil {
		return *new(Out), err
	}

	records, err := stdcsv.NewReader(bytes.NewReader(content)).ReadAll()
	if err != nil {
		return *new(Out), err
	}

	headers := normalizeHeaders(records[0], fields)
	if err := validateNumbers(records[1:], headers, fields); err != nil {
		return *new(Out), err
	}

	prototype := reflect.New(rowType)
	if outType.Elem().Kind() != reflect.Ptr {
		prototype = prototype.Elem()
	}

	unmarshaller, err := gocsv.NewUnmarshaller(
		stdcsv.NewReader(bytes.NewReader(content)),
		prototype.Interface(),
	)
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

	return out, nil
}

func isNil(value any) bool {
	if value == nil {
		return true
	}

	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

func getFieldInfo(
	rowType reflect.Type,
	parentKeys []string,
	active map[reflect.Type]struct{},
) ([]fieldInfo, error) {
	if rowType.Kind() != reflect.Struct {
		return nil, nil
	}

	if active == nil {
		active = make(map[reflect.Type]struct{})
	}
	if _, exists := active[rowType]; exists {
		return nil, customerror.NewInvalidError(
			fmt.Sprintf("cyclic CSV output type %s", rowType),
		)
	}

	active[rowType] = struct{}{}
	defer delete(active, rowType)

	fields := make([]fieldInfo, 0, rowType.NumField())

	for index := range rowType.NumField() {
		structField := rowType.Field(index)
		if structField.PkgPath != "" {
			continue
		}

		current, ignored := newFieldInfo(structField, parentKeys)
		if ignored {
			continue
		}

		fieldType := structField.Type
		if fieldType.Kind() == reflect.Ptr {
			fieldType = fieldType.Elem()
		}

		if fieldType.Kind() == reflect.Struct && !isCSVScalar(fieldType) {
			keys := parentKeys
			if current != nil {
				keys = current.keys
			}

			nested, err := getFieldInfo(
				fieldType,
				keys,
				active,
			)
			if err != nil {
				return nil, err
			}

			fields = append(fields, nested...)

			continue
		}

		originalType := structField.Type
		if (originalType.Kind() == reflect.Slice || originalType.Kind() == reflect.Array) &&
			originalType.Elem().Kind() == reflect.Struct {
			if _, err := getFieldInfo(originalType.Elem(), nil, active); err != nil {
				return nil, err
			}
		}

		if current != nil {
			current.valueType = fieldType
			current.custom = hasCustomUnmarshaller(fieldType)
			fields = append(fields, *current)
		}
	}

	return fields, nil
}

func newFieldInfo(structField reflect.StructField, parentKeys []string) (*fieldInfo, bool) {
	if structField.Anonymous {
		return nil, false
	}

	tagParts := strings.Split(structField.Tag.Get("csv"), ",")
	keys := make([]string, 0, len(tagParts))
	defaultValue := ""
	partial := false

	for _, part := range tagParts {
		part = strings.TrimSpace(part)
		switch {
		case part == "omitempty":
		case strings.HasPrefix(part, "partial"):
			partial = true
		case strings.HasPrefix(part, "default="):
			defaultValue = strings.TrimPrefix(part, "default=")
		default:
			keys = append(keys, part)
		}
	}

	if len(keys) == 1 && keys[0] == "-" {
		return nil, true
	}

	tagged := len(keys) > 0 && keys[0] != ""
	if !tagged {
		keys = []string{structField.Name}
	}

	if len(parentKeys) > 0 {
		nestedKeys := make([]string, 0, len(parentKeys)*len(keys))
		for _, parentKey := range parentKeys {
			for _, key := range keys {
				nestedKeys = append(nestedKeys, parentKey+"."+key)
			}
		}
		keys = nestedKeys
		tagged = true
	}

	return &fieldInfo{
		keys:         keys,
		name:         structField.Name,
		defaultValue: defaultValue,
		tagged:       tagged,
		partial:      partial,
	}, false
}

func isCSVScalar(fieldType reflect.Type) bool {
	interfaces := []reflect.Type{
		csvMarshallerType,
		csvUnmarshallerType,
		csvFieldsUnmarshallerType,
		textMarshallerType,
		textUnmarshallerType,
	}

	for _, interfaceType := range interfaces {
		if fieldType.Implements(interfaceType) || reflect.PointerTo(fieldType).Implements(interfaceType) {
			return true
		}
	}

	return false
}

func hasCustomUnmarshaller(fieldType reflect.Type) bool {
	return fieldType.Implements(csvUnmarshallerType) ||
		fieldType.Implements(textUnmarshallerType) ||
		reflect.PointerTo(fieldType).Implements(csvUnmarshallerType) ||
		reflect.PointerTo(fieldType).Implements(textUnmarshallerType)
}

func normalizeHeaders(headers []string, fields []fieldInfo) []string {
	normalized := append([]string(nil), headers...)

	for index, header := range normalized {
		if key, found := exactTaggedKey(header, fields); found {
			normalized[index] = key

			continue
		}

		for _, field := range fields {
			if field.tagged {
				continue
			}

			for _, key := range field.keys {
				if strings.EqualFold(header, key) {
					normalized[index] = key

					break
				}
			}
		}
	}

	return normalized
}

func exactTaggedKey(header string, fields []fieldInfo) (string, bool) {
	for _, field := range fields {
		if !field.tagged {
			continue
		}

		for _, key := range field.keys {
			if header == key {
				return key, true
			}
		}
	}

	return "", false
}

func validateNumbers(records [][]string, headers []string, fields []fieldInfo) error {
	for rowIndex, record := range records {
		for columnIndex, value := range record {
			field, found := matchingField(headers[columnIndex], fields)
			if !found || field.custom || !isNumber(field.valueType.Kind()) {
				continue
			}

			if value == "" {
				value = field.defaultValue
			}

			if err := validateNumber(value, field.valueType); err != nil {
				message := fmt.Sprintf(
					"field %s value %q",
					field.name,
					value,
				)
				if errors.Is(err, strconv.ErrRange) {
					message += fmt.Sprintf(" overflows %s", field.valueType)
				}

				return &stdcsv.ParseError{
					Line:   rowIndex + 2,
					Column: columnIndex + 1,
					Err:    fmt.Errorf("%s: %w", message, err),
				}
			}
		}
	}

	return nil
}

func matchingField(header string, fields []fieldInfo) (fieldInfo, bool) {
	for _, field := range fields {
		for _, key := range field.keys {
			if header == key ||
				strings.TrimSpace(header) == key ||
				(field.partial && strings.Contains(header, key)) ||
				removeZeroWidthCharacters(header) == key {
				return field, true
			}
		}
	}

	return fieldInfo{}, false
}

func removeZeroWidthCharacters(value string) string {
	return strings.Map(func(character rune) rune {
		switch character {
		case '\u200B', '\uFEFF', '\u200D', '\u200C':
			return -1
		default:
			return character
		}
	}, value)
}

func isNumber(kind reflect.Kind) bool {
	switch kind {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	default:
		return false
	}
}

func validateNumber(value string, valueType reflect.Type) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}

	switch valueType.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		value = strings.SplitN(value, ".", 2)[0]
		_, err := strconv.ParseInt(value, 0, valueType.Bits())

		return err
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if strings.Contains(value, ".") {
			number, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return err
			}
			if math.IsNaN(number) || math.IsInf(number, 0) || number < 0 ||
				number >= math.Ldexp(1, valueType.Bits()) {
				return strconv.ErrRange
			}

			return nil
		}

		_, err := strconv.ParseUint(value, 0, valueType.Bits())

		return err
	case reflect.Float32, reflect.Float64:
		_, err := strconv.ParseFloat(
			strings.ReplaceAll(value, ",", "."),
			valueType.Bits(),
		)

		return err
	}

	return nil
}
