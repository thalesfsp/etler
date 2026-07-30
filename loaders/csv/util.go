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
	index        []int
	fieldType    reflect.Type
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
// Header matching gives precedence to an exact, case-sensitive csv tag.
// Otherwise, untagged field names match case-insensitively; if more than one
// untagged field matches, Load returns an ambiguity error naming the header and
// competing fields.
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
	if rowType.Kind() == reflect.Pointer {
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

	headers, headerFields, err := normalizeHeaders(records[0], fields)
	if err != nil {
		return *new(Out), err
	}
	if err := validateNumbers(records[1:], headerFields, fields); err != nil {
		return *new(Out), err
	}
	normalizeIntegerRecords(records[1:], headerFields, fields)
	decoderContent := encodeRecords(records)

	decoderHeaders, manualFields := decoderHeaderAssignments(headers, headerFields, fields)

	prototype := reflect.New(rowType)
	if outType.Elem().Kind() != reflect.Pointer {
		prototype = prototype.Elem()
	}

	unmarshaller, err := gocsv.NewUnmarshaller(
		stdcsv.NewReader(bytes.NewReader(decoderContent)),
		prototype.Interface(),
	)
	if err != nil {
		return *new(Out), err
	}

	if err := unmarshaller.RenormalizeHeaders(func([]string) []string {
		return decoderHeaders
	}); err != nil {
		return *new(Out), err
	}

	var out Out
	outValue := reflect.ValueOf(&out).Elem()
	rowIndex := 0

	for {
		row, err := unmarshaller.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return *new(Out), err
		}

		row, err = assignManualFields(
			row,
			records[rowIndex+1],
			headerFields,
			manualFields,
			fields,
		)
		if err != nil {
			return *new(Out), err
		}

		outValue.Set(reflect.Append(outValue, reflect.ValueOf(row)))
		rowIndex++
	}

	return out, nil
}

func encodeRecords(records [][]string) []byte {
	var content bytes.Buffer
	writer := stdcsv.NewWriter(&content)
	writer.WriteAll(records)

	return content.Bytes()
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
		if fieldType.Kind() == reflect.Pointer {
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
			for nestedIndex := range nested {
				nested[nestedIndex].index = append(
					[]int{index},
					nested[nestedIndex].index...,
				)
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
			current.index = []int{index}
			current.fieldType = structField.Type
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

func normalizeHeaders(
	headers []string,
	fields []fieldInfo,
) ([]string, []int, error) {
	normalized := append([]string(nil), headers...)
	headerFields := make([]int, len(headers))

	for index, header := range normalized {
		headerFields[index] = -1

		if fieldIndex, key, found := exactTaggedField(header, fields); found {
			normalized[index] = key
			headerFields[index] = fieldIndex

			continue
		}

		candidates := make([]int, 0, 1)
		for fieldIndex, field := range fields {
			if field.tagged {
				continue
			}

			for _, key := range field.keys {
				if strings.EqualFold(header, key) {
					candidates = append(candidates, fieldIndex)

					break
				}
			}
		}

		if len(candidates) > 1 {
			names := make([]string, len(candidates))
			for candidateIndex, fieldIndex := range candidates {
				names[candidateIndex] = fields[fieldIndex].name
			}

			return nil, nil, fmt.Errorf(
				"ambiguous CSV header %q matches fields %s",
				header,
				strings.Join(names, ", "),
			)
		}
		if len(candidates) == 1 {
			fieldIndex := candidates[0]
			normalized[index] = fields[fieldIndex].keys[0]
			headerFields[index] = fieldIndex
		} else if fieldIndex, found := matchingFieldIndex(header, fields); found {
			headerFields[index] = fieldIndex
		}
	}

	return normalized, headerFields, nil
}

func exactTaggedField(header string, fields []fieldInfo) (int, string, bool) {
	for fieldIndex, field := range fields {
		if !field.tagged {
			continue
		}

		for _, key := range field.keys {
			if header == key {
				return fieldIndex, key, true
			}
		}
	}

	return -1, "", false
}

func decoderHeaderAssignments(
	headers []string,
	headerFields []int,
	fields []fieldInfo,
) ([]string, map[int]struct{}) {
	manualFields := make(map[int]struct{})
	for columnIndex, fieldIndex := range headerFields {
		if fieldIndex == -1 {
			continue
		}

		matchedIndex, found := matchingFieldIndex(headers[columnIndex], fields)
		if !found || matchedIndex != fieldIndex {
			manualFields[fieldIndex] = struct{}{}
		}
	}

	decoderHeaders := append([]string(nil), headers...)
	for columnIndex, fieldIndex := range headerFields {
		if _, manual := manualFields[fieldIndex]; !manual {
			continue
		}

		header := fmt.Sprintf("\x00etler-unmatched-%d", columnIndex)
		for {
			if _, found := matchingFieldIndex(header, fields); !found {
				break
			}
			header += "\x00"
		}
		decoderHeaders[columnIndex] = header
	}

	return decoderHeaders, manualFields
}

func assignManualFields(
	row any,
	record []string,
	headerFields []int,
	manualFields map[int]struct{},
	fields []fieldInfo,
) (any, error) {
	if len(manualFields) == 0 {
		return row, nil
	}

	rowValue := reflect.ValueOf(row)
	isPointer := rowValue.Kind() == reflect.Pointer
	if isPointer {
		rowValue = rowValue.Elem()
	} else {
		copyValue := reflect.New(rowValue.Type()).Elem()
		copyValue.Set(rowValue)
		rowValue = copyValue
	}

	for columnIndex, fieldIndex := range headerFields {
		if _, manual := manualFields[fieldIndex]; !manual {
			continue
		}

		value := record[columnIndex]
		if value == "" {
			value = fields[fieldIndex].defaultValue
		}

		decoded, err := decodeFieldValue(value, fields[fieldIndex].fieldType)
		if err != nil {
			return nil, err
		}
		setFieldByIndex(rowValue, fields[fieldIndex].index, decoded)
	}

	if isPointer {
		return row, nil
	}

	return rowValue.Interface(), nil
}

func decodeFieldValue(value string, fieldType reflect.Type) (reflect.Value, error) {
	proxyType := reflect.StructOf([]reflect.StructField{{
		Name: "Value",
		Type: fieldType,
	}})
	prototype := reflect.New(proxyType).Elem()

	unmarshaller, err := gocsv.NewUnmarshaller(
		stdcsv.NewReader(bytes.NewReader(encodeRecords([][]string{
			{"Value"},
			{value},
		}))),
		prototype.Interface(),
	)
	if err != nil {
		return reflect.Value{}, err
	}
	decoded, err := unmarshaller.Read()
	if err != nil {
		return reflect.Value{}, err
	}

	return reflect.ValueOf(decoded).Field(0), nil
}

func setFieldByIndex(row reflect.Value, index []int, value reflect.Value) {
	field := row
	for _, fieldIndex := range index {
		if field.Kind() == reflect.Pointer {
			if field.IsNil() {
				field.Set(reflect.New(field.Type().Elem()))
			}
			field = field.Elem()
		}
		field = field.Field(fieldIndex)
	}
	field.Set(value)
}

func validateNumbers(records [][]string, headerFields []int, fields []fieldInfo) error {
	for rowIndex, record := range records {
		for columnIndex, value := range record {
			fieldIndex := headerFields[columnIndex]
			if fieldIndex == -1 {
				continue
			}

			field := fields[fieldIndex]
			if field.custom || !isNumber(field.valueType.Kind()) {
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

func matchingFieldIndex(header string, fields []fieldInfo) (int, bool) {
	for fieldIndex, field := range fields {
		for _, key := range field.keys {
			if header == key ||
				strings.TrimSpace(header) == key ||
				(field.partial && strings.Contains(header, key)) ||
				removeZeroWidthCharacters(header) == key {
				return fieldIndex, true
			}
		}
	}

	return -1, false
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
		integer, err := integerDecimalValue(value, false)
		if err != nil {
			return err
		}
		_, err = strconv.ParseInt(integer, 0, valueType.Bits())

		return err
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		integer, err := integerDecimalValue(value, true)
		if err != nil {
			return err
		}
		_, err = strconv.ParseUint(integer, 0, valueType.Bits())

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

func integerDecimalValue(value string, unsigned bool) (string, error) {
	integer, fraction, decimal := strings.Cut(value, ".")
	if decimal {
		for _, character := range fraction {
			if character < '0' || character > '9' {
				return "", strconv.ErrSyntax
			}
		}
	}

	if unsigned && strings.HasPrefix(integer, "-") {
		if strings.Trim(integer[1:], "0") != "" ||
			(decimal && strings.Trim(fraction, "0") != "") {
			return "", strconv.ErrRange
		}

		return "0", nil
	}

	return integer, nil
}

func normalizeIntegerRecords(records [][]string, headerFields []int, fields []fieldInfo) {
	for _, record := range records {
		for columnIndex, value := range record {
			fieldIndex := headerFields[columnIndex]
			if fieldIndex == -1 {
				continue
			}

			field := fields[fieldIndex]
			if field.custom {
				continue
			}
			if value == "" {
				continue
			}

			kind := field.valueType.Kind()
			switch kind {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				record[columnIndex], _ = integerDecimalValue(strings.TrimSpace(value), false)
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				record[columnIndex], _ = integerDecimalValue(strings.TrimSpace(value), true)
			}
		}
	}
}
