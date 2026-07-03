package shared

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type unexportedIDHolder struct {
	id   string //nolint:unused
	Name string
}

type ExportedIDHolder struct {
	ID   string
	Name string
}

type embeddedIDHolder struct {
	ExportedIDHolder
	Extra string
}

type unexportedEmbeddedHolder struct {
	exportedIDHolder //nolint:unused
	Extra            string
}

type exportedIDHolder struct {
	ID string //nolint:unused
}

// Bug: a struct with an UNEXPORTED field whose name matches "id" must not
// panic reflection — it must simply be skipped.
func TestExtractID_unexportedIDField_doesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		got := ExtractID(unexportedIDHolder{id: "secret", Name: "n"}, "")
		assert.Empty(t, got, "unexported fields must be skipped, not extracted")
	})
}

// Happy path: exported ID field, by value and by pointer.
func TestExtractID_exportedIDField(t *testing.T) {
	assert.Equal(t, "abc", ExtractID(ExportedIDHolder{ID: "abc"}, ""))
	assert.Equal(t, "abc", ExtractID(&ExportedIDHolder{ID: "abc"}, ""))
}

// Happy path: custom field name takes precedence.
func TestExtractID_customFieldName(t *testing.T) {
	type withCode struct {
		Code string
		ID   string
	}

	assert.Equal(t, "c1", ExtractID(withCode{Code: "c1", ID: "i1"}, "Code"))
}

// Happy path: ID found on an embedded (anonymous) EXPORTED struct.
func TestExtractID_embeddedID(t *testing.T) {
	v := embeddedIDHolder{
		ExportedIDHolder: ExportedIDHolder{ID: "nested"},
		Extra:            "x",
	}

	assert.Equal(t, "nested", ExtractID(v, ""))
}

// Edge case: an embedded UNEXPORTED type cannot be read via reflection —
// it must be skipped without panicking.
func TestExtractID_unexportedEmbedded_doesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		got := ExtractID(unexportedEmbeddedHolder{Extra: "x"}, "")
		assert.Empty(t, got)
	})
}

// Edge case: no ID anywhere returns empty.
func TestExtractID_noID_returnsEmpty(t *testing.T) {
	type noID struct {
		Name string
	}

	assert.Empty(t, ExtractID(noID{Name: "n"}, ""))
}

// Edge cases: nil pointers and non-struct values must not panic.
func TestExtractID_nilPointerAndNonStruct_doNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		var nilHolder *ExportedIDHolder

		assert.Empty(t, ExtractID(nilHolder, ""))
	})

	assert.NotPanics(t, func() {
		assert.Empty(t, ExtractID(123, ""))
		assert.Empty(t, ExtractID("just-a-string", ""))
		assert.Empty(t, ExtractID([]string{"a"}, ""))
	})
}

// Edge cases for Flatten2D.
func TestFlatten2D_edgeCases(t *testing.T) {
	assert.Nil(t, Flatten2D[int](nil))
	assert.Nil(t, Flatten2D([][]int{}))
	assert.Equal(t, []int{1, 2, 3}, Flatten2D([][]int{{1}, {}, {2, 3}}))
}
