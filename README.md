# Squad — Goroutine Lifecycle Management

[![Go Version](https://img.shields.io/badge/go-1.25-blue)](https://go.dev/dl/)
[![License](https://img.shields.io/badge/license-Apache%202.0%20%7C%20MIT-blue)](LICENSE-APACHE)

Squad manages a group of goroutines that start together and stop together when one fails. Graceful shutdown with signal handling out of the box.

## Quick Start

```go
package main

import (
    "context"
    "log"
    "net/http"
    "time"

    "github.com/moeryomenko/squad"
)

func main() {
    s, err := squad.New(squad.WithGracefulPeriod(5 * time.Second))
    if err != nil {
        log.Fatalf("startup failed: %v", err)
    }

    // Register your background workers.
    s.Run(func(ctx context.Context) error {
        <-ctx.Done()
        return nil
    })

    // Run an HTTP server with graceful shutdown.
    s.RunServer(&http.Server{Addr: ":8080"})

    if err := s.Wait(); err != nil {
        log.Fatalf("shutdown error: %v", err)
    }
}
```

## API Reference

### Construction

```go
s, err := squad.New(opts ...Option)
```

`New` creates a Squad, runs bootstrap functions, and starts the OS signal handler. If any bootstrap function fails, the context is cancelled immediately and the error is returned.

### Options

| Option | Description |
|--------|-------------|
| `WithGracefulPeriod(d time.Duration)` | Sets the graceful shutdown timeout for shutdown functions. Default: 30s. |
| `WithBootstrap(fns ...func(ctx) error)` | Registers initialisation functions that run prior to any goroutines starting. |
| `WithCloses(fns ...func(ctx) error)` | Registers cleanup functions that run after all goroutines exit. |
| `WithSubsystem(init, close func(ctx) error)` | Convenience for pairing a bootstrap and cleanup function for a subsystem. |
| `WithSignals(sigs ...os.Signal)` | Overrides the default signal set (SIGINT, SIGTERM, SIGHUP). |
| `WithSecondSignalHandler(fn func())` | Overrides the default second-signal behaviour (`os.Exit(1)`). |

### Methods

| Method | Description |
|--------|-------------|
| `Run(fn func(ctx) error)` | Registers a tracked goroutine. When `fn` returns, all other goroutines receive the shutdown signal. |
| `RunGracefully(background, onDown func(ctx) error)` | Registers a goroutine with a shutdown hook. The `onDown` function runs during graceful shutdown. |
| `RunServer(srv *http.Server)` | Runs an `http.Server` with automatic graceful shutdown on stop. |
| `Go(fn func())` | Starts a fire-and-forget goroutine. Not tracked for errors; use `ctx.Done()` for shutdown. |
| `Done() <-chan struct{}` | Returns a channel that is closed when shutdown begins. |
| `Cause() error` | Returns the first error that caused shutdown, or `context.Canceled` if triggered by a signal. Safe to call after `Wait()`. |
| `Wait() error` | Blocks until all goroutines exit, runs shutdown functions, and returns any errors. |

## Signal Handling

Squad installs a signal handler for `SIGINT`, `SIGTERM`, and `SIGHUP` by default (configurable via `WithSignals`).

1. **First signal**: cancels the squad context, which propagates `ctx.Done()` to all running goroutines. The graceful shutdown sequence begins.
2. **Second signal**: by default calls `os.Exit(1)`. Override with `WithSecondSignalHandler`.

## Graceful Shutdown Lifecycle

```
Signal (SIGINT/SIGTERM/SIGHUP)
    |
    v
context cancelled -> goroutines receive ctx.Done()
    |
    v
shutdown functions (onDown, WithCloses) run with timeout
    |
    v
Wait() returns error (if any)
```

During shutdown, all registered `onDown` functions and `WithCloses` functions run concurrently, bounded by the graceful timeout (default 30s). If a shutdown function exceeds the timeout, `Wait()` returns `context.DeadlineExceeded`.

## Comparison

| Library | Signal handling | Bootstrap lifecycle | Shutdown hooks |
|---------|----------------|---------------------|----------------|
| **Squad** | Built-in (first signal = graceful, second = force) | Built-in (`WithBootstrap`) | Built-in (`WithCloses`, `RunGracefully`) |
| `errgroup` | Manual | Manual | Manual |
| `run.Group` | Per-actor signal handling | Manual | Per-actor interrupt |
| Manual `context.WithCancel` | Manual | Manual | Manual |

Squad is designed for services that need coordinated startup, error propagation, and graceful shutdown without boilerplate.

## Contributing

### Prerequisites

- Go 1.25+

### Targets

- `make fmt` — format code with gofmt, golines, and goimports.
- `make lint` — run golangci-lint.
- `make test` — run tests with race detector and coverage.

## License

Dual-licensed under [Apache 2.0](LICENSE-APACHE) and [MIT](LICENSE-MIT).
