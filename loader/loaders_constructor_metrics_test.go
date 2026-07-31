package loader

import (
	"context"
	"expvar"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thalesfsp/etler/v4/internal/metrics"
)

var loaderConstructorMetricTestID atomic.Uint64

func uniqueLoaderMetricName(path string) string {
	return fmt.Sprintf("loader-rejected-constructor-%s-%d", path, loaderConstructorMetricTestID.Add(1))
}

func loaderMetricExpectations(name string) map[string]string {
	return map[string]string{
		fmt.Sprintf("etler.loader.%s.created.counter", name):  "1",
		fmt.Sprintf("etler.loader.%s.done.counter", name):     "0",
		fmt.Sprintf("etler.loader.%s.failed.counter", name):   "0",
		fmt.Sprintf("etler.loader.%s.running.counter", name):  "0",
		fmt.Sprintf("etler.loader.%s.duration.counter", name): "0",
		fmt.Sprintf("etler.loader.%s.status", name):           `"created"`,
	}
}

func assertRejectedLoaderDidNotPublish(t *testing.T, name string) {
	t.Helper()

	var published []string
	for metricName := range loaderMetricExpectations(name) {
		if expvar.Get(metricName) != nil {
			published = append(published, metricName)
		}
	}

	assert.Empty(t, published, "rejected loader published metrics: %v", published)
}

func assertValidLoaderPublishedMetrics(t *testing.T, name string) {
	t.Helper()

	for metricName, expected := range loaderMetricExpectations(name) {
		metric := expvar.Get(metricName)
		require.NotNil(t, metric, "valid loader did not publish %q", metricName)
		assert.Equal(t, expected, metric.String(), "unexpected value for %q", metricName)
	}
}

func TestLoaderNew_rejectedInputsDoNotPublishMetrics(t *testing.T) {
	tests := []struct {
		name    string
		wantErr string
		reject  func(string) (ILoader[int, int], error)
	}{
		{
			name:    "nil-function",
			wantErr: "invalid struct. Original Error: Key: 'Loader[int,int].Func' Error:Field validation for 'Func' failed on the 'required' tag",
			reject: func(name string) (ILoader[int, int], error) {
				return New[int, int](name, "nil function", nil)
			},
		},
		{
			name:    "nil-option",
			wantErr: "loader option required",
			reject: func(name string) (ILoader[int, int], error) {
				return New(
					name,
					"nil option",
					func(_ context.Context, in int) (int, error) { return in, nil },
					nil,
				)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name := uniqueLoaderMetricName(tt.name)
			t.Setenv(metrics.PublishEnvVar, "true")

			l, err := tt.reject(name)

			assert.Nil(t, l)
			assert.EqualError(t, err, tt.wantErr)
			assertRejectedLoaderDidNotPublish(t, name)

			l, err = New(
				name,
				"valid negative control",
				func(_ context.Context, in int) (int, error) { return in, nil },
			)
			require.NoError(t, err)
			require.NotNil(t, l)
			assertValidLoaderPublishedMetrics(t, name)
		})
	}
}

func TestLoaderNew_rejectsStructurallyInvalidOptionResult(t *testing.T) {
	l, err := New(
		uniqueLoaderMetricName("invalid-option-result"),
		"invalid option result",
		func(_ context.Context, in int) (int, error) { return in, nil },
		func(ILoader[int, int]) ILoader[int, int] {
			return &Loader[int, int]{}
		},
	)

	assert.Nil(t, l)
	assert.EqualError(
		t,
		err,
		"invalid struct. Original Error: Key: 'Loader[int,int].Func' Error:Field validation for 'Func' failed on the 'required' tag\nKey: 'Loader[int,int].Logger' Error:Field validation for 'Logger' failed on the 'required' tag\nKey: 'Loader[int,int].Name' Error:Field validation for 'Name' failed on the 'required' tag",
	)
}
