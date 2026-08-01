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

type v4PanickingStringerID struct {
	Value string `json:"value"`
}

func (v4PanickingStringerID) String() string {
	panic("String must not be called implicitly")
}

type v4WithPanickingStringerID struct {
	ID   v4PanickingStringerID `json:"id"`
	Name string                `json:"name"`
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
		processor *Storage[v4WithID]
		err       error
	)

	require.NotPanics(t, func() {
		processor, err = New[v4WithID](
			stub,
			1,
			"prefix-",
			WithIDFunc(func(_ context.Context, item v4WithID) (string, error) {
				return "custom-" + item.Name, nil
			}),
			WithIDFieldName[v4WithID]("Name"),
			WithProcessorOptions[v4WithID](),
		)
	})
	require.NoError(t, err)
	require.NotNil(t, processor)
}

func TestNew_nilOptionsReturnErrorNotPanic(t *testing.T) {
	tests := []struct {
		name string
		opt  Func[v4NoID]
	}{
		{
			name: "storage processor option",
			opt:  nil,
		},
		{
			name: "underlying processor option",
			opt:  WithProcessorOptions[v4NoID](nil),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var (
				processor *Storage[v4NoID]
				err       error
			)

			require.NotPanics(t, func() {
				processor, err = New[v4NoID](&stubStorage{}, 1, "prefix-", test.opt)
			})
			assert.Nil(t, processor)
			require.Error(t, err)
		})
	}
}

func TestStorage_idResolutionIgnoresUnrelatedContextValues(t *testing.T) {
	stub := &stubStorage{}
	processor, err := New[v4WithCustomID](
		stub,
		1,
		"prefix-",
		WithIDFieldName[v4WithCustomID]("Key"),
	)
	require.NoError(t, err)

	item := v4WithCustomID{Key: "key-7", Name: "alice"}

	_, err = processor.Run(context.Background(), []v4WithCustomID{item})
	require.NoError(t, err)

	unrelatedContext := v4UnrelatedValueContext{Context: context.Background()}
	_, err = processor.Run(unrelatedContext, []v4WithCustomID{item})
	require.NoError(t, err)

	assert.Equal(t, []string{"prefix-key-7", "prefix-key-7"}, stub.createdIDs())
}

func TestNew_nilCustomIDFuncReturnsRequiredArgumentError(t *testing.T) {
	stub := &stubStorage{}
	processor, err := New[v4NoID](
		stub,
		1,
		"prefix-",
		WithIDFunc[v4NoID](nil),
	)

	assert.Nil(t, processor)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "custom id function is nil")
	assert.Empty(t, stub.createdIDs())
}

func TestStorage_customIDFuncErrorPropagatesWithoutWriting(t *testing.T) {
	stub := &stubStorage{}
	derivationErr := errors.New("id derivation failed")
	contextValue := make(chan any, 1)
	ctx := context.WithValue(context.Background(), v4UnrelatedContextKey{}, "run-context")
	processor, err := New[v4NoID](
		stub,
		1,
		"prefix-",
		WithIDFunc(func(gotCtx context.Context, _ v4NoID) (string, error) {
			contextValue <- gotCtx.Value(v4UnrelatedContextKey{})

			return "", derivationErr
		}),
	)
	require.NoError(t, err)

	out, err := processor.Run(ctx, []v4NoID{{Name: "alice"}})

	assert.Nil(t, out)
	require.Error(t, err)
	assert.ErrorIs(t, err, derivationErr)
	assert.Equal(t, "run-context", <-contextValue)
	assert.Empty(t, stub.createdIDs())
}

func TestStorage_customTargetReachesCreate(t *testing.T) {
	stub := &stubStorage{}
	processor, err := New[v4NoID](
		stub,
		1,
		"prefix-",
		WithTarget[v4NoID]("archive"),
	)
	require.NoError(t, err)

	_, err = processor.Run(context.Background(), []v4NoID{{Name: "alice"}})
	require.NoError(t, err)

	assert.Equal(t, []string{"archive"}, stub.createdTargets())
}

func TestNew_blankTargetReturnsError(t *testing.T) {
	tests := []struct {
		name   string
		target string
	}{
		{name: "empty"},
		{name: "whitespace-only", target: " \t"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stub := &stubStorage{}

			processor, err := New[v4NoID](
				stub,
				1,
				"prefix-",
				WithTarget[v4NoID](test.target),
			)

			assert.Nil(t, processor)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "target")
			assert.Empty(t, stub.createdTargets())
		})
	}
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
			wantID: "prefix-item-42",
		},
		{
			name:   "empty ID field falls back to content",
			item:   v4WithID{Name: "alice"},
			wantID: "prefix-4f5c7a3b9e3939f0fe9eea2f3c7e9c15ddfe132c19dbf58a01295eaed6092ba3",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stub := &stubStorage{}
			processor, err := New[v4WithID](stub, 1, "prefix-")
			require.NoError(t, err)

			_, err = processor.Run(context.Background(), []v4WithID{test.item})
			require.NoError(t, err)

			assert.Equal(t, []string{test.wantID}, stub.createdIDs())
		})
	}

	t.Run("item without ID field uses exact content hash and prefix", func(t *testing.T) {
		stub := &stubStorage{}
		processor, err := New[v4NoID](stub, 1, "prefix-")
		require.NoError(t, err)

		_, err = processor.Run(context.Background(), []v4NoID{{Name: "alice"}})
		require.NoError(t, err)

		assert.Equal(t, []string{"prefix-3b8f02c64624e355de637e609642b441ab1427b619d9fb91cb6c7b0e8f8ceed1"}, stub.createdIDs())
	})
}

func TestStorage_stringerOnlyIDFallsBackToContentWithoutCallingString(t *testing.T) {
	stub := &stubStorage{}
	processor, err := New[v4WithPanickingStringerID](stub, 1, "prefix-")
	require.NoError(t, err)

	item := v4WithPanickingStringerID{
		ID:   v4PanickingStringerID{Value: "field-content"},
		Name: "alice",
	}
	content, err := shared.Marshal(item)
	require.NoError(t, err)
	wantID := "prefix-" + shared.GenerateIDBasedOnContent(string(content))

	var out []v4WithPanickingStringerID
	require.NotPanics(t, func() {
		out, err = processor.Run(context.Background(), []v4WithPanickingStringerID{item})
	})
	require.NoError(t, err)
	assert.Equal(t, []v4WithPanickingStringerID{item}, out)
	assert.Equal(t, []string{wantID}, stub.createdIDs())
}

func TestStorage_sameItemTwiceUsesSameID(t *testing.T) {
	stub := &stubStorage{}
	processor, err := New[v4NoID](stub, 1, "prefix-")
	require.NoError(t, err)

	item := v4NoID{Name: "alice"}
	_, err = processor.Run(context.Background(), []v4NoID{item, item})
	require.NoError(t, err)

	ids := stub.createdIDs()
	require.Len(t, ids, 2)
	require.Equal(t, ids[0], ids[1], "same item retry must reuse its storage id")

	_, err = processor.Run(context.Background(), []v4NoID{{Name: "beta"}})
	require.NoError(t, err)

	ids = stub.createdIDs()
	require.Len(t, ids, 3)
	require.NotEqual(t, ids[0], ids[2], "different items must use different storage ids")
}

func TestStorage_structurallyIdenticalItemsDeduplicate(t *testing.T) {
	stub := &stubStorage{}
	processor, err := New[v4NoID](stub, 1, "prefix-")
	require.NoError(t, err)

	first := v4NoID{Name: "alice"}
	second := v4NoID{Name: "alice"}
	_, err = processor.Run(context.Background(), []v4NoID{first, second})
	require.NoError(t, err)

	ids := stub.createdIDs()
	require.Len(t, ids, 2)
	assert.Equal(t, ids[0], ids[1], "structurally identical items intentionally deduplicate")
}

func TestStorage_idOptions(t *testing.T) {
	t.Run("custom id function has first priority and keeps prefix", func(t *testing.T) {
		stub := &stubStorage{}
		processor, err := New[v4WithID](
			stub,
			1,
			"prefix-",
			WithIDFunc(func(_ context.Context, item v4WithID) (string, error) {
				return "custom-" + item.Name, nil
			}),
			WithIDFieldName[v4WithID]("Name"),
		)
		require.NoError(t, err)

		_, err = processor.Run(context.Background(), []v4WithID{{ID: "field-id", Name: "alice"}})
		require.NoError(t, err)

		assert.Equal(t, []string{"prefix-custom-alice"}, stub.createdIDs())
	})

	t.Run("configured field name is honoured", func(t *testing.T) {
		stub := &stubStorage{}
		processor, err := New[v4WithCustomID](
			stub,
			1,
			"prefix-",
			WithIDFieldName[v4WithCustomID]("Key"),
		)
		require.NoError(t, err)

		_, err = processor.Run(context.Background(), []v4WithCustomID{{Key: "key-7", Name: "alice"}})
		require.NoError(t, err)

		assert.Equal(t, []string{"prefix-key-7"}, stub.createdIDs())
	})
}

func TestStorage_blankCustomIDFailsWithoutWriting(t *testing.T) {
	tests := []struct {
		name string
		id   string
	}{
		{name: "empty"},
		{name: "whitespace-only", id: " \t"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stub := &stubStorage{}
			processor, err := New[v4NoID](
				stub,
				1,
				"prefix-",
				WithIDFunc(func(context.Context, v4NoID) (string, error) { return test.id, nil }),
			)
			require.NoError(t, err)

			out, err := processor.Run(context.Background(), []v4NoID{{Name: "alice"}})

			assert.Nil(t, out)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "custom id function returned an empty id")
			assert.Empty(t, stub.createdIDs())
			assert.Equal(t, int64(0), stub.created.Load())
			assert.Equal(t, "1", processor.GetCounterFailed().String())
			assert.Equal(t, status.Failed.String(), processor.GetStatus().Value())
		})
	}
}

func TestStorage_createErrorIsPropagatedAndRecorded(t *testing.T) {
	stub := &stubStorage{failWith: errors.New("write committed but response lost")}
	processor, err := New[v4NoID](stub, 1, "prefix-")
	require.NoError(t, err)

	out, err := processor.Run(context.Background(), []v4NoID{{Name: "alice"}})

	assert.Nil(t, out)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "write committed but response lost")
	assert.Equal(t, []string{"prefix-3b8f02c64624e355de637e609642b441ab1427b619d9fb91cb6c7b0e8f8ceed1"}, stub.createdIDs())
	assert.Equal(t, "1", processor.GetCounterFailed().String())
	assert.Equal(t, status.Failed.String(), processor.GetStatus().Value())
}

func TestStorage_unmarshallableItemReturnsErrorWithoutWriting(t *testing.T) {
	stub := &stubStorage{}
	processor, err := New[v4Unmarshallable](stub, 1, "prefix-")
	require.NoError(t, err)

	var runErr error
	require.NotPanics(t, func() {
		_, runErr = processor.Run(
			context.Background(),
			[]v4Unmarshallable{{Action: func() {}}},
		)
	})

	require.Error(t, runErr)
	assert.Contains(t, runErr.Error(), "marshal")
	assert.Empty(t, stub.createdIDs())
	assert.Equal(t, int64(0), stub.created.Load())
}
