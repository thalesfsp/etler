package csvshape

import (
	"encoding"
	"fmt"
	"reflect"
	"strconv"
	"sync"

	"github.com/gocarina/gocsv"
	"github.com/thalesfsp/customerror/v2"
)

//////
// Consts, vars and types.
//////

var (
	csvMarshallerType             = reflect.TypeFor[gocsv.TypeMarshaller]()
	textMarshallerType            = reflect.TypeFor[encoding.TextMarshaler]()
	csvUnmarshallerType           = reflect.TypeFor[gocsv.TypeUnmarshaller]()
	csvWithFieldsUnmarshallerType = reflect.TypeFor[gocsv.TypeUnmarshalCSVWithFields]()
	csvOutputTypeValidationCache  sync.Map
)

type csvOutputTypeValidationCacheKey struct {
	rowType           reflect.Type
	tagName           string
	validateExpansion bool
}

type csvOutputTypeValidationCacheValue struct {
	err error
}

//////
// Validators.
//////

// ValidateOutput rejects cyclic row types and lossy csv[] expansion forms.
func ValidateOutput(rowType reflect.Type) error {
	return validate(rowType, nil, gocsv.TagName, true)
}

// ValidateCycle rejects cyclic row types without checking csv[] expansions.
func ValidateCycle(rowType reflect.Type) error {
	return validate(rowType, nil, gocsv.TagName, false)
}

//////
// Helpers.
//////

func validate(
	rowType reflect.Type,
	active map[reflect.Type]struct{},
	tagName string,
	validateExpansion bool,
) (err error) {
	if rowType.Kind() != reflect.Struct {
		return nil
	}
	if active == nil {
		active = make(map[reflect.Type]struct{})
	}
	if _, exists := active[rowType]; exists {
		return customerror.NewInvalidError(fmt.Sprintf("cyclic CSV output type %s", rowType))
	}

	cacheKey := csvOutputTypeValidationCacheKey{
		rowType:           rowType,
		tagName:           tagName,
		validateExpansion: validateExpansion,
	}
	if cached, exists := csvOutputTypeValidationCache.Load(cacheKey); exists {
		return cached.(csvOutputTypeValidationCacheValue).err
	}

	active[rowType] = struct{}{}
	defer delete(active, rowType)
	defer func() {
		csvOutputTypeValidationCache.Store(cacheKey, csvOutputTypeValidationCacheValue{err: err})
	}()

	for index := range rowType.NumField() {
		field := rowType.Field(index)
		if field.PkgPath != "" {
			continue
		}
		if validateExpansion {
			// This deliberately follows every pointer level even though gocsv's
			// traversal follows one: accepting an ignored csv[] tag loses columns.
			expansionType := field.Type
			for expansionType.Kind() == reflect.Pointer {
				expansionType = expansionType.Elem()
			}
			if expansionCount, expands := field.Tag.Lookup(tagName + "[]"); expands &&
				(expansionType.Kind() == reflect.Slice || expansionType.Kind() == reflect.Array) {
				if field.Type.Kind() != reflect.Slice && field.Type.Kind() != reflect.Array {
					return customerror.NewInvalidError(fmt.Sprintf(
						"CSV output type %s uses unsupported csv[] slice expansion on field %s: pointer-wrapped slice or array fields are ignored by gocsv; move the tag to a direct slice or array field",
						rowType,
						field.Name,
					))
				}
				count, err := strconv.Atoi(expansionCount)
				if err != nil || count <= 0 {
					return customerror.NewInvalidError(fmt.Sprintf(
						"CSV output type %s uses unsupported csv[] slice expansion on field %s: count %q is non-numeric or non-positive; use a positive integer count",
						rowType,
						field.Name,
						expansionCount,
					))
				}
				if field.Type.Kind() == reflect.Array && count > field.Type.Len() {
					return customerror.NewInvalidError(fmt.Sprintf(
						"CSV output type %s uses unsupported csv[] slice expansion on field %s: count %d exceeds array length %d; use a count no greater than the array length",
						rowType,
						field.Name,
						count,
						field.Type.Len(),
					))
				}
			}
		}

		structType := field.Type
		if structType.Kind() == reflect.Pointer {
			structType = structType.Elem()
		}
		if structType.Kind() == reflect.Struct && !gocsvCanMarshal(structType) {
			if err := validate(structType, active, tagName, validateExpansion); err != nil {
				return err
			}
			continue
		}
		if !field.Anonymous &&
			(field.Type.Kind() == reflect.Slice || field.Type.Kind() == reflect.Array) &&
			field.Type.Elem().Kind() == reflect.Struct {
			if err := validate(field.Type.Elem(), active, tagName, validateExpansion); err != nil {
				return err
			}
		}
	}
	return nil
}

func gocsvCanMarshal(typ reflect.Type) bool {
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
