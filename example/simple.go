package main

import (
	"context"
	"fmt"
	"time"

	"github.com/moeryomenko/squad"
)

// Simple example empty service.
// Into two expressions you can create simple service with
// healthchecker and signal handler, which will provide
// graceful shutdown service.
func main() {
	s := squad.NewSquad(context.Background(),
		squad.WithHealthHandler(5000),
		squad.WithSignalHandler(),
		squad.WithProfileHandler(6000))

	s.RunGracefully(func(_ context.Context) error {
		return nil
	}, func(_ context.Context) error {
		fmt.Println("service shutdowning...")
		time.Sleep(3 * time.Second)

		// never printed, because default cancellation timeout 2 seconds.
		fmt.Println("service shutdowned")

		return nil
	})

	s.Wait()
}
