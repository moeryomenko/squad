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
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/moeryomenko/synx"
)

// defaultContextGracePeriod is default grace period.
// see: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#pod-termination
const defaultContextGracePeriod = 30 * time.Second

// Squad is a collection of goroutines that go up and running altogether.
// If one goroutine exits, other goroutines also go down.
type Squad struct {
	// primitives for control running goroutines.
	wg     *synx.CtxGroup
	ctx    context.Context
	cancel func()

	// primitives for control goroutines shutdowning.
	shutdownGracefulTimeout time.Duration
	shutdownFuncs           []func(ctx context.Context) error

	// bootstrap functions.
	bootstraps []func(context.Context) error
}

// New returns a new Squad with the context.
func New(opts ...Option) (*Squad, error) {
	ctx, cancel := context.WithCancel(context.Background())
	squad := &Squad{
		ctx:                     ctx,
		cancel:                  cancel,
		shutdownGracefulTimeout: defaultContextGracePeriod,
		wg:                      synx.NewCtxGroup(ctx),
	}

	for _, opt := range opts {
		opt(squad)
	}

	squad.startSignalHandler()

	if err := onStart(ctx, squad.bootstraps...); err != nil {
		return nil, err
	}

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

// Wait blocks until all squad members exit.
func (s *Squad) Wait() error {
	var err error

	waitErr := s.wg.Wait()
	if waitErr != nil {
		err = errors.Join(err, waitErr)
	}

	shutdownErr := s.shutdown()
	if shutdownErr != nil {
		err = errors.Join(err, shutdownErr)
	}

	return err
}

func (s *Squad) shutdown() error {
	if len(s.shutdownFuncs) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.WithoutCancel(s.ctx), s.shutdownGracefulTimeout)
	defer cancel()

	group := synx.NewErrGroup(ctx)
	for _, cancelFn := range s.shutdownFuncs {
		group.Go(func(ctx context.Context) error {
			return synx.CallWithContext(ctx, cancelFn)
		})
	}

	return group.Wait()
}

// startSignalHandler starts a goroutine that listens for OS signals
// and handles graceful shutdown. On first signal, it cancels the context
// to trigger graceful shutdown. On second signal, it forces exit.
func (s *Squad) startSignalHandler() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	go func() {
		defer signal.Stop(sigChan)

		select {
		case <-sigChan:
			s.cancel()
		case <-s.ctx.Done():
			return
		}

		// Wait for second signal or context timeout
		select {
		case <-sigChan:
			os.Exit(1)
		case <-time.After(s.shutdownGracefulTimeout + 5*time.Second):
			return
		}
	}()
}

func onStart(ctx context.Context, bootstraps ...func(context.Context) error) error {
	if len(bootstraps) == 0 {
		return nil
	}

	group := synx.NewErrGroup(ctx)
	for _, fn := range bootstraps {
		if fn == nil {
			continue
		}
		group.Go(fn)
	}

	return group.Wait()
}
