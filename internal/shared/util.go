package shared

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/thalesfsp/customerror/v2"
	"github.com/thalesfsp/etler/v4/internal/customapm"
	"github.com/thalesfsp/status"
	"github.com/thalesfsp/sypl/v2"
)

// GenerateIDBasedOnContent returns the lowercase SHA-256 hex digest of ct after
// trimming leading and trailing form feeds, tabs, carriage returns, newlines,
// and spaces. Equal normalized content produces the same ID.
func GenerateIDBasedOnContent(ct string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Trim(ct, "\f\t\r\n "))))
}

// GenerateUUID returns a random RFC 4122 UUID string.
func GenerateUUID() string {
	return uuid.New().String()
}

// Flatten2D concatenates the inner slices in order. It returns nil when data
// contains no elements.
func Flatten2D[T any](data [][]T) []T {
	total := 0
	for _, outer := range data {
		total += len(outer)
	}

	if total == 0 {
		return nil
	}

	result := make([]T, 0, total)
	for _, outer := range data {
		result = append(result, outer...)
	}

	return result
}

// ExtractID returns an ID from an exported field of t. It matches idFieldName
// case-insensitively when provided; otherwise it matches ID case-insensitively.
// Top-level fields take precedence over fields in exported anonymous structs.
//
// NOTE: String, int, int8, int16, int32, int64, uint, uint8, uint16, uint32,
// and uint64 values are supported after pointer and interface dereferencing.
// Nil pointers, whitespace-only strings, uintptr, fmt.Stringer-only values, and
// all other kinds are treated as absent.
func ExtractID[T any](t T, idFieldName string) string {
	return extractID(reflect.ValueOf(t), idFieldName, make(map[extractIDPointer]struct{}))
}

type extractIDPointer struct {
	address uintptr
	typ     reflect.Type
}

func extractID(v reflect.Value, idFieldName string, visited map[extractIDPointer]struct{}) string {
	// Unwrap pointers and interfaces alike: an embedded interface field holds
	// its value indirectly, and the traversal has to reach the concrete struct
	// behind it. Pointers are recorded so a cycle terminates instead of
	// recursing until the stack overflows.
	for v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return ""
		}

		if v.Kind() == reflect.Pointer {
			pointer := extractIDPointer{
				address: v.Pointer(),
				typ:     v.Type(),
			}
			if _, ok := visited[pointer]; ok {
				return ""
			}

			visited[pointer] = struct{}{}
		}

		v = v.Elem()
	}

	// Nil pointers dereference to an invalid value, and only structs have
	// fields — anything else has no ID to extract.
	if !v.IsValid() || v.Kind() != reflect.Struct {
		return ""
	}

	tType := v.Type()

	for i := 0; i < tType.NumField(); i++ {
		field := tType.Field(i)

		// Unexported fields can't be read via Interface() — skip them.
		if !field.IsExported() {
			continue
		}

		if idFieldName != "" {
			if strings.EqualFold(field.Name, idFieldName) {
				return extractIDValueWithVisited(v.Field(i), visited)
			}
		} else if strings.EqualFold(field.Name, "ID") || strings.EqualFold(field.Name, "Id") {
			return extractIDValueWithVisited(v.Field(i), visited)
		}
	}

	for i := 0; i < tType.NumField(); i++ {
		field := tType.Field(i)
		if field.Anonymous && field.IsExported() {
			if id := extractID(v.Field(i), idFieldName, visited); id != "" {
				return id
			}
		}
	}

	return ""
}

func extractIDValue(v reflect.Value) string {
	return extractIDValueWithVisited(v, make(map[extractIDPointer]struct{}))
}

func extractIDValueWithVisited(v reflect.Value, visited map[extractIDPointer]struct{}) string {
	for v.IsValid() && (v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface) {
		if v.IsNil() {
			return ""
		}

		if v.Kind() == reflect.Pointer {
			pointer := extractIDPointer{
				address: v.Pointer(),
				typ:     v.Type(),
			}
			if _, ok := visited[pointer]; ok {
				return ""
			}

			visited[pointer] = struct{}{}
		}

		v = v.Elem()
	}

	if !v.IsValid() || !v.CanInterface() {
		return ""
	}

	switch v.Kind() {
	case reflect.String:
		return nonEmptyID(v.String())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(v.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(v.Uint(), 10)
	default:
		return ""
	}
}

func nonEmptyID(id string) string {
	if strings.TrimSpace(id) == "" {
		return ""
	}

	return id
}

// Unmarshal decodes JSON data into v and wraps decoding errors.
func Unmarshal(data []byte, v any) error {
	if err := json.Unmarshal(data, v); err != nil {
		return customerror.NewFailedToError("to unmarshal",
			customerror.WithError(err),
		)
	}

	return nil
}

// Marshal encodes v as JSON and wraps encoding errors.
func Marshal(v any) ([]byte, error) {
	data, err := json.Marshal(&v)
	if err != nil {
		return nil, customerror.NewFailedToError("to marshal",
			customerror.WithError(err),
		)
	}

	return data, nil
}

// Decode reads one JSON value from r into v and wraps decoding errors.
func Decode(r io.Reader, v any) error {
	if err := json.NewDecoder(r).Decode(v); err != nil {
		return customerror.NewFailedToError("decode",
			customerror.WithError(err),
		)
	}

	return nil
}

// Encode writes v as JSON followed by a newline and wraps encoding errors.
func Encode(w io.Writer, v any) error {
	if err := json.NewEncoder(w).Encode(v); err != nil {
		return customerror.NewFailedToError("encode",
			customerror.WithError(err),
		)
	}

	return nil
}

// ReadAll reads r to EOF and wraps read errors.
func ReadAll(r io.Reader) ([]byte, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, customerror.NewFailedToError("read response body", customerror.WithError(err))
	}

	return b, nil
}

// OnErrorHandler marks an entity failed, records and logs err, and returns the
// wrapped error.
func OnErrorHandler(
	tracedContext context.Context,
	iMetric IMetrics,
	l sypl.ISypl,
	err error,
	message, t, name string,
) error {
	// Observability: update the processor status, logging, metrics.
	iMetric.GetStatus().Set(status.Failed.String())

	return customapm.TraceError(
		tracedContext,
		customerror.NewFailedToError(
			message,
			customerror.WithError(err),
			customerror.WithField(t, name),
		),
		l,
		iMetric.GetCounterFailed(),
	)
}
