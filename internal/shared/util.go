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

// GenerateIDBasedOnContent generates a SHA-256 hash (content-based) for message
// ID. Good to be used to avoid duplicated messages.
func GenerateIDBasedOnContent(ct string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Trim(ct, "\f\t\r\n "))))
}

// GenerateUUID generates a RFC4122 UUID and DCE 1.1: Authentication and
// Security Services.
func GenerateUUID() string {
	return uuid.New().String()
}

// Flatten2D takes a 2D slice and returns a 1D slice containing all the elements.
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

// ExtractID extracts `idFieldName` from `v` - an arbitrary struct. When no
// field name is configured, it searches for ID or Id.
//
// NOTE: Only exported fields are considered.
// NOTE: Supported ID values are strings, signed integers, unsigned integers
// except uintptr, and fmt.Stringer implementations. Pointer and interface
// wrappers are dereferenced. Nil pointers, whitespace-only values, and
// unsupported kinds are treated as absent.
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
				return extractIDValue(v.Field(i))
			}
		} else if strings.EqualFold(field.Name, "ID") || strings.EqualFold(field.Name, "Id") {
			return extractIDValue(v.Field(i))
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
	var pointerStringer fmt.Stringer

	for v.IsValid() && (v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface) {
		if v.IsNil() {
			return ""
		}

		if v.Kind() == reflect.Pointer && pointerStringer == nil && v.CanInterface() {
			pointerStringer, _ = v.Interface().(fmt.Stringer)
		}

		v = v.Elem()
	}

	if !v.IsValid() || !v.CanInterface() {
		return ""
	}

	if stringer, ok := v.Interface().(fmt.Stringer); ok {
		return nonEmptyID(stringer.String())
	}

	if pointerStringer != nil {
		return nonEmptyID(pointerStringer.String())
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

// Unmarshal with custom error.
func Unmarshal(data []byte, v any) error {
	if err := json.Unmarshal(data, v); err != nil {
		return customerror.NewFailedToError("to unmarshal",
			customerror.WithError(err),
		)
	}

	return nil
}

// Marshal with custom error.
func Marshal(v any) ([]byte, error) {
	data, err := json.Marshal(&v)
	if err != nil {
		return nil, customerror.NewFailedToError("to marshal",
			customerror.WithError(err),
		)
	}

	return data, nil
}

// Decode process stream `r` into `v` and returns an error if any.
func Decode(r io.Reader, v any) error {
	if err := json.NewDecoder(r).Decode(v); err != nil {
		return customerror.NewFailedToError("decode",
			customerror.WithError(err),
		)
	}

	return nil
}

// Encode process `v` into stream `w` and returns an error if any.
func Encode(w io.Writer, v any) error {
	if err := json.NewEncoder(w).Encode(v); err != nil {
		return customerror.NewFailedToError("encode",
			customerror.WithError(err),
		)
	}

	return nil
}

// ReadAll reads all the data from `r` and returns an error if any.
func ReadAll(r io.Reader) ([]byte, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, customerror.NewFailedToError("read response body", customerror.WithError(err))
	}

	return b, nil
}

// OnErrorHandler deals with observability (update status, logging, metrics)
// when an processor, or stage, or the pipeline error.
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
