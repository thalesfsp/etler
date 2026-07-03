package shared

import (
	"bytes"
	"context"
	"errors"
	"expvar"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thalesfsp/status"
)

//////
// Marshal / Unmarshal / Encode / Decode / ReadAll.
//////

type failingReadWriter struct{}

func (failingReadWriter) Read([]byte) (int, error)  { return 0, errors.New("read boom") }
func (failingReadWriter) Write([]byte) (int, error) { return 0, errors.New("write boom") }

func TestMarshalUnmarshal(t *testing.T) {
	// Happy path round trip.
	b, err := Marshal(map[string]int{"a": 1})
	require.NoError(t, err)

	var out map[string]int

	require.NoError(t, Unmarshal(b, &out))
	assert.Equal(t, 1, out["a"])

	// Bad path: unmarshalable type / invalid JSON.
	_, err = Marshal(make(chan int))
	assert.Error(t, err)

	assert.Error(t, Unmarshal([]byte("{invalid"), &out))
}

func TestEncodeDecode(t *testing.T) {
	var buf bytes.Buffer

	require.NoError(t, Encode(&buf, map[string]string{"k": "v"}))

	var out map[string]string

	require.NoError(t, Decode(&buf, &out))
	assert.Equal(t, "v", out["k"])

	// Bad paths.
	assert.Error(t, Encode(failingReadWriter{}, "x"))
	assert.Error(t, Decode(strings.NewReader("{invalid"), &out))
}

func TestReadAll(t *testing.T) {
	b, err := ReadAll(strings.NewReader("payload"))
	require.NoError(t, err)
	assert.Equal(t, "payload", string(b))

	_, err = ReadAll(failingReadWriter{})
	assert.Error(t, err)
}

//////
// OnErrorHandler.
//////

// fakeMetrics implements IMetrics for handler tests.
type fakeMetrics struct {
	statusVar *expvar.String
	failed    *expvar.Int
}

func (f *fakeMetrics) GetCounterCreated() *expvar.Int { return nil }
func (f *fakeMetrics) GetCounterRunning() *expvar.Int { return nil }
func (f *fakeMetrics) GetCounterFailed() *expvar.Int  { return f.failed }
func (f *fakeMetrics) GetCounterDone() *expvar.Int    { return nil }
func (f *fakeMetrics) GetStatus() *expvar.String      { return f.statusVar }
func (f *fakeMetrics) GetCreatedAt() time.Time        { return time.Time{} }
func (f *fakeMetrics) GetDuration() *expvar.Int       { return nil }
func (f *fakeMetrics) GetMetrics() map[string]string  { return nil }

// OnErrorHandler wraps the error, flips the status to failed, and counts the
// failure.
func TestOnErrorHandler(t *testing.T) {
	fake := &fakeMetrics{
		statusVar: new(expvar.String),
		failed:    new(expvar.Int),
	}

	boom := errors.New("boom-handler")

	err := OnErrorHandler(context.Background(), fake, nil, boom, "do thing", "test", "unit")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "do thing")
	assert.Equal(t, status.Failed.String(), fake.statusVar.Value())
	assert.Equal(t, int64(1), fake.failed.Value())
	assert.ErrorIs(t, err, boom)
}
