package task

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Happy path: a task gets an ID, a creation date, and carries the data.
func TestTask_new_happyPath(t *testing.T) {
	tsk, err := New[int, string]([]int{1, 2, 3})
	require.NoError(t, err)

	assert.NotEmpty(t, tsk.ID)
	assert.NotEmpty(t, tsk.CreatedAt)
	assert.Equal(t, []int{1, 2, 3}, tsk.ProcessingData)
	assert.NotNil(t, tsk.ConvertedData)
	assert.Empty(t, tsk.ConvertedData)
}

// Bad path: a task with NIL data is rejected by validation. An empty,
// non-nil slice is accepted (an empty run is valid).
func TestTask_new_nilData_returnsError(t *testing.T) {
	_, err := New[int, string](nil)
	assert.Error(t, err)

	_, err = New[int, string]([]int{})
	assert.NoError(t, err)
}

// MustNew panics on invalid input.
func TestTask_mustNew_panicsOnEmptyData(t *testing.T) {
	assert.Panics(t, func() {
		MustNew[int, string](nil)
	})

	assert.NotPanics(t, func() {
		MustNew[int, string]([]int{1})
	})
}

// Bug: the task entity type constant must identify a task, not a stage
// (copy-paste error).
func TestTask_typeConstant(t *testing.T) {
	assert.Equal(t, "task", Type)
	assert.Equal(t, "task", Name)
}
