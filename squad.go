// Package squad contains a shared shutdown primitive.
package squad

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// Squad is a collection of goroutines that go up and running altogether.
// If one goroutine exits, other goroutines also go down.
type Squad struct {
	wg     sync.WaitGroup
	ctx    context.Context
	cancel func()

	funcs []func(ctx context.Context) error
	errs  []error
}

// Run runs the fn. When fn is done, it signals all the group members to stop.
func (s *Squad) Run(fn func(context.Context) error) {
	s.wg.Add(1)

	go func() {
		defer func() {
			s.cancel()
			s.wg.Done()
		}()

		if err := fn(s.ctx); err != nil {
			s.errs = append(s.errs, err)
		}
	}()
}

// Wait blocks until all squad members exit.
func (s *Squad) Wait() []error {
	s.wg.Wait()

	return s.errs
}

// NewSquad returns a new Squad with the context.
func NewSquad(ctx context.Context, opts ...SquadOption) *Squad {
	ctx, cancel := context.WithCancel(ctx)
	squad := &Squad{
		wg:     sync.WaitGroup{},
		ctx:    ctx,
		cancel: cancel,
	}

	for _, opt := range opts {
		opt(squad)
	}

	for _, f := range squad.funcs {
		squad.Run(f)
	}

	return squad
}

// SquadOption is an option that can be applied to Squad.
type SquadOption func(*Squad)

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
		squad.funcs = append(squad.funcs, healthHandler(port))
	}
}

func healthHandler(port int) func(context.Context) error {
	router := http.NewServeMux()
	// empty handler default return 200 OK.
	router.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {})
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: router,
	}

	return func(ctx context.Context) (err error) {
		go srv.ListenAndServe()

		select {
		case <-ctx.Done():
			if err = srv.Shutdown(ctx); err == http.ErrServerClosed {
				err = nil
			}
		}
		return nil
	}
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
