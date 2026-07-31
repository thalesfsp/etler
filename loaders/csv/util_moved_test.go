package csv

import (
	"encoding"
	"reflect"
	"sync"

	"github.com/gocarina/gocsv"
	"github.com/thalesfsp/etler/v4/internal/csvshape"
)

//////
// Moved validator compatibility.
//////

// These test-only names preserve the existing loader white-box tests after the
// production validator moved to internal/csvshape. Loader production calls the
// shared package directly.
var csvOutputTypeValidationCache sync.Map

type csvOutputTypeValidationCacheKey struct {
	rowType reflect.Type
	tagName string
}

type csvOutputTypeValidationCacheValue struct {
	err error
}

func init() {
	validateCSVOutputType = validateCSVOutputTypeForMovedTests
}

func validateCSVOutputTypeForMovedTests(rowType reflect.Type) error {
	cacheKey := csvOutputTypeValidationCacheKey{rowType: rowType, tagName: gocsv.TagName}
	if cached, exists := csvOutputTypeValidationCache.Load(cacheKey); exists {
		return cached.(csvOutputTypeValidationCacheValue).err
	}

	err := csvshape.ValidateOutput(rowType)
	csvOutputTypeValidationCache.Store(cacheKey, csvOutputTypeValidationCacheValue{err: err})

	return err
}

func gocsvCanMarshal(typ reflect.Type) bool {
	csvMarshallerType := reflect.TypeFor[gocsv.TypeMarshaller]()
	textMarshallerType := reflect.TypeFor[encoding.TextMarshaler]()
	csvUnmarshallerType := reflect.TypeFor[gocsv.TypeUnmarshaller]()
	csvWithFieldsUnmarshallerType := reflect.TypeFor[gocsv.TypeUnmarshalCSVWithFields]()
	if typ.Implements(csvMarshallerType) ||
		typ.Implements(textMarshallerType) ||
		typ.Implements(csvUnmarshallerType) ||
		typ.Implements(csvWithFieldsUnmarshallerType) {
		return true
	}

	pointerType := reflect.PointerTo(typ)
	return pointerType.Implements(csvMarshallerType) ||
		pointerType.Implements(textMarshallerType) ||
		pointerType.Implements(csvUnmarshallerType) ||
		pointerType.Implements(csvWithFieldsUnmarshallerType)
}
