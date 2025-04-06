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

// Package squad contains a shared shutdown primitive.
package squad

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/moeryomenko/synx"
)

const (
	defaultCancellationDelay = 2 * time.Second
	// defaultContextGracePeriod is default grace period.
	// see: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#pod-termination
	defaultContextGracePeriod = 30 * time.Second
)

// Squad is a collection of goroutines that go up and running altogether.
// If one goroutine exits, other goroutines also go down.
type Squad struct {
	// primitives for control running goroutines.
	wg                 *synx.CtxGroup
	ctx, serverContext context.Context
	cancel             func()

	// primitives for control goroutines shutdowning.
	cancellationDelay time.Duration
	cancellationFuncs []func(ctx context.Context) error

	// bootstrap functions.
	bootstraps []func(context.Context) error

	// guarded errors.
	mtx sync.Mutex
	err error
}

// New returns a new Squad with the context.
func New(opts ...Option) (*Squad, error) {
	ctx, cancel := context.WithCancel(context.Background())
	squad := &Squad{
		ctx:               ctx,
		cancel:            cancel,
		cancellationDelay: defaultCancellationDelay,
		wg:                synx.NewCtxGroup(ctx),
	}

	for _, opt := range opts {
		opt(squad)
	}

	if err := onStart(ctx, squad.bootstraps...); err != nil {
		return nil, err
	}

	return squad, nil
}

// RunServer is wrapper function for launch http server.
func (s *Squad) RunServer(srv *http.Server) {
	s.wg.Go(func(_ context.Context) error {
		err := srv.ListenAndServe()
		if err == nil || errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	})

	// NOTE: After receiving shutdowning signal first of all,
	// gracefully shuts down the server without interrupting any active connections.
	go func(ctx context.Context) {
		shutdownCtx := context.WithoutCancel(ctx)
		<-ctx.Done()
		err := srv.Shutdown(shutdownCtx)
		s.appendErr(err)
	}(s.serverContext)
}

// RunConsumer is wrapper function for run cosumer worker
// after receiving shutdowning signal stop context for consumer events/messages
// without interrupting any active handler.
func (s *Squad) RunConsumer(consumer ConsumerLoop) {
	s.wg.Go(func(ctx context.Context) error {
		return consumer(ctx, context.WithoutCancel(ctx))
	})
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

	s.wg.Go(backgroudFn)
}

// Wait blocks until all squad members exit.
func (s *Squad) Wait() error {
	err := s.wg.Wait()
	if err != nil {
		s.err = errors.Join(s.err, err)
	}
	err = s.shutdown()
	if err != nil {
		s.err = errors.Join(s.err, err)
	}
	return s.err
}

func (s *Squad) appendErr(err error) {
	s.mtx.Lock()
	s.err = errors.Join(s.err, err)
	s.mtx.Unlock()
}

func (s *Squad) shutdown() error {
	if len(s.cancellationFuncs) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.WithoutCancel(s.ctx), s.cancellationDelay)
	defer cancel()

	group := synx.NewErrGroup(ctx)
	for _, cancelFn := range s.cancellationFuncs {
		cancelFn := cancelFn
		group.Go(func(ctx context.Context) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case err := <-callTimeout(ctx, cancelFn):
				return err
			}
		})
	}

	return group.Wait()
}

func callTimeout(ctx context.Context, fn func(context.Context) error) chan error {
	ch := make(chan error, 1)

	go func() {
		ch <- fn(ctx)
	}()

	return ch
}

func onStart(ctx context.Context, bootstraps ...func(context.Context) error) error {
	if len(bootstraps) == 0 {
		return nil
	}

	group := synx.NewErrGroup(ctx)
	for _, fn := range bootstraps {
		group.Go(fn)
	}

	return group.Wait()
}
