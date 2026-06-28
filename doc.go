// Package squad provides goroutine lifecycle management with graceful shutdown.
//
// Squad manages a collection of goroutines that start together and shut down
// gracefully when one fails or an OS signal is received.
//
// # Basic usage
//
// Create a Squad with New, register goroutines with Run or RunServer, then
// call Wait to block until all goroutines exit:
//
//	s, err := squad.New(squad.WithGracefulPeriod(5 * time.Second))
//	if err != nil {
//	    log.Fatal(err)
//	}
//	s.Run(func(ctx context.Context) error {
//	    <-ctx.Done()
//	    return nil
//	})
//	s.RunServer(&http.Server{Addr: ":8080"})
//	if err := s.Wait(); err != nil {
//	    log.Fatal(err)
//	}
//
// # Signal handling
//
// Squad installs a signal handler for SIGINT, SIGTERM, and SIGHUP by default.
// The first signal triggers graceful shutdown by cancelling the internal context.
// A second signal forces immediate exit via os.Exit(1). Both the signal set and
// the second-signal behaviour are configurable with WithSignals and
// WithSecondSignalHandler.
//
// # Bootstrap and cleanup
//
// Bootstrap functions (WithBootstrap) run before any goroutines start. If a
// bootstrap fails, New returns the error without starting any goroutines.
// Cleanup functions (WithCloses, or the onDown parameter of RunGracefully) run
// after all goroutines exit, bounded by the graceful timeout.
package squad
