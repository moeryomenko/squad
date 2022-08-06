package squad

import (
	"context"
	"os/signal"
	"syscall"
	"time"
)

// Option is an option that can be applied to Squad.
type Option func(*Squad)

// WithSignalHandler is a Squad option that adds signal handling
// goroutine to the squad. This goroutine will exit on SIGINT or SIGHUP
// or SIGTERM or SIGQUIT with graceful timeount and reserves
// time for the release of resources.
func WithSignalHandler(shutdownTimeout time.Duration, customDelay ...time.Duration) Option {
	delay := defaultContextGracePeriod
	if len(customDelay) != 0 {
		delay = customDelay[0]
	}
	return func(squad *Squad) {
		squad.cancellationDelay = shutdownTimeout
		go handleSignals(delay-shutdownTimeout, squad.cancel)
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

func handleSignals(delay time.Duration, cancel func()) {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGHUP, syscall.SIGTERM, syscall.SIGQUIT)
	defer stop()
	<-ctx.Done()
	<-time.After(delay)
	cancel()
}
