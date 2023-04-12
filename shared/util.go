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

// ExtractID extracts `possibleIDFieldNames` from `v` - an arbitrary struct.
//
// NOTE: Only exported fields are considered.
func ExtractID(v any, possibleIDFieldNames ...string) string {
	var id string

	for _, name := range possibleIDFieldNames {
		if name != "" {
			if reflect.TypeOf(v).Kind() == reflect.Struct {
				v := reflect.ValueOf(v)
				f := v.FieldByName(name)

				if f.IsValid() {
					// Convet to string if it's not.
					if f.Kind() != reflect.String {
						id = fmt.Sprintf("%v", f.Interface())

						break
					}

					id = f.String()

					break
				}
			}
		}
	}

	return id
}
