package squad

import (
	"context"
	"time"
)

type delayedContext struct {
	parentCtx context.Context
	ch        chan struct{}
}

func WithDelay(ctx context.Context, delay time.Duration) context.Context {
	ch := make(chan struct{})
	go func() {
		<-ctx.Done()
		time.Sleep(delay)
		ch <- struct{}{}
		close(ch)
	}()
	return &delayedContext{parentCtx: ctx, ch: ch}
}

var _ context.Context = (*delayedContext)(nil)

func (ctx *delayedContext) Done() <-chan struct{} {
	return ctx.ch
}

func (ctx *delayedContext) Err() error {
	return ctx.parentCtx.Err()
}

func (ctx *delayedContext) Deadline() (time.Time, bool) {
	return ctx.parentCtx.Deadline()
}

func (ctx *delayedContext) Value(key any) any {
	return ctx.parentCtx.Value(key)
}
