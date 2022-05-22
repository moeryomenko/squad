package squad

import (
	"context"
	"errors"
	"sync"
	"time"
)

var ErrContextCancelled = errors.New("context cancelled")

// WithGracefulPeriod returns context with deferred cancelation.
func WithGracefulPeriod(parentCtx context.Context, gracefulPeriod time.Duration) (context.Context, func()) {
	cancelCtx, cancel := context.WithCancel(parentCtx)

	ctx := gracefulContext{Context: cancelCtx}

	return &ctx, ctx.cancel(cancel, gracefulPeriod)
}

type gracefulContext struct {
	context.Context

	mu         sync.Mutex
	isCanceled bool
}

func (ctx *gracefulContext) Err() error {
	ctx.mu.Lock()
	err := ctx.Context.Err()
	isCanceled := ctx.isCanceled
	ctx.mu.Unlock()

	if isCanceled && err == nil {
		return ErrContextCancelled
	}
	return err
}

func (ctx *gracefulContext) cancel(cancelFn func(), gracefulPeriod time.Duration) func() {
	return func() {
		ctx.mu.Lock()
		ctx.isCanceled = true
		ctx.mu.Unlock()

		<-time.After(gracefulPeriod)
		cancelFn()
	}
}
