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
	s, err := squad.New(squad.WithSignalHandler(2 * time.Second))
	if err != nil {
		log.Fatalf("service could not start, reason: %v", err)
	}

	s.RunGracefully(func(_ context.Context) error {
		return nil
	}, func(_ context.Context) error {
		fmt.Println("service shutdowning...")
		time.Sleep(3 * time.Second)

		// never printed, because default grace period 2 seconds.
		fmt.Println("service shutdowned")

		return nil
	})

	s.Wait()
}
