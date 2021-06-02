package main

import (
	"context"

	"github.com/moeryomenko/squad"
)

// Simple example empty service.
// Into two expressions you can create simple service with
// healthchecker and signal handler, which will provide
// graceful shutdown service.
func main() {
	s := squad.NewSquad(context.Background(),
		squad.WithHealthHandler(5000),
		squad.WithSignalHandler())

	s.Wait()
}
