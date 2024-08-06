package squad

import (
	"context"
	"os/signal"
	"syscall"
	"time"
)

// Option is an option that can be applied to Squad.
type Option func(*Squad)

// ShutdownOpt is an options that can be applied to signal handler.
type ShutdownOpt func(*shutdown)

// WithGracefulPeriod sets graceful shutdown period to signal handler.
func WithGracefulPeriod(period time.Duration) ShutdownOpt {
	return func(s *shutdown) {
		s.gracefulPeriod = period
	}
}

// WithShutdownTimeout sets timeout for reserves time for the release of resource.
func WithShutdownTimeout(timeout time.Duration) ShutdownOpt {
	return func(s *shutdown) {
		s.shutdownTimeout = timeout
	}
}

// WithShutdownInGracePriod sets timeout for shutdown process which will be run immediatly in grace period.
func WithShutdownInGracePriod(timeout time.Duration) ShutdownOpt {
	return func(s *shutdown) {
		s.gracefulPeriod = timeout
		s.shutdownTimeout = timeout
	}
}

// WithSignalHandler is a Squad option that adds signal handling
// goroutine to the squad. This goroutine will exit on SIGINT or SIGHUP
// or SIGTERM or SIGQUIT with graceful timeount and reserves
// time for the release of resources.
func WithSignalHandler(opts ...ShutdownOpt) Option {
	config := shutdown{
		gracefulPeriod:  defaultContextGracePeriod,
		shutdownTimeout: defaultCancellationDelay,
	}

	for _, opt := range opts {
		opt(&config)
	}
	return func(squad *Squad) {
		squad.cancellationDelay = config.shutdownTimeout
		squad.serverContext = handleSignals(config.delay(), squad.cancel)
	}
}

// WithBootstrap is a Squad option that adds bootstrap functions,
// which will be executed before squad started.
func WithBootstrap(fns ...func(context.Context) error) Option {
	return func(s *Squad) {
		for _, fn := range fns {
			if fn == nil {
				continue
			}
			s.bootstraps = append(s.bootstraps, fn)
		}
	}
}

// WithCloses is a Squad options that adds cleanup functions,
// which will be executed after squad stopped.
func WithCloses(fns ...func(context.Context) error) Option {
	return func(s *Squad) {
		s.cancellationFuncs = append(s.cancellationFuncs, fns...)
	}
}

// WithSubsystem is Squad option that add init and cleanup functions
// for given subsystem witll be executed before and after squad ran.
func WithSubsystem(initFn, closeFn func(context.Context) error) Option {
	return func(s *Squad) {
		s.bootstraps = append(s.bootstraps, initFn)
		s.cancellationFuncs = append(s.cancellationFuncs, closeFn)
	}
}

func handleSignals(delay time.Duration, cancel func()) context.Context {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGHUP, syscall.SIGTERM, syscall.SIGQUIT)

	go func() {
		defer stop()
		<-ctx.Done()
		// NOTE: After receiving signal shut down server, and
		// wait while all active request and operations complete,
		// after delay cancel squad context.
		<-time.After(delay)
		cancel()
	}()

	return ctx
}

type shutdown struct {
	gracefulPeriod  time.Duration
	shutdownTimeout time.Duration
}

func (s *shutdown) delay() time.Duration {
	return s.gracefulPeriod - s.shutdownTimeout
}
