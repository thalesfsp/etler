package converter

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

var converterConstructorMetricTestID atomic.Uint64

func uniqueConverterMetricName(path string) string {
	return fmt.Sprintf("converter-rejected-constructor-%s-%d", path, converterConstructorMetricTestID.Add(1))
}

func converterMetricExpectations(name string) map[string]string {
	return map[string]string{
		fmt.Sprintf("etler.converter.%s.created.counter", name):  "1",
		fmt.Sprintf("etler.converter.%s.done.counter", name):     "0",
		fmt.Sprintf("etler.converter.%s.failed.counter", name):   "0",
		fmt.Sprintf("etler.converter.%s.running.counter", name):  "0",
		fmt.Sprintf("etler.converter.%s.duration.counter", name): "0",
		fmt.Sprintf("etler.converter.%s.status", name):           `"created"`,
	}
}

func assertRejectedConverterDidNotPublish(t *testing.T, name string) {
	t.Helper()

	var published []string
	for metricName := range converterMetricExpectations(name) {
		if expvar.Get(metricName) != nil {
			published = append(published, metricName)
		}
	}

	assert.Empty(t, published, "rejected converter published metrics: %v", published)
}

func assertValidConverterPublishedMetrics(t *testing.T, name string) {
	t.Helper()

	for metricName, expected := range converterMetricExpectations(name) {
		metric := expvar.Get(metricName)
		require.NotNil(t, metric, "valid converter did not publish %q", metricName)
		assert.Equal(t, expected, metric.String(), "unexpected value for %q", metricName)
	}
}

func TestConverterNew_rejectedInputsDoNotPublishMetrics(t *testing.T) {
	tests := []struct {
		name    string
		wantErr string
		reject  func(string) (IConverter[int, int], error)
	}{
		{
			name:    "nil-function",
			wantErr: "invalid struct. Original Error: Key: 'Converter[int,int].Func' Error:Field validation for 'Func' failed on the 'required' tag",
			reject: func(name string) (IConverter[int, int], error) {
				return New[int, int](name, "nil function", nil)
			},
		},
		{
			name:    "nil-option",
			wantErr: "converter option required",
			reject: func(name string) (IConverter[int, int], error) {
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
			name := uniqueConverterMetricName(tt.name)
			t.Setenv(metrics.PublishEnvVar, "true")

			c, err := tt.reject(name)

			assert.Nil(t, c)
			assert.EqualError(t, err, tt.wantErr)
			assertRejectedConverterDidNotPublish(t, name)

			c, err = New(
				name,
				"valid negative control",
				func(_ context.Context, in int) (int, error) { return in, nil },
			)
			require.NoError(t, err)
			require.NotNil(t, c)
			assertValidConverterPublishedMetrics(t, name)
		})
	}
}

func TestConverterNew_rejectsStructurallyInvalidOptionResult(t *testing.T) {
	c, err := New(
		uniqueConverterMetricName("invalid-option-result"),
		"invalid option result",
		func(_ context.Context, in int) (int, error) { return in, nil },
		func(IConverter[int, int]) IConverter[int, int] {
			return &Converter[int, int]{}
		},
	)

	assert.Nil(t, c)
	assert.EqualError(
		t,
		err,
		"invalid struct. Original Error: Key: 'Converter[int,int].Func' Error:Field validation for 'Func' failed on the 'required' tag\nKey: 'Converter[int,int].Logger' Error:Field validation for 'Logger' failed on the 'required' tag\nKey: 'Converter[int,int].Name' Error:Field validation for 'Name' failed on the 'required' tag",
	)
}
