package squad

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/pprof"
	"os/signal"
	"syscall"
	"time"
)

// Option is an option that can be applied to Squad.
type Option func(*Squad)

// WithShutdownDelay sets time for cancellation timeout.
// Default timeout is 2 seconds.
func WithShutdownDelay(t time.Duration) Option {
	return func(squad *Squad) {
		squad.cancellationDelay = t
	}
}

// WithSignalHandler is a Squad option that adds signal handling
// goroutine to the squad. This goroutine will exit on SIGINT or SIGTERM
// or SIGQUIT and trigger cancellation of the whole squad.
func WithSignalHandler() Option {
	return func(squad *Squad) {
		squad.funcs = append(squad.funcs, handleSignals)
	}
}

// WithBootstrap is a Squad option that adds bootstrap functions,
// which will be executed before squad started.
func WithBootstrap(fns ...func(context.Context) error) Option {
	return func(s *Squad) {
		s.bootstraps = fns
	}
}

// WithCloses is a Squad options that adds cleanup functions,
// which will be executed after squad stopped.
func WithCloses(fns ...func(context.Context) error) Option {
	return func(s *Squad) {
		s.cancellationFuncs = append(s.cancellationFuncs, fns...)
	}
}

// WithProfileHandler is a Squad option that adds pprof handling
// goroutine to squad. This goroutine launches the http/pprof server.
func WithProfileHandler(port int) Option {
	return func(squad *Squad) {
		runFn, onDownFn := profileHandler(port)
		squad.funcs = append(squad.funcs, runFn)
		squad.cancellationFuncs = append(squad.cancellationFuncs, onDownFn)
	}
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
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer stop()

	<-ctx.Done()

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
