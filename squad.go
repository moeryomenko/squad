// ███████╗ ██████╗ ██╗   ██╗ █████╗ ██████╗
// ██╔════╝██╔═══██╗██║   ██║██╔══██╗██╔══██╗
// ███████╗██║   ██║██║   ██║███████║██║  ██║
// ╚════██║██║▄▄ ██║██║   ██║██╔══██║██║  ██║
// ███████║╚██████╔╝╚██████╔╝██║  ██║██████╔╝
// ╚══════╝ ╚══▀▀═╝  ╚═════╝ ╚═╝  ╚═╝╚═════╝
//
// The APACHE License (APACHE)
//
// Copyright (c) 2023 Maxim Eryomenko <moeryomenko at gmail dot com>. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// 	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package squad contains a shared shutdown primitive for goroutine lifecycle
// management. It provides coordinated startup, error propagation, and graceful
// shutdown with OS signal handling.
package squad

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"
)

// ctxGroup is a replacement for synx.CtxGroup.
// It collects all errors via errors.Join under a mutex and wraps
// every fn call in panic-recovery via graceful.
//
//nolint:containedctx // ctxGroup is the owner of the context, managing goroutine lifecycle.
type ctxGroup struct {
	ctx context.Context
	wg  sync.WaitGroup
	mu  sync.Mutex
	err error
}

func newCtxGroup(ctx context.Context) *ctxGroup {
	return &ctxGroup{ctx: ctx}
}

func (g *ctxGroup) Go(fn func(context.Context) error) {
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		if err := graceful(g.ctx, fn); err != nil {
			g.mu.Lock()
			g.err = errors.Join(g.err, err)
			g.mu.Unlock()
		}
	}()
}

func (g *ctxGroup) Wait() error {
	g.wg.Wait()
	return g.err
}

// graceful wraps fn in panic-recovery, converting panics to errors.
func graceful(ctx context.Context, fn func(context.Context) error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()
	return fn(ctx)
}

// callWithContext runs fn in a goroutine and races it against ctx.Done().
// The caller MUST provide a context with a deadline.
func callWithContext(ctx context.Context, fn func(context.Context) error) error {
	result := make(chan error, 1)
	go func() {
		defer close(result)
		result <- graceful(ctx, fn)
	}()
	select {
	case err := <-result:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// defaultContextGracePeriod is default grace period.
// see: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#pod-termination
const defaultContextGracePeriod = 30 * time.Second

// Squad manages a group of goroutines that start together and stop together.
// If one goroutine exits with an error, all other goroutines are signalled
// to shut down gracefully. Squad provides OS signal handling, bootstrap
// lifecycle, and cleanup functions out of the box.
//
//nolint:containedctx // Squad is the owner of the context, managing the lifecycle of all child goroutines.
type Squad struct {
	// primitives for control running goroutines.
	wg     *ctxGroup
	ctx    context.Context
	cancel func()

	// primitives for control goroutines shutdowning.
	shutdownGracefulTimeout time.Duration
	shutdownFuncs           []func(ctx context.Context) error

	// bootstrap functions.
	bootstraps []func(context.Context) error

	// custom signal set for signal.Notify (nil = default SIGINT/SIGTERM/SIGHUP).
	signals []os.Signal

	// custom second signal handler (nil = os.Exit(1)).
	secondSignal func()

	// cause is the first error that caused shutdown, set during Wait().
	cause error
}

// New creates a Squad, runs bootstrap functions, and starts the signal handler.
// If any bootstrap function returns an error, the context is cancelled and the
// error is returned.
func New(opts ...Option) (*Squad, error) {
	ctx, cancel := context.WithCancel(context.Background())
	squad := &Squad{
		ctx:                     ctx,
		cancel:                  cancel,
		shutdownGracefulTimeout: defaultContextGracePeriod,
		wg:                      newCtxGroup(ctx),
	}

	for _, opt := range opts {
		opt(squad)
	}

	if err := runBootstrap(ctx, squad.bootstraps...); err != nil {
		cancel() // Cancel context on bootstrap failure
		return nil, err
	}

	// Start signal handler after successful bootstrap
	squad.startSignalHandler()

	return squad, nil
}

// Run runs the fn. When fn is done, it signals all the group members to stop.
func (s *Squad) Run(fn func(context.Context) error) {
	s.RunGracefully(fn, nil)
}

// RunGracefully runs the backgroundFn. When fn is done, it signals all group members to stop.
// When stop signal has been received, squad run onDown function.
func (s *Squad) RunGracefully(backgroundFn, onDown func(context.Context) error) {
	if onDown != nil {
		s.shutdownFuncs = append(s.shutdownFuncs, onDown)
	}

	s.wg.Go(backgroundFn)
}

// RunServer runs an http.Server with graceful shutdown on stop. When the squad
// shuts down, the server is shut down via Shutdown with the graceful timeout context.
func (s *Squad) RunServer(srv *http.Server) {
	s.RunGracefully(func(ctx context.Context) error {
		err := srv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("listen server: %w", err)
		}

		return nil
	}, func(ctx context.Context) error {
		err := srv.Shutdown(ctx)
		if err != nil {
			return fmt.Errorf("shutdown server: %w", err)
		}

		return nil
	})
}

// Done returns a channel that is closed when the squad's context is cancelled.
func (s *Squad) Done() <-chan struct{} {
	return s.ctx.Done()
}

// Go starts a fire-and-forget goroutine. Not tracked for errors; use
// ctx.Done() for shutdown notification.
func (s *Squad) Go(fn func()) {
	go fn()
}

// Wait blocks until all goroutines exit, runs all shutdown functions, and
// returns any errors encountered. After Wait returns, call Cause to obtain
// the shutdown reason.
func (s *Squad) Wait() error {
	var err error

	waitErr := s.wg.Wait()
	if waitErr != nil {
		s.cause = waitErr
		err = errors.Join(err, waitErr)
	} else if s.ctx.Err() != nil {
		s.cause = context.Canceled
	}

	shutdownErr := s.shutdown()
	if shutdownErr != nil {
		err = errors.Join(err, shutdownErr)
	}

	return err
}

// Cause returns the first error that caused shutdown, or context.Canceled if
// shutdown was triggered by a signal. It is safe to call after Wait() returns.
func (s *Squad) Cause() error {
	return s.cause
}

func (s *Squad) shutdown() error {
	if len(s.shutdownFuncs) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.WithoutCancel(s.ctx), s.shutdownGracefulTimeout)
	defer cancel()

	g, _ := errgroup.WithContext(ctx)
	for _, cancelFn := range s.shutdownFuncs {
		if cancelFn != nil {
			cf := cancelFn
			g.Go(func() error {
				return callWithContext(ctx, cf)
			})
		}
	}

	return g.Wait()
}

// startSignalHandler starts a goroutine that listens for OS signals
// and handles graceful shutdown. On first signal, it cancels the context
// to trigger graceful shutdown. On second signal, it forces exit.
func (s *Squad) startSignalHandler() {
	sigChan := make(chan os.Signal, 1)

	// Use custom signals if provided, otherwise default to SIGINT, SIGTERM, SIGHUP.
	sigs := s.signals
	if len(sigs) == 0 {
		sigs = []os.Signal{syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP}
	}
	signal.Notify(sigChan, sigs...)

	go func() {
		select {
		case <-sigChan:
			// Safely cancel the context
			s.cancel()
		case <-s.ctx.Done():
			return
		}

		// Wait for second signal or context timeout
		select {
		case <-sigChan:
			if s.secondSignal != nil {
				s.secondSignal()
			} else {
				os.Exit(1)
			}
		case <-time.After(s.shutdownGracefulTimeout + 5*time.Second):
			return
		}
	}()
}

func runBootstrap(ctx context.Context, bootstraps ...func(context.Context) error) error {
	if len(bootstraps) == 0 {
		return nil
	}

	g, gctx := errgroup.WithContext(ctx)
	for _, fn := range bootstraps {
		if fn == nil {
			continue
		}
		f := fn
		g.Go(func() error {
			return graceful(gctx, f)
		})
	}

	return g.Wait()
}
