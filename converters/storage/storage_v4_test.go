package storage

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thalesfsp/etler/v4/internal/shared"
	"github.com/thalesfsp/status"
)

type v4NoID struct {
	Name string `json:"name"`
}

type v4WithID struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type v4WithCustomID struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

type v4WithPointerID struct {
	ID   *string `json:"id"`
	Name string  `json:"name"`
}

type v4Unmarshallable struct {
	Action func() `json:"action"`
}

type v4UnrelatedContextKey struct{}

type v4UnrelatedValueContext struct {
	context.Context
}

func (c v4UnrelatedValueContext) Value(key any) any {
	if key == (v4UnrelatedContextKey{}) {
		return "unrelated"
	}

	return c.Context.Value(key)
}

func TestNew_idOptionsDoNotPanic(t *testing.T) {
	stub := &stubStorage{}

	var (
		converter *Storage[v4WithID]
		err       error
	)

	require.NotPanics(t, func() {
		converter, err = New[v4WithID](
			stub,
			"etl",
			WithIDFunc(func(_ context.Context, item v4WithID) (string, error) {
				return "custom-" + item.Name, nil
			}),
			WithIDFieldName[v4WithID]("Name"),
			WithConverterOptions[v4WithID](),
		)
	})
	require.NoError(t, err)
	require.NotNil(t, converter)
}

func TestNew_nilOptionsReturnErrorNotPanic(t *testing.T) {
	tests := []struct {
		name string
		opt  Func[v4NoID]
	}{
		{
			name: "storage converter option",
			opt:  nil,
		},
		{
			name: "underlying converter option",
			opt:  WithConverterOptions[v4NoID](nil),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var (
				converter *Storage[v4NoID]
				err       error
			)

			require.NotPanics(t, func() {
				converter, err = New[v4NoID](&stubStorage{}, "etl", test.opt)
			})
			assert.Nil(t, converter)
			require.Error(t, err)
		})
	}
}

func TestStorage_idResolutionIgnoresUnrelatedContextValues(t *testing.T) {
	stub := &stubStorage{}
	converter, err := New[v4WithCustomID](
		stub,
		"etl",
		WithIDFieldName[v4WithCustomID]("Key"),
	)
	require.NoError(t, err)

	item := v4WithCustomID{Key: "key-7", Name: "alice"}

	backgroundID, err := converter.Run(context.Background(), item)
	require.NoError(t, err)

	unrelatedContext := v4UnrelatedValueContext{Context: context.Background()}
	unrelatedContextID, err := converter.Run(unrelatedContext, item)
	require.NoError(t, err)

	assert.Equal(t, backgroundID, unrelatedContextID)
	assert.Equal(t, []string{"key-7", "key-7"}, stub.createdIDs())
}

func TestNew_nilCustomIDFuncReturnsRequiredArgumentError(t *testing.T) {
	stub := &stubStorage{}
	converter, err := New[v4NoID](
		stub,
		"etl",
		WithIDFunc[v4NoID](nil),
	)

	assert.Nil(t, converter)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "custom id function is nil")
	assert.Empty(t, stub.createdIDs())
}

func TestStorage_customIDFuncErrorPropagatesWithoutWriting(t *testing.T) {
	stub := &stubStorage{}
	derivationErr := errors.New("id derivation failed")
	ctx := context.WithValue(context.Background(), v4UnrelatedContextKey{}, "run-context")
	converter, err := New[v4NoID](
		stub,
		"etl",
		WithIDFunc(func(gotCtx context.Context, _ v4NoID) (string, error) {
			assert.Equal(t, "run-context", gotCtx.Value(v4UnrelatedContextKey{}))

			return "", derivationErr
		}),
	)
	require.NoError(t, err)

	gotID, err := converter.Run(ctx, v4NoID{Name: "alice"})

	assert.Empty(t, gotID)
	require.Error(t, err)
	assert.ErrorIs(t, err, derivationErr)
	assert.Empty(t, stub.createdIDs())
}

func TestStorage_usesExtractedOrContentDerivedID(t *testing.T) {
	tests := []struct {
		name   string
		item   v4WithID
		wantID string
	}{
		{
			name:   "non-empty ID field",
			item:   v4WithID{ID: "item-42", Name: "alice"},
			wantID: "item-42",
		},
		{
			name:   "empty ID field falls back to content",
			item:   v4WithID{Name: "alice"},
			wantID: "4f5c7a3b9e3939f0fe9eea2f3c7e9c15ddfe132c19dbf58a01295eaed6092ba3",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stub := &stubStorage{}
			converter, err := New[v4WithID](stub, "etl")
			require.NoError(t, err)

			gotID, err := converter.Run(context.Background(), test.item)
			require.NoError(t, err)

			assert.Equal(t, test.wantID, gotID)
			assert.Equal(t, []string{test.wantID}, stub.createdIDs())
		})
	}

	t.Run("item without ID field uses exact content hash", func(t *testing.T) {
		stub := &stubStorage{}
		converter, err := New[v4NoID](stub, "etl")
		require.NoError(t, err)

		gotID, err := converter.Run(context.Background(), v4NoID{Name: "alice"})
		require.NoError(t, err)

		assert.Equal(t, "3b8f02c64624e355de637e609642b441ab1427b619d9fb91cb6c7b0e8f8ceed1", gotID)
		assert.Equal(t, []string{"3b8f02c64624e355de637e609642b441ab1427b619d9fb91cb6c7b0e8f8ceed1"}, stub.createdIDs())
	})
}

func TestStorage_absentPointerIDFallsBackToContent(t *testing.T) {
	tests := []struct {
		name string
		id   *string
	}{
		{name: "nil pointer"},
		{name: "whitespace-only value", id: func() *string {
			value := " \t\n"

			return &value
		}()},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stub := &stubStorage{}
			converter, err := New[v4WithPointerID](stub, "etl")
			require.NoError(t, err)

			item := v4WithPointerID{ID: test.id, Name: "alice"}
			content, err := shared.Marshal(item)
			require.NoError(t, err)
			wantID := shared.GenerateIDBasedOnContent(string(content))

			gotID, err := converter.Run(context.Background(), item)
			require.NoError(t, err)

			assert.Equal(t, wantID, gotID)
			assert.Equal(t, []string{wantID}, stub.createdIDs())
		})
	}
}

func TestStorage_sameItemTwiceUsesSameID(t *testing.T) {
	stub := &stubStorage{}
	converter, err := New[v4NoID](stub, "etl")
	require.NoError(t, err)

	item := v4NoID{Name: "alice"}
	_, err = converter.Run(context.Background(), item)
	require.NoError(t, err)
	_, err = converter.Run(context.Background(), item)
	require.NoError(t, err)

	ids := stub.createdIDs()
	require.Len(t, ids, 2)
	require.Equal(t, ids[0], ids[1], "same item retry must reuse its storage id")

	_, err = converter.Run(context.Background(), v4NoID{Name: "beta"})
	require.NoError(t, err)

	ids = stub.createdIDs()
	require.Len(t, ids, 3)
	require.NotEqual(t, ids[0], ids[2], "different items must use different storage ids")
}

func TestStorage_structurallyIdenticalItemsDeduplicate(t *testing.T) {
	stub := &stubStorage{}
	converter, err := New[v4NoID](stub, "etl")
	require.NoError(t, err)

	first := v4NoID{Name: "alice"}
	second := v4NoID{Name: "alice"}
	_, err = converter.Run(context.Background(), first)
	require.NoError(t, err)
	_, err = converter.Run(context.Background(), second)
	require.NoError(t, err)

	ids := stub.createdIDs()
	require.Len(t, ids, 2)
	assert.Equal(t, ids[0], ids[1], "structurally identical items intentionally deduplicate")
}

func TestStorage_idOptions(t *testing.T) {
	t.Run("custom id function has first priority", func(t *testing.T) {
		stub := &stubStorage{}
		converter, err := New[v4WithID](
			stub,
			"etl",
			WithIDFunc(func(_ context.Context, item v4WithID) (string, error) {
				return "custom-" + item.Name, nil
			}),
			WithIDFieldName[v4WithID]("Name"),
		)
		require.NoError(t, err)

		gotID, err := converter.Run(context.Background(), v4WithID{ID: "field-id", Name: "alice"})
		require.NoError(t, err)

		assert.Equal(t, "custom-alice", gotID)
		assert.Equal(t, []string{"custom-alice"}, stub.createdIDs())
	})

	t.Run("configured field name is honoured", func(t *testing.T) {
		stub := &stubStorage{}
		converter, err := New[v4WithCustomID](
			stub,
			"etl",
			WithIDFieldName[v4WithCustomID]("Key"),
		)
		require.NoError(t, err)

		gotID, err := converter.Run(context.Background(), v4WithCustomID{Key: "key-7", Name: "alice"})
		require.NoError(t, err)

		assert.Equal(t, "key-7", gotID)
		assert.Equal(t, []string{"key-7"}, stub.createdIDs())
	})
}

func TestStorage_emptyCustomIDFailsWithoutWriting(t *testing.T) {
	stub := &stubStorage{}
	converter, err := New[v4NoID](
		stub,
		"etl",
		WithIDFunc(func(context.Context, v4NoID) (string, error) { return "", nil }),
	)
	require.NoError(t, err)

	gotID, err := converter.Run(context.Background(), v4NoID{Name: "alice"})

	assert.Empty(t, gotID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "custom id function returned an empty id")
	assert.Empty(t, stub.createdIDs())
	assert.Equal(t, int64(0), stub.created.Load())
	assert.Equal(t, "1", converter.GetCounterFailed().String())
	assert.Equal(t, status.Failed.String(), converter.GetStatus().Value())
}

func TestStorage_createErrorIsPropagatedAndRecorded(t *testing.T) {
	stub := &stubStorage{failWith: errors.New("write committed but response lost")}
	converter, err := New[v4NoID](stub, "etl")
	require.NoError(t, err)

	gotID, err := converter.Run(context.Background(), v4NoID{Name: "alice"})

	assert.Empty(t, gotID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "write committed but response lost")
	assert.Equal(t, []string{"3b8f02c64624e355de637e609642b441ab1427b619d9fb91cb6c7b0e8f8ceed1"}, stub.createdIDs())
	assert.Equal(t, "1", converter.GetCounterFailed().String())
	assert.Equal(t, status.Failed.String(), converter.GetStatus().Value())
}

func TestStorage_unmarshallableItemReturnsErrorWithoutWriting(t *testing.T) {
	stub := &stubStorage{}
	converter, err := New[v4Unmarshallable](stub, "etl")
	require.NoError(t, err)

	var runErr error
	require.NotPanics(t, func() {
		_, runErr = converter.Run(context.Background(), v4Unmarshallable{Action: func() {}})
	})

	require.Error(t, runErr)
	assert.Contains(t, runErr.Error(), "marshal")
	assert.Empty(t, stub.createdIDs())
	assert.Equal(t, int64(0), stub.created.Load())
}
