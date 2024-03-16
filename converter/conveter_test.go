package converter

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/thalesfsp/status"
)

func TestNew(t *testing.T) {
	conv, err := New(
		"int to float",
		"converts int to float",
		func(ctx context.Context, in io.Reader) (float64, error) {
			time.Sleep(100 * time.Millisecond)

			return 1.0, nil
		},
	)
	assert.NoError(t, err)

	f, err := conv.Run(context.Background(), strings.NewReader("1"))
	if err != nil {
		t.Fatal(err)
	}

	// Validates processors metrics.
	assert.Equal(t, int64(1), conv.GetCounterCreated().Value())
	assert.Equal(t, int64(1), conv.GetCounterRunning().Value())
	assert.Equal(t, int64(0), conv.GetCounterFailed().Value())
	assert.Equal(t, int64(1), conv.GetCounterDone().Value())
	assert.Equal(t, status.Done.String(), conv.GetStatus().Value())
	assert.Equal(t, true, conv.GetDuration().Value() >= int64(100))
	assert.NotEmpty(t, conv.GetCreatedAt())
	assert.Equal(t, status.Done.String(), conv.GetStatus().Value())

	// Validates processors metrics.
	assert.Equal(t, int64(1), conv.GetCounterCreated().Value())
	assert.Equal(t, int64(1), conv.GetCounterRunning().Value())
	assert.Equal(t, int64(0), conv.GetCounterFailed().Value())
	assert.Equal(t, int64(1), conv.GetCounterDone().Value())
	assert.Equal(t, status.Done.String(), conv.GetStatus().Value())

	assert.Equal(t, 1.0, f)
}
