// Package squad contains a shared shutdown primitive.
package squad

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

const defaultCancellationDelay = 2 * time.Second

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

	// guarded errors.
	mtx  sync.Mutex
	errs []error
}

// Run runs the fn. When fn is done, it signals all the group members to stop.
func (s *Squad) Run(fn func(context.Context) error) {
	s.RunGracefully(fn, nil)
}

// RunGracefully runs the fn. When fn is done, it signals all group members to stop.
// When stop signal has been received, squad run onDown function.
func (s *Squad) RunGracefully(fn func(context.Context) error, onDown func(context.Context) error) {
	if onDown != nil {
		s.cancellationFuncs = append(s.cancellationFuncs, onDown)
	}

	s.wg.Add(1)
	go func() {
		defer func() {
			s.cancel()
			s.wg.Done()
		}()

		if err := fn(s.ctx); err != nil {
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

func (s *Squad) shutdown() {
	cancellationContext, cancel := context.WithTimeout(context.Background(), s.cancellationDelay)
	defer cancel()

	for _, cancelFn := range s.cancellationFuncs {
		go func(cancelFn func(ctx context.Context) error) {
			err := cancelFn(cancellationContext)
			if err != nil {
				s.appendErr(err)
			}
		}(cancelFn)
	}

	<-cancellationContext.Done()

	return
}

// NewSquad returns a new Squad with the context.
func NewSquad(ctx context.Context, opts ...SquadOption) *Squad {
	ctx, cancel := context.WithCancel(ctx)
	squad := &Squad{
		wg:                sync.WaitGroup{},
		ctx:               ctx,
		cancel:            cancel,
		cancellationDelay: defaultCancellationDelay,
	}

	for _, opt := range opts {
		opt(squad)
	}

	for _, f := range squad.funcs {
		squad.Run(f)
	}

	// launching in the background listener for a graceful shutdown.
	squad.wg.Add(1)
	go func() {
		defer squad.wg.Done()

		<-ctx.Done()
		if squad.cancellationFuncs != nil {
			squad.shutdown()
		}
	}()

	return squad
}

// SquadOption is an option that can be applied to Squad.
type SquadOption func(*Squad)

// WithShutdownDelay sets time for cancellation timeout.
// Default timeout is 2 seconds.
func WithShutdownDelay(t time.Duration) SquadOption {
	return func(squad *Squad) {
		squad.cancellationDelay = t
	}
}

// WithSignalHandler is a Squad option that adds signal handling
// goroutine to the squad. This goroutine will exit on SIGINT or SIGTERM
// or SIGQUIT and trigger cancellation of the whole squad.
func WithSignalHandler() SquadOption {
	return func(squad *Squad) {
		squad.funcs = append(squad.funcs, handleSignals)
	}
}

// WithHealthHandler is a Squad option that adds health handling
// goroutine to the squad. This goroutine launches the health http server,
// which, if the squad stops working, will be a signal to external services.
func WithHealthHandler(port int) SquadOption {
	return func(squad *Squad) {
		runFn, onDownFn := healthHandler(port)
		squad.funcs = append(squad.funcs, runFn)
		squad.cancellationFuncs = append(squad.cancellationFuncs, onDownFn)
	}
}

// WithProfileHandler is a Squad option that adds pprof handling
// goroutine to squad. This goroutine launches the http/pprof server.
func WithProfileHandler(port int) SquadOption {
	return func(squad *Squad) {
		runFn, onDownFn := profileHandler(port)
		squad.funcs = append(squad.funcs, runFn)
		squad.cancellationFuncs = append(squad.cancellationFuncs, onDownFn)
	}
}

func healthHandler(port int) (func(context.Context) error, func(context.Context) error) {
	router := http.NewServeMux()
	// empty handler default return 200 OK.
	router.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {})
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: router,
	}
	return func(_ context.Context) error {
		return srv.ListenAndServe()
	}, shutdownServer(srv)
}

func profileHandler(port int) (func(context.Context) error, func(context.Context) error) {
	router := http.NewServeMux()
	router.HandleFunc("/debug/pprof/", pprof.Index)
	router.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	router.HandleFunc("/debug/pprof/profile", pprof.Profile)
	router.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	router.HandleFunc("/debug/pprof/trace", pprof.Trace)
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: router,
	}

	return func(_ context.Context) error {
		return srv.ListenAndServe()
	}, shutdownServer(srv)
}

func handleSignals(ctx context.Context) error {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer signal.Stop(sigs)

	select {
	case <-sigs:
	case <-ctx.Done():
	}

	return nil
}

func shutdownServer(srv *http.Server) func(context.Context) error {
	return func(ctx context.Context) error {
		if err := srv.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}

		return nil
	}
}
