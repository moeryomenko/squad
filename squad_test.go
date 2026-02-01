package squad_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/moeryomenko/squad"
)

func TestSquad(t *testing.T) {
	errInit := errors.New("init failed")
	errTask := errors.New("failed task")

	testcases := []struct {
		name           string
		giveBootstrap  func(context.Context) error
		giveBackground [2]func(context.Context) error
		wantStart      bool
		wantErr        error
	}{
		{
			name:          "basic case",
			giveBootstrap: nil,
			wantStart:     true,
			giveBackground: [2]func(context.Context) error{
				func(ctx context.Context) error { return nil },
				func(ctx context.Context) error { return nil },
			},
			wantErr: nil,
		},
		{
			name:          "failed on start",
			giveBootstrap: func(ctx context.Context) error { return errInit },
			wantStart:     false,
		},
		{
			name:          "background task failed",
			giveBootstrap: nil,
			wantStart:     true,
			giveBackground: [2]func(context.Context) error{
				func(ctx context.Context) error {
					<-time.After(200 * time.Millisecond)
					return errTask
				},
				func(ctx context.Context) error { return nil },
			},
			wantErr: errors.Join(errors.Join(errTask)),
		},
		{
			name:          "failed shutdown",
			giveBootstrap: nil,
			wantStart:     true,
			giveBackground: [2]func(context.Context) error{
				func(ctx context.Context) error {
					<-time.After(200 * time.Millisecond)
					return nil
				},
				func(ctx context.Context) error {
					return errTask
				},
			},
			wantErr: errors.Join(errTask),
		},
		{
			name:          "up and down failed",
			giveBootstrap: nil,
			wantStart:     true,
			giveBackground: [2]func(context.Context) error{
				func(ctx context.Context) error {
					return errTask
				},
				func(ctx context.Context) error {
					return errTask
				},
			},
			wantErr: errors.Join(errors.Join(errTask), errTask),
		},
		{
			name:          "up failed and down failed by timeout",
			giveBootstrap: nil,
			wantStart:     true,
			giveBackground: [2]func(context.Context) error{
				func(ctx context.Context) error {
					return errTask
				},
				func(ctx context.Context) error {
					<-time.After(300 * time.Millisecond)
					return errTask
				},
			},
			wantErr: errors.Join(errors.Join(errTask), context.DeadlineExceeded),
		},
	}

	t.Parallel()
	for _, testcase := range testcases {
		tc := testcase
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			testGroup, err := squad.New(
				squad.WithGracefulPeriod(100*time.Millisecond),
				squad.WithBootstrap(tc.giveBootstrap),
			)
			if tc.wantStart {
				if err != nil {
					t.Errorf("New() error = %v, want nil", err)
				}
			} else {
				if err == nil {
					t.Errorf("New() error = nil, want non-nil")
				}
				return
			}

			testGroup.RunGracefully(tc.giveBackground[0], tc.giveBackground[1])

			err = testGroup.Wait()
			if tc.wantErr == nil {
				if err != nil {
					t.Errorf("Wait() error = %v, want nil", err)
				}

				return
			}

			if diff := cmp.Diff(tc.wantErr.Error(), err.Error()); diff != "" {
				t.Errorf("Wait() error mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
