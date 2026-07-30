package shared

import (
	"reflect"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
)

func TestExtractIDValue_dereferencesPointersAndInterfaces(t *testing.T) {
	value := 42
	pointer := &value
	var nilPointer *int

	assert.Equal(t, "42", extractIDValue(reflect.ValueOf(&pointer)))
	assert.Empty(t, extractIDValue(reflect.ValueOf(nilPointer)))

	interfaceHolder := struct {
		Value any
	}{Value: uint64(84)}
	assert.Equal(t, "84", extractIDValue(reflect.ValueOf(interfaceHolder).Field(0)))

	nilInterfaceHolder := struct {
		Value any
	}{}
	assert.Empty(t, extractIDValue(reflect.ValueOf(nilInterfaceHolder).Field(0)))
}

func TestExtractIDValue_rejectsUnsupportedKinds(t *testing.T) {
	number := 1
	values := []struct {
		name  string
		value reflect.Value
	}{
		{name: "invalid", value: reflect.Value{}},
		{name: "bool", value: reflect.ValueOf(true)},
		{name: "float32", value: reflect.ValueOf(float32(1))},
		{name: "float64", value: reflect.ValueOf(float64(1))},
		{name: "complex64", value: reflect.ValueOf(complex64(1))},
		{name: "complex128", value: reflect.ValueOf(complex128(1))},
		{name: "uintptr", value: reflect.ValueOf(uintptr(1))},
		{name: "array", value: reflect.ValueOf([1]int{1})},
		{name: "channel", value: reflect.ValueOf(make(chan int))},
		{name: "function", value: reflect.ValueOf(func() {})},
		{name: "map", value: reflect.ValueOf(map[string]int{"one": 1})},
		{name: "slice", value: reflect.ValueOf([]int{1})},
		{name: "struct", value: reflect.ValueOf(struct{ Value int }{Value: 1})},
		{name: "unsafe pointer", value: reflect.ValueOf(unsafe.Pointer(&number))},
		{
			name:  "non-interfaceable",
			value: reflect.ValueOf(struct{ value int }{value: 1}).Field(0),
		},
	}

	for _, tt := range values {
		t.Run(tt.name, func(t *testing.T) {
			assert.Empty(t, extractIDValue(tt.value))
		})
	}
}
