package squad

import (
	"context"
	"time"
)

// Option is an option that can be applied to Squad.
type Option func(*Squad)

// WithGracefulPeriod sets graceful shutdown period to signal handler.
func WithGracefulPeriod(gracePeriod time.Duration) Option {
	return func(s *Squad) {
		s.shutdownGracefulTimeout = gracePeriod
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
		s.shutdownFuncs = append(s.shutdownFuncs, fns...)
	}
}

// WithSubsystem is Squad option that add init and cleanup functions
// for given subsystem will be executed before and after squad ran.
func WithSubsystem(initFn, closeFn func(context.Context) error) Option {
	return func(s *Squad) {
		s.bootstraps = append(s.bootstraps, initFn)
		s.shutdownFuncs = append(s.shutdownFuncs, closeFn)
	}
}
