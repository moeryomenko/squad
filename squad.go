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
	wg             sync.WaitGroup
	closeOnce      sync.Once
	ctx            context.Context
	gracefulPeriod time.Duration
	cancel         func()
	funcs          []func(ctx context.Context) error

	// primitives for control goroutines shutdowning.
	cancellationDelay time.Duration
	cancellationFuncs []func(ctx context.Context) error

	// bootstrap functions.
	bootstraps []func(context.Context) error

	// guarded errors.
	mtx  sync.Mutex
	errs []error
}

// Run runs the fn. When fn is done, it signals all the group members to stop.
func (s *Squad) Run(fn func(context.Context) error) {
	s.RunGracefully(fn, nil)
}

// RunGracefully runs the backgroudFn. When fn is done, it signals all group members to stop.
// When stop signal has been received, squad run onDown function.
func (s *Squad) RunGracefully(backgroudFn, onDown func(context.Context) error) {
	if onDown != nil {
		s.cancellationFuncs = append(s.cancellationFuncs, onDown)
	}

	s.wg.Add(1)

	go func() {
		defer func() {
			s.closeOnce.Do(s.cancel)
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
	s.wg.Add(len(s.cancellationFuncs))
	for _, cancelFn := range s.cancellationFuncs {
		go func(cancelFn func(ctx context.Context) error) {
			defer s.wg.Done()

			var err error
			select {
			case <-ctx.Done():
				return
			case err = <-callTimeout(ctx, cancelFn):
			}
			if err != nil {
				s.appendErr(err)
			}
		}(cancelFn)
	}
}

func callTimeout(ctx context.Context, fn func(context.Context) error) chan error {
	ch := make(chan error, 1)

	go func() {
		ch <- fn(ctx)
	}()

	return ch
}

// NewSquad returns a new Squad with the context.
func NewSquad(ctx context.Context, opts ...Option) (*Squad, error) {
	squad := &Squad{
		gracefulPeriod:    defaultContextCancellation,
		cancellationDelay: defaultCancellationDelay,
	}
	for _, opt := range opts {
		opt(squad)
	}

	squad.ctx, squad.cancel = WithGracefulPeriod(ctx, squad.gracefulPeriod)

	if err := onStart(ctx, squad.bootstraps...); err != nil {
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

func onStart(ctx context.Context, bootstraps ...func(context.Context) error) error {
	if len(bootstraps) == 0 {
		return nil
	}

	group, bootstrapCtx := errgroup.WithContext(ctx)
	for _, fn := range bootstraps {
		fn := fn
		if fn == nil {
			continue
		}

		group.Go(func() error { return fn(bootstrapCtx) })
	}

	err := group.Wait()
	if err != nil {
		return err
	}
	return nil
}

const (
	defaultCancellationDelay   = 2 * time.Second
	defaultContextCancellation = 30 * time.Second
)
