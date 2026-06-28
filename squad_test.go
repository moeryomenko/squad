package squad_test

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync"
	"syscall"
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
		giveBackground func(context.Context) error
		giveOnDown     func(context.Context) error
		wantStart      bool
		wantErr        error
	}{
		{
			name:           "basic case",
			giveBootstrap:  nil,
			wantStart:      true,
			giveBackground: func(ctx context.Context) error { return nil },
			giveOnDown:     func(ctx context.Context) error { return nil },
			wantErr:        nil,
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
			giveBackground: func(ctx context.Context) error {
				<-time.After(200 * time.Millisecond)
				return errTask
			},
			giveOnDown: func(ctx context.Context) error { return nil },
			wantErr:    errors.Join(errors.Join(errTask)),
		},
		{
			name:          "failed shutdown",
			giveBootstrap: nil,
			wantStart:     true,
			giveBackground: func(ctx context.Context) error {
				<-time.After(200 * time.Millisecond)
				return nil
			},
			giveOnDown: func(ctx context.Context) error {
				return errTask
			},
			wantErr: errors.Join(errTask),
		},
		{
			name:          "up and down failed",
			giveBootstrap: nil,
			wantStart:     true,
			giveBackground: func(ctx context.Context) error {
				return errTask
			},
			giveOnDown: func(ctx context.Context) error {
				return errTask
			},
			wantErr: errors.Join(errors.Join(errTask), errTask),
		},
		{
			name:          "up failed and down failed by timeout",
			giveBootstrap: nil,
			wantStart:     true,
			giveBackground: func(ctx context.Context) error {
				return errTask
			},
			giveOnDown: func(ctx context.Context) error {
				<-time.After(300 * time.Millisecond)
				return errTask
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

			testGroup.RunGracefully(tc.giveBackground, tc.giveOnDown)

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

func TestRun(t *testing.T) {
	t.Parallel()

	sq, err := squad.New(
		squad.WithGracefulPeriod(100 * time.Millisecond),
	)
	if err != nil {
		t.Fatal(err)
	}

	sq.Run(func(ctx context.Context) error { return nil })

	err = sq.Wait()
	if err != nil {
		t.Errorf("Wait() error = %v, want nil", err)
	}
}

func TestRunServer(t *testing.T) {
	t.Parallel()

	sq, err := squad.New(
		squad.WithGracefulPeriod(100 * time.Millisecond),
	)
	if err != nil {
		t.Fatal(err)
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	listener.Close()

	srv := &http.Server{Addr: addr, Handler: handler}
	sq.RunServer(srv)

	// Retry GET to handle race between listener.Close() and ListenAndServe.
	var resp *http.Response
	for range 5 {
		resp, err = http.Get("http://" + addr + "/")
		if err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	srv.Close()

	err = sq.Wait()
	if err != nil {
		t.Errorf("Wait() error = %v, want nil", err)
	}
}

func TestWithCloses(t *testing.T) {
	t.Parallel()

	var closed bool
	sq, err := squad.New(
		squad.WithGracefulPeriod(100*time.Millisecond),
		squad.WithCloses(func(ctx context.Context) error {
			closed = true
			return nil
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	sq.Run(func(ctx context.Context) error { return nil })

	err = sq.Wait()
	if err != nil {
		t.Errorf("Wait() error = %v, want nil", err)
	}
	if !closed {
		t.Error("close func was not executed")
	}
}

func TestWithSubsystem(t *testing.T) {
	t.Parallel()

	var (
		initDone  bool
		closeDone bool
	)
	sq, err := squad.New(
		squad.WithGracefulPeriod(100*time.Millisecond),
		squad.WithSubsystem(
			func(ctx context.Context) error {
				initDone = true
				return nil
			},
			func(ctx context.Context) error {
				closeDone = true
				return nil
			},
		),
	)
	if err != nil {
		t.Fatal(err)
	}

	if !initDone {
		t.Error("init function was not executed during New")
	}

	sq.Run(func(ctx context.Context) error { return nil })

	err = sq.Wait()
	if err != nil {
		t.Errorf("Wait() error = %v, want nil", err)
	}
	if !closeDone {
		t.Error("close function was not executed")
	}
}

func TestShutdownTimeout(t *testing.T) {
	t.Parallel()

	sq, err := squad.New(
		squad.WithGracefulPeriod(50 * time.Millisecond),
	)
	if err != nil {
		t.Fatal(err)
	}

	sq.RunGracefully(
		func(ctx context.Context) error { return nil },
		func(ctx context.Context) error {
			<-time.After(200 * time.Millisecond)
			return nil
		},
	)

	err = sq.Wait()
	if err == nil {
		t.Fatal("Wait() error = nil, want context.DeadlineExceeded")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Wait() error = %v, want context.DeadlineExceeded", err)
	}
}

func TestNilBootstrap(t *testing.T) {
	t.Parallel()

	sq, err := squad.New(
		squad.WithGracefulPeriod(100*time.Millisecond),
		squad.WithBootstrap(nil),
	)
	if err != nil {
		t.Errorf("New() with nil bootstrap error = %v, want nil", err)
	}

	sq.Run(func(ctx context.Context) error { return nil })

	err = sq.Wait()
	if err != nil {
		t.Errorf("Wait() error = %v, want nil", err)
	}
}

func TestMultipleTasksSuccess(t *testing.T) {
	t.Parallel()

	sq, err := squad.New(
		squad.WithGracefulPeriod(100 * time.Millisecond),
	)
	if err != nil {
		t.Fatal(err)
	}

	for range 3 {
		sq.Run(func(ctx context.Context) error { return nil })
	}

	err = sq.Wait()
	if err != nil {
		t.Errorf("Wait() error = %v, want nil", err)
	}
}

func TestNilOnDown(t *testing.T) {
	t.Parallel()

	sq, err := squad.New(
		squad.WithGracefulPeriod(100 * time.Millisecond),
	)
	if err != nil {
		t.Fatal(err)
	}

	sq.RunGracefully(
		func(ctx context.Context) error { return nil },
		nil,
	)

	err = sq.Wait()
	if err != nil {
		t.Errorf("Wait() error = %v, want nil", err)
	}
}

func TestDone(t *testing.T) {
	sq, err := squad.New(
		squad.WithGracefulPeriod(100*time.Millisecond),
		squad.WithSignals(syscall.SIGUSR1),
	)
	if err != nil {
		t.Fatal(err)
	}

	// Done() should return a non-nil channel before shutdown.
	done := sq.Done()
	if done == nil {
		t.Error("Done() returned nil before shutdown")
	}

	// Channel should be open (not yet closed).
	select {
	case <-done:
		t.Error("Done() channel closed before cancellation")
	default:
	}

	sq.Run(func(ctx context.Context) error {
		<-ctx.Done()
		return nil
	})

	// Send SIGUSR1 to trigger cancellation via the signal handler.
	if err := syscall.Kill(syscall.Getpid(), syscall.SIGUSR1); err != nil {
		t.Fatalf("Kill() error = %v", err)
	}

	err = sq.Wait()
	if err != nil {
		t.Logf("Wait() returned error: %v", err)
	}

	// After cancellation and Wait(), Done() channel should be closed.
	select {
	case <-done:
		// Expected — channel is closed.
	default:
		t.Error("Done() channel still open after Wait()")
	}
}

func TestGo(t *testing.T) {
	t.Parallel()

	sq, err := squad.New(
		squad.WithGracefulPeriod(100 * time.Millisecond),
	)
	if err != nil {
		t.Fatal(err)
	}

	var (
		flag bool
		wg   sync.WaitGroup
	)
	wg.Add(1)
	sq.Go(func() {
		flag = true
		wg.Done()
	})

	wg.Wait()

	if !flag {
		t.Error("goroutine launched with Go() did not set flag")
	}

	// Wait for squad to finish (no tasks registered).
	err = sq.Wait()
	if err != nil {
		t.Errorf("Wait() error = %v, want nil", err)
	}
}

func TestCauseNilBeforeWait(t *testing.T) {
	t.Parallel()

	sq, err := squad.New(
		squad.WithGracefulPeriod(100 * time.Millisecond),
	)
	if err != nil {
		t.Fatal(err)
	}

	// Before Wait(), Cause() should return nil.
	if cause := sq.Cause(); cause != nil {
		t.Errorf("Cause() before Wait() = %v, want nil", cause)
	}

	sq.Run(func(ctx context.Context) error { return nil })

	err = sq.Wait()
	if err != nil {
		t.Errorf("Wait() error = %v, want nil", err)
	}

	// After successful Wait(), Cause() should still be nil.
	if cause := sq.Cause(); cause != nil {
		t.Errorf("Cause() after successful Wait() = %v, want nil", cause)
	}
}

func TestCauseAfterTaskFailure(t *testing.T) {
	t.Parallel()

	errTask := errors.New("task error")

	sq, err := squad.New(
		squad.WithGracefulPeriod(100 * time.Millisecond),
	)
	if err != nil {
		t.Fatal(err)
	}

	sq.Run(func(ctx context.Context) error {
		return errTask
	})

	err = sq.Wait()
	if err == nil {
		t.Fatal("Wait() error = nil, want non-nil")
	}

	// Cause() should return the task error.
	cause := sq.Cause()
	if cause == nil {
		t.Fatal("Cause() after task failure = nil, want non-nil")
	}
	if !errors.Is(cause, errTask) {
		t.Errorf("Cause() = %v, want %v", cause, errTask)
	}
}

func TestCauseAfterCancel(t *testing.T) {
	sq, err := squad.New(
		squad.WithGracefulPeriod(100*time.Millisecond),
		squad.WithSignals(syscall.SIGUSR2),
	)
	if err != nil {
		t.Fatal(err)
	}

	sq.Run(func(ctx context.Context) error {
		// Block until context is cancelled by the signal.
		<-ctx.Done()
		return nil
	})

	// Send SIGUSR2 to self to trigger cancellation via the signal handler.
	if err := syscall.Kill(syscall.Getpid(), syscall.SIGUSR2); err != nil {
		t.Fatalf("Kill() error = %v", err)
	}

	err = sq.Wait()
	if err != nil {
		t.Logf("Wait() returned error: %v", err)
	}

	cause := sq.Cause()
	if cause == nil {
		t.Fatal("Cause() after signal = nil, want non-nil")
	}
	if !errors.Is(cause, context.Canceled) {
		t.Errorf("Cause() = %v, want context.Canceled", cause)
	}
}

func TestWithSignals(t *testing.T) {
	t.Parallel()

	// Override signals with custom set, verify no error on creation.
	sq, err := squad.New(
		squad.WithGracefulPeriod(100*time.Millisecond),
		squad.WithSignals(syscall.SIGUSR1, syscall.SIGUSR2),
	)
	if err != nil {
		t.Fatalf("New() with WithSignals error = %v, want nil", err)
	}

	sq.Run(func(ctx context.Context) error { return nil })

	err = sq.Wait()
	if err != nil {
		t.Errorf("Wait() error = %v, want nil", err)
	}
}

func TestWithSecondSignalHandler(t *testing.T) {
	t.Parallel()

	var secondSignalCalled bool
	sq, err := squad.New(
		squad.WithGracefulPeriod(100*time.Millisecond),
		squad.WithSecondSignalHandler(func() {
			secondSignalCalled = true
		}),
	)
	if err != nil {
		t.Fatalf("New() with WithSecondSignalHandler error = %v, want nil", err)
	}

	sq.Run(func(ctx context.Context) error { return nil })

	err = sq.Wait()
	if err != nil {
		t.Errorf("Wait() error = %v, want nil", err)
	}

	// Second signal handler should NOT have been called (no signals sent).
	if secondSignalCalled {
		t.Error("second signal handler was called unexpectedly")
	}
}
