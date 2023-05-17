package shared

import (
	"crypto/sha1"
	"fmt"
	"reflect"
	"strings"

	"github.com/google/uuid"
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
