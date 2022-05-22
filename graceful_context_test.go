package squad

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWithGracefulPeriod(t *testing.T) {
	tests := []struct {
		name           string
		parentCtx      context.Context
		gracefulPeriod time.Duration
	}{
		{
			name:           "basic test",
			parentCtx:      context.Background(),
			gracefulPeriod: 100 * time.Microsecond,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := WithGracefulPeriod(tt.parentCtx, tt.gracefulPeriod)
			assert.NoError(t, ctx.Err())
			cancel()
			assert.Error(t, ErrContextCancelled, ctx.Err())
			<-ctx.Done()
			assert.NotErrorIs(t, ErrContextCancelled, ctx.Err())
		})
	}
}
