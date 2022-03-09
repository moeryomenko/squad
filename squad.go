// Package squad contains a shared shutdown primitive.
package squad

import (
	"context"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

// Squad is a collection of goroutines that go up and running altogether.
// If one goroutine exits, other goroutines also go down.
type Squad struct {
	// primitives for control running goroutines.
	wg     sync.WaitGroup
	ctx    context.Context
	cancel func()
	funcs  []func(ctx context.Context) error

	// primitives for control goroutines shutdowning.
	cancellationDelay time.Duration
	cancellationFuncs []func(ctx context.Context) error

	// bootstrap functions.
	bootstraps []func(context.Context) error

	// guarded errors.
	mtx  sync.Mutex
	errs []error
}

const defaultCancellationDelay = 2 * time.Second

// Run runs the fn. When fn is done, it signals all the group members to stop.
func (s *Squad) Run(fn func(context.Context) error) {
	s.RunGracefully(fn, nil)
}

// RunGracefully runs the backgroudFn. When fn is done, it signals all group members to stop.
// When stop signal has been received, squad run onDown function.
func (s *Squad) RunGracefully(backgroudFn func(context.Context) error, onDown func(context.Context) error) {
	if onDown != nil {
		s.cancellationFuncs = append(s.cancellationFuncs, onDown)
	}

	s.wg.Add(1)

	go func() {
		defer func() {
			s.cancel()
			s.wg.Done()
		}()

		if err := backgroudFn(s.ctx); err != nil {
			s.appendErr(err)
		}
	}()
}

// Wait blocks until all squad members exit.
func (s *Squad) Wait() []error {
	s.wg.Wait()

	return s.errs
}

func (s *Squad) appendErr(err error) {
	s.mtx.Lock()
	s.errs = append(s.errs, err)
	s.mtx.Unlock()
}

func (s *Squad) shutdown(ctx context.Context) {
	for _, cancelFn := range s.cancellationFuncs {
		go func(cancelFn func(ctx context.Context) error) {
			err := cancelFn(ctx)
			if err != nil {
				s.appendErr(err)
			}
		}(cancelFn)
	}
}

// NewSquad returns a new Squad with the context.
func NewSquad(ctx context.Context, opts ...Option) (*Squad, error) {
	ctx, cancel := context.WithCancel(ctx)
	squad := &Squad{
		ctx:               ctx,
		cancel:            cancel,
		cancellationDelay: defaultCancellationDelay,
	}

	for _, opt := range opts {
		opt(squad)
	}

	group, bootstrapCtx := errgroup.WithContext(ctx)
	for _, fn := range squad.bootstraps {
		fn := fn

		group.Go(func() error { return fn(bootstrapCtx) })
	}

	err := group.Wait()
	if err != nil {
		return nil, err
	}

	for _, f := range squad.funcs {
		squad.Run(f)
	}

	// launching in the background listener for a graceful shutdown.
	squad.wg.Add(1)
	go func() {
		defer squad.wg.Done()

		<-ctx.Done()

		ctx, cancel := context.WithTimeout(context.Background(), squad.cancellationDelay)
		defer cancel()

		if squad.cancellationFuncs != nil {
			squad.shutdown(ctx)
		}

		<-ctx.Done()
	}()

	return squad, nil
}
