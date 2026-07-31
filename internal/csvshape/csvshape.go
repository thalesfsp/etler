package csvshape

import (
	"encoding"
	"errors"
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
	errCyclicCSVOutputType        = errors.New("cyclic CSV output type")
)

type csvOutputTypeValidationCacheKey struct {
	rowType reflect.Type
	tagName string
}

type csvOutputTypeValidationCacheValue struct {
	err error
}

type cyclicCSVOutputTraversalError struct {
	offendingType reflect.Type
}

func (e *cyclicCSVOutputTraversalError) Error() string {
	return errCyclicCSVOutputType.Error()
}

func (e *cyclicCSVOutputTraversalError) Unwrap() error {
	return errCyclicCSVOutputType
}

type cyclicCSVOutputValidationError struct {
	requestedType reflect.Type
	offendingType reflect.Type
	invalid       error
}

func (e *cyclicCSVOutputValidationError) message() string {
	message := fmt.Sprintf("cyclic CSV output type %s", e.requestedType)
	if e.requestedType != e.offendingType {
		message += fmt.Sprintf(": cycle detected at %s", e.offendingType)
	}

	return message
}

func (e *cyclicCSVOutputValidationError) Error() string {
	return e.invalid.Error()
}

func (e *cyclicCSVOutputValidationError) Unwrap() []error {
	return []error{e.invalid, errCyclicCSVOutputType}
}

//////
// Validators.
//////

// ValidateOutput rejects cyclic row types and lossy csv[] expansion forms.
func ValidateOutput(rowType reflect.Type) error {
	err := validate(rowType, nil, gocsv.TagName)
	var cycleErr *cyclicCSVOutputTraversalError
	if errors.As(err, &cycleErr) {
		return newCyclicCSVOutputValidationError(rowType, cycleErr.offendingType)
	}

	return err
}

//////
// Helpers.
//////

func validate(
	rowType reflect.Type,
	active map[reflect.Type]struct{},
	tagName string,
) (err error) {
	if rowType.Kind() != reflect.Struct {
		return nil
	}
	if active == nil {
		active = make(map[reflect.Type]struct{})
	}
	if _, exists := active[rowType]; exists {
		return &cyclicCSVOutputTraversalError{offendingType: rowType}
	}

	cacheKey := csvOutputTypeValidationCacheKey{
		rowType: rowType,
		tagName: tagName,
	}
	if cached, exists := csvOutputTypeValidationCache.Load(cacheKey); exists {
		cachedErr := cached.(csvOutputTypeValidationCacheValue).err
		if !errors.Is(cachedErr, errCyclicCSVOutputType) {
			return cachedErr
		}
	}

	active[rowType] = struct{}{}
	defer delete(active, rowType)
	defer func() {
		cachedErr := err
		if errors.Is(err, errCyclicCSVOutputType) {
			cachedErr = errCyclicCSVOutputType
		}
		csvOutputTypeValidationCache.Store(cacheKey, csvOutputTypeValidationCacheValue{err: cachedErr})
	}()

	for index := range rowType.NumField() {
		field := rowType.Field(index)
		if field.PkgPath != "" {
			continue
		}
		if expansionCount, expands := field.Tag.Lookup(tagName + "[]"); expands {
			// This deliberately follows every pointer level even though gocsv's
			// traversal follows one: accepting an ignored csv[] tag loses columns.
			expansionType := field.Type
			visitedExpansionTypes := make(map[reflect.Type]struct{})
			for expansionType.Kind() == reflect.Pointer {
				if _, exists := visitedExpansionTypes[expansionType]; exists {
					break
				}
				visitedExpansionTypes[expansionType] = struct{}{}
				expansionType = expansionType.Elem()
			}
			if expansionType.Kind() == reflect.Slice || expansionType.Kind() == reflect.Array {
				if field.Anonymous {
					return customerror.NewInvalidError(fmt.Sprintf(
						"CSV output type %s uses unsupported csv[] slice expansion on anonymous field %s: gocsv drops anonymous non-struct fields entirely, so the tag cannot take effect; move the tag to a named field",
						rowType,
						field.Name,
					))
				}
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
			if err := validate(structType, active, tagName); err != nil {
				return err
			}
			continue
		}
		if !field.Anonymous &&
			(field.Type.Kind() == reflect.Slice || field.Type.Kind() == reflect.Array) &&
			field.Type.Elem().Kind() == reflect.Struct {
			if err := validate(field.Type.Elem(), active, tagName); err != nil {
				return err
			}
		}
	}
	return nil
}

func newCyclicCSVOutputValidationError(
	requestedType reflect.Type,
	offendingType reflect.Type,
) error {
	validationErr := &cyclicCSVOutputValidationError{
		requestedType: requestedType,
		offendingType: offendingType,
	}
	validationErr.invalid = customerror.NewInvalidError(validationErr.message())

	return validationErr
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
