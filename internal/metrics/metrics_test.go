package metrics

import (
	"expvar"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thalesfsp/status"
)

// Default: metrics are NOT registered in the global expvar registry, so
// constructing many entities doesn't leak registry entries.
func TestMetrics_unpublishedByDefault(t *testing.T) {
	i := NewInt("etler.test.unpublished.int")
	require.NotNil(t, i)
	i.Add(1)
	assert.Equal(t, int64(1), i.Value())

	s := NewString("etler.test.unpublished.string")
	require.NotNil(t, s)
	assert.Equal(t, status.Created.String(), s.Value())

	assert.Nil(t, expvar.Get("etler.test.unpublished.int"),
		"metrics must not be globally registered by default")
	assert.Nil(t, expvar.Get("etler.test.unpublished.string"))
}

// Opt-in publishing: metrics register under their pattern name, and the same
// name is REUSED (no panic, shared var) instead of growing the registry.
func TestMetrics_publishOptIn_registersAndReuses(t *testing.T) {
	t.Setenv(PublishEnvVar, "true")

	i1 := NewIntWithPattern("test", "publish-reuse", status.Created)
	i2 := NewIntWithPattern("test", "publish-reuse", status.Created)
	assert.Same(t, i1, i2, "same pattern name must reuse the same metric")

	name := "etler.test.publish-reuse." + status.Created.String() + "." + DefaultMetricCounterLabel
	assert.NotNil(t, expvar.Get(name), "published metric must be registered")

	s1 := NewStringWithPattern("test", "publish-reuse", status.Name)
	s2 := NewStringWithPattern("test", "publish-reuse", status.Name)
	assert.Same(t, s1, s2)
}

// Pattern names are stable and human-readable (no random suffix).
func TestMetrics_patternNames(t *testing.T) {
	t.Setenv(PublishEnvVar, "true")

	NewIntWithPattern("pipeline", "my-pipeline", status.Done)
	assert.NotNil(t, expvar.Get("etler.pipeline.my-pipeline.done.counter"))
}

// Reusing a published metric must NOT clobber its accumulated value.
func TestMetrics_publishReuse_doesNotResetValues(t *testing.T) {
	t.Setenv(PublishEnvVar, "true")

	// Relative to the current value: published vars persist for the process
	// lifetime, so this test must be idempotent across -count reruns.
	i1 := NewIntWithPattern("test", "no-clobber", status.Done)
	base := i1.Value()
	i1.Add(5)

	i2 := NewIntWithPattern("test", "no-clobber", status.Done)
	require.Same(t, i1, i2)
	assert.Equal(t, base+5, i2.Value(),
		"constructing a same-named entity must not reset the shared counter")

	s1 := NewStringWithPattern("test", "no-clobber", status.Name)
	s1.Set("something-live")

	s2 := NewStringWithPattern("test", "no-clobber", status.Name)
	require.Same(t, s1, s2)
	assert.Equal(t, "something-live", s2.Value(),
		"constructing a same-named entity must not reset the shared string")
}
