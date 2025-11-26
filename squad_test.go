package squad_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/moeryomenko/squad"
)

func TestSquad(t *testing.T) {
	errInit := errors.New("init failed")
	errTask := errors.New("failed task")

	testcases := []struct {
		name        string
		bootstraps  func(context.Context) error
		background  [2]func(context.Context) error
		shouldStart bool
		err         error
	}{
		{
			name:        "basic case",
			bootstraps:  nil,
			shouldStart: true,
			background: [2]func(context.Context) error{
				func(ctx context.Context) error { return nil },
				func(ctx context.Context) error { return nil },
			},
			err: nil,
		},
		{
			name:        "failed on start",
			bootstraps:  func(ctx context.Context) error { return errInit },
			shouldStart: false,
		},
		{
			name:        "background task faield",
			bootstraps:  nil,
			shouldStart: true,
			background: [2]func(context.Context) error{
				func(ctx context.Context) error {
					<-time.After(200 * time.Millisecond)
					return errTask
				},
				func(ctx context.Context) error { return nil },
			},
			err: errors.Join(errors.Join(errTask)),
		},
		{
			name:        "failed shutdown",
			bootstraps:  nil,
			shouldStart: true,
			background: [2]func(context.Context) error{
				func(ctx context.Context) error {
					<-time.After(200 * time.Millisecond)
					return nil
				},
				func(ctx context.Context) error {
					return errTask
				},
			},
			err: errors.Join(errTask),
		},
		{
			name:        "up and down failed",
			bootstraps:  nil,
			shouldStart: true,
			background: [2]func(context.Context) error{
				func(ctx context.Context) error {
					return errTask
				},
				func(ctx context.Context) error {
					return errTask
				},
			},
			err: errors.Join(errors.Join(errTask), errTask),
		},
		{
			name:        "up failed and down failed by timeout",
			bootstraps:  nil,
			shouldStart: true,
			background: [2]func(context.Context) error{
				func(ctx context.Context) error {
					return errTask
				},
				func(ctx context.Context) error {
					<-time.After(300 * time.Millisecond)
					return errTask
				},
			},
			err: errors.Join(errors.Join(errTask), context.DeadlineExceeded),
		},
	}

	t.Parallel()
	for _, testcase := range testcases {
		tc := testcase
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			testGroup, err := squad.New(
				squad.WithSignalHandler(squad.WithShutdownTimeout(100*time.Millisecond)),
				squad.WithBootstrap(tc.bootstraps),
			)
			if tc.shouldStart {
				assert.NoError(t, err)
			} else {
				assert.NotNil(t, err)
				return
			}

			testGroup.RunGracefully(tc.background[0], tc.background[1])

			err = testGroup.Wait()
			if tc.err != nil {
				assert.EqualError(t, tc.err, err.Error())
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func TestHTTPServerGracefulShutdown(t *testing.T) {
	// Setup test server
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		fmt.Fprintf(w, "Hello")
	})

	srv := &http.Server{
		Addr:    ":9090",
		Handler: handler,
	}

	s, _ := squad.New(
		squad.WithSignalHandler(
			squad.WithGracefulPeriod(300 * time.Millisecond),
		),
	)

	// Start server
	s.RunServer(srv)

	// Start client requests
	var wg sync.WaitGroup
	var successCount int32

	for range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := http.Get("http://localhost:9090")
			if err == nil && resp.StatusCode == http.StatusOK {
				atomic.AddInt32(&successCount, 1)
			}
		}()
	}

	// Trigger shutdown after starting requests
	go func() {
		time.Sleep(50 * time.Millisecond)
		syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
	}()

	// Wait for shutdown
	err := s.Wait()
	wg.Wait()

	// Verify in-flight requests completed
	assert.NoError(t, err)
	assert.Equal(t, int32(5), successCount, "All in-flight requests should complete")
}
