package shared

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/google/uuid"
	"github.com/thalesfsp/customerror"
	"github.com/thalesfsp/status"
	"github.com/thalesfsp/sypl"

	"github.com/thalesfsp/etler/v2/internal/customapm"
)

// GenerateIDBasedOnContent generates MD5 hash (content-based) for message ID. Good to be used
// to avoid duplicated messages.
func GenerateIDBasedOnContent(ct string) string {
	return fmt.Sprintf("%x", sha1.Sum([]byte(strings.Trim(ct, "\f\t\r\n "))))
}

// GenerateUUID generates a RFC4122 UUID and DCE 1.1: Authentication and
// Security Services.
func GenerateUUID() string {
	return uuid.New().String()
}

// Flatten2D takes a 2D slice and returns a 1D slice containing all the elements.
func Flatten2D[T any](data [][]T) []T {
	var result []T

	for _, outer := range data {
		result = append(result, outer...)
	}

	return result
}

// ExtractID extracts `possibleIDFieldNames` from `v` - an arbitrary struct.
//
// NOTE: Only exported fields are considered.
func ExtractID[T any](t T, idFieldName string) string {
	v := reflect.ValueOf(t)
	for v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	tType := v.Type()

	for i := 0; i < tType.NumField(); i++ {
		field := tType.Field(i)

		if idFieldName != "" && strings.EqualFold(field.Name, idFieldName) {
			return fmt.Sprintf("%v", v.Field(i).Interface())
		}

		if strings.EqualFold(field.Name, "ID") || strings.EqualFold(field.Name, "Id") {
			return fmt.Sprintf("%v", v.Field(i).Interface())
		}
	}

	for i := 0; i < tType.NumField(); i++ {
		field := tType.Field(i)
		if field.Anonymous {
			if id := ExtractID(v.Field(i).Interface(), idFieldName); id != "" {
				return id
			}
		}
	}

	return ""
}

// Unmarshal with custom error.
func Unmarshal(data []byte, v any) error {
	if err := json.Unmarshal(data, &v); err != nil {
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
