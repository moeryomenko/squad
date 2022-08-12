package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/moeryomenko/squad"
)

// Simple example empty service.
// Into two expressions you can create simple service with
// healthchecker and signal handler, which will provide
// graceful shutdown service.
func main() {
	s, err := squad.New(squad.WithSignalHandler(
		squad.WithGracefulPeriod(10*time.Second),
		squad.WithShutdownTimeout(2*time.Second),
	))
	if err != nil {
		log.Fatalf("service could not start, reason: %v", err)
	}

	s.RunGracefully(func(ctx context.Context) error {
		<-ctx.Done()
		return nil
	}, func(_ context.Context) error {
		fmt.Println("service shutdowning...")
		time.Sleep(3 * time.Second)

		// never printed, because we set shutdown timeout 2 seconds.
		fmt.Println("service shutdowned")

		return nil
	})

	s.Wait()
}
