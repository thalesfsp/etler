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
	"regexp"
	"strconv"
	"strings"

	"github.com/gocarina/gocsv"
	"github.com/thalesfsp/customerror/v2"
)

var (
	csvUnmarshallerType    = reflect.TypeFor[gocsv.TypeUnmarshaller]()
	textUnmarshallerType   = reflect.TypeFor[encoding.TextUnmarshaler]()
	decimalIntegerPattern  = regexp.MustCompile(`^[+-]?(0|[1-9][0-9]*)$`)
	decimalUnsignedPattern = regexp.MustCompile(`^\+?(0|[1-9][0-9]*)$`)
	decimalFloatPattern    = regexp.MustCompile(`^[+-]?([0-9]+(\.[0-9]*)?|\.[0-9]+)([eE][+-]?[0-9]+)?$`)
)

type csvField struct {
	key   string
	index []int
	typ   reflect.Type
	fold  bool
}

// Load converts CSV data to a slice of structs. Untagged field names match
// case-insensitively through gocsv's per-unmarshaller header normalizer; gocsv
// resolves all field collisions in declaration order. Output types using csv[]
// slice expansion are unsupported.
//
// Numeric fields accept ordinary signed decimal input with surrounding
// whitespace, including decimal fractions and signed exponents for floats, but
// reject digit separators, non-decimal notation, leading zeroes on multi-digit
// integers, fractional integers, and values outside the destination width.
// Other behavior comes from gocsv's Unmarshaller. Its package-level tag,
// header, and duplicate settings influence Load; its reader factory and
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
	records, err := stdcsv.NewReader(bytes.NewReader(source)).ReadAll()
	if err != nil {
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
	if err := validateNumericValues(records, headers, rowType, fields); err != nil {
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
			for index := range nested {
				nested[index].index = append([]int{field.Index[0]}, nested[index].index...)
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
		fields = append(fields, csvField{key: key, index: []int{field.Index[0]}, typ: field.Type, fold: fold})
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

func validateNumericValues(
	records [][]string,
	headers []string,
	rowType reflect.Type,
	fields []csvField,
) error {
	if len(records) < 2 {
		return nil
	}
	headerFields := make([]*csvField, len(headers))
	for column := range headers {
		field, err := gocsvNumericField(headers[:column+1], rowType, fields)
		if err != nil {
			return err
		}
		headerFields[column] = field
	}
	for _, record := range records[1:] {
		for column, value := range record {
			if column >= len(headerFields) || headerFields[column] == nil {
				continue
			}
			field := headerFields[column]
			valueErr := numericValueError(value, field.typ)
			if valueErr == nil {
				continue
			}
			typ := baseType(field.typ)
			if errors.Is(valueErr, strconv.ErrRange) {
				return fmt.Errorf("value %q in column %q is out of range for %s", value, records[0][column], typ)
			}
			return fmt.Errorf("value %q in column %q is not valid decimal input for %s", value, records[0][column], typ)
		}
	}
	return nil
}

func gocsvNumericField(headers []string, rowType reflect.Type, fields []csvField) (*csvField, error) {
	values := make([]string, len(headers))
	values[len(values)-1] = "1"
	var probeSource bytes.Buffer
	writer := stdcsv.NewWriter(&probeSource)
	if err := writer.WriteAll([][]string{headers, values}); err != nil {
		return nil, customerror.NewFailedToError("build the CSV header probe",
			customerror.WithError(err),
		)
	}

	unmarshaller, err := gocsv.NewUnmarshaller(stdcsv.NewReader(&probeSource), reflect.New(rowType).Elem().Interface())
	if err != nil {
		return nil, err
	}
	if err := unmarshaller.RenormalizeHeaders(func([]string) []string {
		return headers
	}); err != nil {
		return nil, err
	}
	row, err := unmarshaller.Read()
	if err != nil {
		return nil, nil
	}
	for index := range fields {
		if !hasCustomUnmarshaller(fields[index].typ) &&
			numericOneAt(reflect.ValueOf(row), fields[index].index) {
			return &fields[index], nil
		}
	}
	return nil, nil
}

func numericOneAt(value reflect.Value, index []int) bool {
	for _, fieldIndex := range index {
		for value.Kind() == reflect.Pointer {
			if value.IsNil() {
				return false
			}
			value = value.Elem()
		}
		if value.Kind() != reflect.Struct {
			return false
		}
		value = value.Field(fieldIndex)
	}
	for value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return false
		}
		value = value.Elem()
	}
	switch value.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return value.Int() == 1
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return value.Uint() == 1
	case reflect.Float32, reflect.Float64:
		return value.Float() == 1
	default:
		return false
	}
}

func baseType(typ reflect.Type) reflect.Type {
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	return typ
}

func numericValueError(value string, typ reflect.Type) error {
	typ = baseType(typ)
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	switch typ.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if !decimalIntegerPattern.MatchString(value) {
			return strconv.ErrSyntax
		}
		_, err := strconv.ParseInt(value, 10, typ.Bits())
		return err
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if !decimalUnsignedPattern.MatchString(value) {
			return strconv.ErrSyntax
		}
		_, err := strconv.ParseUint(value, 10, typ.Bits())
		return err
	case reflect.Float32, reflect.Float64:
		if !decimalFloatPattern.MatchString(value) {
			return strconv.ErrSyntax
		}
		_, err := strconv.ParseFloat(value, typ.Bits())
		return err
	default:
		return nil
	}
}
