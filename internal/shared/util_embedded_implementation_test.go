package shared

import (
	"os"
	"os/exec"
	"runtime/debug"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const embeddedImplementationCycleChild = "ETLER_EMBEDDED_IMPLEMENTATION_CYCLE_CHILD"

//////
// HasNilEmbeddedImplementation.
//////

type testImplementation interface {
	implement()
}

type testConcreteImplementation struct {
	name string
}

func (testConcreteImplementation) implement() {}

type testScalarImplementation int

func (testScalarImplementation) implement() {}

type testNilImplementation struct {
	testImplementation
}

type testTwoLevelNilImplementation struct {
	testNilImplementation
}

type testThreeLevelNilImplementation struct {
	testTwoLevelNilImplementation
}

type testUnexportedNilImplementation struct {
	testImplementation
}

type testUnexportedNilWrapper struct {
	testUnexportedNilImplementation
}

type testValidImplementationWrapper struct {
	testImplementation
}

type testValidImplementationOuter struct {
	testValidImplementationWrapper
}

type testImplementationCycle struct {
	*testImplementationCycle
}

func (*testImplementationCycle) implement() {}

type testDiamondImplementation struct {
	name string
}

func (*testDiamondImplementation) implement() {}

type testDiamondLeft struct {
	*testDiamondImplementation
}

type testDiamondRight struct {
	*testDiamondImplementation
}

type testImplementationDiamond struct {
	testDiamondLeft
	testDiamondRight
}

func (*testImplementationDiamond) implement() {}

func TestHasNilEmbeddedImplementation_nilAtAnyDepth(t *testing.T) {
	tests := []struct {
		name           string
		implementation testImplementation
	}{
		{
			name:           "depth one",
			implementation: &testNilImplementation{},
		},
		{
			name:           "depth two",
			implementation: &testTwoLevelNilImplementation{},
		},
		{
			name:           "depth three",
			implementation: &testThreeLevelNilImplementation{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.True(t, HasNilEmbeddedImplementation(tt.implementation))
		})
	}
}

func TestHasNilEmbeddedImplementation_unexportedEmbeddedTypeDoesNotPanic(t *testing.T) {
	implementation := testImplementation(&testUnexportedNilWrapper{})

	assert.NotPanics(t, func() {
		assert.True(t, HasNilEmbeddedImplementation(implementation))
	})
}

func TestHasNilEmbeddedImplementation_validAtAnyDepth(t *testing.T) {
	plain := testImplementation(testConcreteImplementation{name: "plain"})

	tests := []struct {
		name           string
		implementation testImplementation
	}{
		{
			name:           "plain",
			implementation: plain,
		},
		{
			name: "wrapped once",
			implementation: &testValidImplementationWrapper{
				testImplementation: plain,
			},
		},
		{
			name: "wrapped twice",
			implementation: &testValidImplementationOuter{
				testValidImplementationWrapper: testValidImplementationWrapper{
					testImplementation: plain,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.False(t, HasNilEmbeddedImplementation(tt.implementation))
		})
	}
}

func TestHasNilEmbeddedImplementation_nonStructAndNilValuesDoNotPanic(t *testing.T) {
	var (
		nilPointer        *testConcreteImplementation
		nilImplementation testImplementation
	)

	tests := []struct {
		name           string
		implementation testImplementation
		want           bool
	}{
		{
			name:           "non-struct",
			implementation: testScalarImplementation(1),
			want:           false,
		},
		{
			name:           "nil pointer",
			implementation: nilPointer,
			want:           true,
		},
		{
			name:           "nil interface",
			implementation: nilImplementation,
			want:           true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				assert.Equal(
					t,
					tt.want,
					HasNilEmbeddedImplementation(tt.implementation),
				)
			})
		})
	}
}

func TestHasNilEmbeddedImplementation_selfReferentialPointerTerminates(t *testing.T) {
	if os.Getenv(embeddedImplementationCycleChild) == "1" {
		debug.SetMaxStack(1 << 20)

		implementation := &testImplementationCycle{}
		implementation.testImplementationCycle = implementation

		assert.False(t, HasNilEmbeddedImplementation[testImplementation](implementation))

		return
	}

	cmd := exec.Command(
		os.Args[0],
		"-test.run=^TestHasNilEmbeddedImplementation_selfReferentialPointerTerminates$",
	)
	cmd.Env = append(
		os.Environ(),
		embeddedImplementationCycleChild+"=1",
		"GOTRACEBACK=none",
	)

	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "cyclic helper subprocess failed:\n%s", output)
}

func TestHasNilEmbeddedImplementation_nonCyclicDiamond(t *testing.T) {
	shared := &testDiamondImplementation{name: "shared"}
	implementation := &testImplementationDiamond{
		testDiamondLeft: testDiamondLeft{
			testDiamondImplementation: shared,
		},
		testDiamondRight: testDiamondRight{
			testDiamondImplementation: shared,
		},
	}

	assert.False(t, HasNilEmbeddedImplementation[testImplementation](implementation))
}
