package squad

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSquad(t *testing.T) {
	errInit := errors.New("init failed")
	errTask := errors.New("failed task")

	testcases := []struct {
		name        string
		bootstraps  func(context.Context) error
		background  [2]func(context.Context) error
		shouldStart bool
		errs        []error
	}{
		{
			name:        "basic case",
			bootstraps:  nil,
			shouldStart: true,
			background: [2]func(context.Context) error{
				func(ctx context.Context) error { return nil },
				func(ctx context.Context) error { return nil },
			},
			errs: nil,
		},
		{
			name:        "failed on start",
			bootstraps:  func(ctx context.Context) error { return errInit },
			shouldStart: false,
		},
		{
			name:        "backgroud task faield",
			bootstraps:  nil,
			shouldStart: true,
			background: [2]func(context.Context) error{
				func(ctx context.Context) error {
					<-time.After(200 * time.Millisecond)
					return errTask
				},
				func(ctx context.Context) error { return nil },
			},
			errs: []error{errTask},
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
			errs: []error{errTask},
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
			errs: []error{errTask, errTask},
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
			errs: []error{errTask},
		},
	}

	for _, testcase := range testcases {
		tc := testcase
		t.Run(tc.name, func(t *testing.T) {
			testGroup, err := New(context.Background(), WithShutdownDelay(100*time.Millisecond), WithBootstrap(tc.bootstraps))
			if tc.shouldStart {
				assert.NoError(t, err)
			} else {
				assert.NotNil(t, err)
				return
			}

			testGroup.RunGracefully(tc.background[0], tc.background[1])

			errs := testGroup.Wait()
			assert.Equal(t, tc.errs, errs)
		})
	}
}
