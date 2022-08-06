package squad

import (
	"context"
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
// goroutine to the squad. This goroutine will exit on SIGINT or SIGHUP
// or SIGTERM or SIGQUIT and trigger cancellation of the whole squad.
// Also replace squad context by delayed context.
func WithSignalHandler(customDelay ...time.Duration) Option {
	delay := defaultContextGracePeriod
	if len(customDelay) != 0 {
		delay = customDelay[0]
	}
	return func(squad *Squad) {
		go handleSignals(delay, squad.cancel)
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
