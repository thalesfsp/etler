package shared

import (
	"fmt"
	"os"
	"os/exec"
	"runtime/debug"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const extractIDCycleChildCase = "ETLER_EXTRACT_ID_CYCLE_CHILD_CASE"

// Bug: cyclic exported embedded pointers must terminate without losing an ID
// reachable through another embedded field.
func TestExtractID_cyclicEmbeddedPointers(t *testing.T) {
	if childCase := os.Getenv(extractIDCycleChildCase); childCase != "" {
		debug.SetMaxStack(1 << 20)
		assertExtractIDCycleCase(t, childCase)

		return
	}

	for _, childCase := range []string{"self", "with-id", "mutual"} {
		t.Run(childCase, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestExtractID_cyclicEmbeddedPointers$")
			cmd.Env = append(os.Environ(),
				extractIDCycleChildCase+"="+childCase,
				"GOTRACEBACK=none",
			)

			err := cmd.Run()
			require.NoError(t, err, "cyclic ExtractID subprocess crashed")

			assertExtractIDCycleCase(t, childCase)
		})
	}
}

func assertExtractIDCycleCase(t *testing.T, childCase string) {
	t.Helper()

	switch childCase {
	case "self":
		type Node struct {
			*Node
			Name string
		}

		node := &Node{Name: "root"}
		node.Node = node

		assert.Empty(t, ExtractID(node, ""))
	case "with-id":
		type Node struct {
			*Node
			ExportedIDHolder
		}

		node := &Node{
			ExportedIDHolder: ExportedIDHolder{ID: "nested"},
		}
		node.Node = node

		assert.Equal(t, "nested", ExtractID(node, ""))
	case "mutual":
		type Node struct {
			*Node
			Name string
		}

		first := &Node{Name: "first"}
		second := &Node{Name: "second"}
		first.Node = second
		second.Node = first

		assert.Empty(t, ExtractID(first, ""))
	default:
		t.Fatalf("unknown cycle case %q", childCase)
	}
}

// Happy and bad paths: Unmarshal populates a pointer destination and reports
// malformed JSON or a non-pointer destination.
func TestUnmarshal_destinationValidation(t *testing.T) {
	t.Run("pointer", func(t *testing.T) {
		var value int

		require.NoError(t, Unmarshal([]byte("2"), &value))
		assert.Equal(t, 2, value)
	})

	t.Run("non-pointer", func(t *testing.T) {
		value := 1

		assert.Error(t, Unmarshal([]byte("2"), value))
		assert.Equal(t, 1, value)
	})

	t.Run("malformed JSON", func(t *testing.T) {
		var value int

		assert.Error(t, Unmarshal([]byte("{invalid"), &value))
	})
}

// Edge cases: zero-element inputs preserve the existing nil result while
// populated input is flattened in order.
func TestFlatten2D_nilAndEmptyResults(t *testing.T) {
	assert.Nil(t, Flatten2D[int](nil))
	assert.Nil(t, Flatten2D(make([][]int, 0)))
	assert.Nil(t, Flatten2D([][]int{{}, nil, {}}))

	result := Flatten2D([][]int{{1, 2}, {}, {3}})
	assert.Equal(t, []int{1, 2, 3}, result)
	assert.NotNil(t, result)
}

// Performance: populated input allocates its result backing array once.
func TestFlatten2D_allocatesResultOnce(t *testing.T) {
	data := make([][]int, 64)
	for i := range data {
		data[i] = []int{i}
	}

	var result []int

	allocations := testing.AllocsPerRun(100, func() {
		result = Flatten2D(data)
	})

	assert.Len(t, result, len(data))
	assert.Equal(t, float64(1), allocations)
}

// An embedded exported interface holds its value indirectly; the traversal must
// still reach the concrete struct behind it to find the ID.
type idCarrier struct {
	ID string
}

type embedsInterface struct {
	fmt.Stringer
	Name string
}

func (i idCarrier) String() string { return i.ID }

func TestExtractID_embeddedInterface_isTraversed(t *testing.T) {
	got := ExtractID(embedsInterface{Stringer: idCarrier{ID: "from-interface"}}, "")

	assert.Equal(t, "from-interface", got)
}

func TestExtractID_embeddedNilInterface_returnsEmpty(t *testing.T) {
	assert.Empty(t, ExtractID(embedsInterface{Name: "no-id"}, ""))
}
