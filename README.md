<div align="center">

# 🚀 Squad

[![Go Reference](https://pkg.go.dev/badge/github.com/moeryomenko/squad.svg)](https://pkg.go.dev/github.com/moeryomenko/squad)
[![Go Report Card](https://goreportcard.com/badge/github.com/moeryomenko/squad)](https://goreportcard.com/report/github.com/moeryomenko/squad)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](./LICENSE-MIT)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](./LICENSE-APACHE)

**A shared shutdown primitive for graceful application lifecycle management in Go**

Squad helps you orchestrate goroutines, manage graceful shutdowns, and handle application lifecycle events with minimal boilerplate.

[Features](#-features) • [Installation](#-installation) • [Quick Start](#-quick-start) • [Documentation](#-documentation) • [Examples](#-examples)

</div>

---

## 📋 Table of Contents

- [About](#-about)
- [Features](#-features)
- [Installation](#-installation)
- [Quick Start](#-quick-start)
- [Usage](#-usage)
  - [Basic Service](#basic-service)
  - [HTTP Server](#http-server)
  - [Consumer Workers](#consumer-workers)
  - [Graceful Shutdown](#graceful-shutdown)
  - [Bootstrap & Cleanup](#bootstrap--cleanup)
- [API Reference](#-api-reference)
- [Examples](#-examples)
- [Project Structure](#-project-structure)
- [Development](#-development)
- [Testing](#-testing)
- [FAQ](#-faq)
- [Contributing](#-contributing)
- [License](#-license)
- [Author](#-author)

---

## 🎯 About

**Squad** is a lightweight Go package that provides a shared shutdown primitive for managing application lifecycle. It allows you to:

- **Coordinate multiple goroutines** that must start and stop together
- **Handle graceful shutdowns** with configurable timeouts and grace periods
- **Manage HTTP servers** with proper shutdown sequences
- **Run consumer workers** (e.g., message queues, event streams) with graceful stop
- **Execute bootstrap and cleanup functions** for subsystems
- **Automatically handle OS signals** (SIGINT, SIGTERM, SIGHUP, SIGQUIT)

Squad is particularly useful for:
- Microservices and API servers
- Background workers and job processors
- Event-driven applications with multiple consumers
- Any Go application requiring coordinated startup/shutdown

The package is designed to be Kubernetes-aware, with default grace periods aligned with Pod termination lifecycle.

---

## ✨ Features

- ⚡ **Zero dependencies** - Uses only standard library and `github.com/moeryomenko/synx`
- 🔄 **Coordinated lifecycle** - If one goroutine exits, others are signaled to stop
- 🛡️ **Graceful shutdowns** - Configurable grace periods and shutdown timeouts
- 🌐 **HTTP server support** - Built-in wrapper for `http.Server` with proper shutdown
- 📨 **Consumer pattern** - Graceful stop for message/event consumers without interrupting active handlers
- 🎯 **Signal handling** - Automatic SIGINT/SIGTERM/SIGHUP/SIGQUIT handling
- 🔌 **Bootstrap/cleanup hooks** - Run initialization and cleanup functions
- ⏱️ **Kubernetes-friendly** - Default 30s grace period matches k8s pod termination
- 🧪 **Well tested** - Comprehensive test coverage
- 📦 **Simple API** - Minimal boilerplate, intuitive usage

---

## 📦 Installation

```bash
go get github.com/moeryomenko/squad
```

**Requirements:**
- Go 1.24 or higher

---

## 🚀 Quick Start

```go
package main

import (
    "context"
    "log"

    "github.com/moeryomenko/squad"
)

func main() {
    // Create a new squad with signal handler
    s, err := squad.New(squad.WithSignalHandler())
    if err != nil {
        log.Fatal(err)
    }

    // Run your application logic
    s.Run(func(ctx context.Context) error {
        // Your code here
        <-ctx.Done()
        return nil
    })

    // Wait for shutdown
    if err := s.Wait(); err != nil {
        log.Printf("shutdown error: %v", err)
    }
}
```

---

## 📖 Usage

### Basic Service

Create a simple service with signal handling:

```go
s, err := squad.New(squad.WithSignalHandler())
if err != nil {
    log.Fatal(err)
}

s.Run(func(ctx context.Context) error {
    // Your background work
    ticker := time.NewTicker(time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return nil
        case <-ticker.C:
            log.Println("tick")
        }
    }
})

s.Wait()
```

### HTTP Server

Launch an HTTP server with graceful shutdown:

```go
s, err := squad.New(squad.WithSignalHandler())
if err != nil {
    log.Fatal(err)
}

http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
})

s.RunServer(&http.Server{Addr: ":8080"})
s.Wait()
```

### Consumer Workers

Run consumer workers that stop gracefully without interrupting active handlers:

```go
s, err := squad.New(squad.WithSignalHandler())
if err != nil {
    log.Fatal(err)
}

s.RunConsumer(func(consumeCtx, handleCtx context.Context) error {
    // consumeCtx: cancelled when shutdown starts
    // handleCtx: never cancelled, allows active handlers to complete

    for {
        select {
        case <-consumeCtx.Done():
            return nil
        default:
            msg := receiveMessage() // Your message source
            go processMessage(handleCtx, msg)
        }
    }
})

s.Wait()
```

### Graceful Shutdown

Run tasks with cleanup functions:

```go
s, err := squad.New(squad.WithSignalHandler(
    squad.WithGracefulPeriod(30 * time.Second),
    squad.WithShutdownTimeout(5 * time.Second),
))
if err != nil {
    log.Fatal(err)
}

s.RunGracefully(
    // Background function
    func(ctx context.Context) error {
        // Your work
        <-ctx.Done()
        return nil
    },
    // Cleanup function (runs on shutdown)
    func(ctx context.Context) error {
        log.Println("cleaning up...")
        return closeResources(ctx)
    },
)

s.Wait()
```

### Bootstrap & Cleanup

Initialize and cleanup subsystems:

```go
s, err := squad.New(
    squad.WithSignalHandler(),
    squad.WithBootstrap(
        func(ctx context.Context) error {
            return initDatabase(ctx)
        },
        func(ctx context.Context) error {
            return connectToCache(ctx)
        },
    ),
    squad.WithCloses(
        func(ctx context.Context) error {
            return closeDatabase(ctx)
        },
    ),
    squad.WithSubsystem(
        func(ctx context.Context) error { return openMessageQueue(ctx) },
        func(ctx context.Context) error { return closeMessageQueue(ctx) },
    ),
)
if err != nil {
    log.Fatal("initialization failed:", err)
}

// Your application code...

s.Wait()
```

---

## 🔍 API Reference

### Core Types

- **`Squad`** - Main struct for coordinating goroutines
- **`ConsumerLoop`** - Function signature for consumer workers

### Constructor

- **`New(opts ...Option) (*Squad, error)`** - Create a new Squad instance

### Options

- **`WithSignalHandler(...ShutdownOpt)`** - Add OS signal handling
- **`WithGracefulPeriod(duration)`** - Set graceful shutdown period (default: 30s)
- **`WithShutdownTimeout(duration)`** - Set shutdown timeout (default: 2s)
- **`WithShutdownInGracePeriod(duration)`** - Set both graceful and shutdown timeout
- **`WithBootstrap(...func)`** - Add initialization functions
- **`WithCloses(...func)`** - Add cleanup functions
- **`WithSubsystem(init, close)`** - Add init+cleanup pair for a subsystem

### Methods

- **`Run(fn)`** - Run a function in the squad
- **`RunGracefully(backgroundFn, onDown)`** - Run with cleanup function
- **`RunServer(*http.Server)`** - Run HTTP server with graceful shutdown
- **`RunConsumer(ConsumerLoop)`** - Run consumer worker
- **`Wait() error`** - Block until all squad members exit

---

## 💡 Examples

See the [`example/`](./example) directory for complete examples:

- **[simple.go](./example/simple.go)** - HTTP server with signal handling and graceful shutdown

Run the example:

```bash
go run example/simple.go
```

---

## 📁 Project Structure

```
squad/
├── squad.go           # Core Squad implementation
├── consumers.go       # Consumer worker helpers
├── options.go         # Configuration options
├── squad_test.go      # Unit tests
├── example/           # Example applications
│   └── simple.go
├── tools/             # Development tools
├── go.mod             # Go module definition
├── Makefile           # Build automation
├── LICENSE-APACHE     # Apache 2.0 license
├── LICENSE-MIT        # MIT license
└── README.md          # This file
```

---

## 🛠 Development

### Prerequisites

- Go 1.24+
- golangci-lint (for linting)
- go-mod-upgrade (for dependency management)

### Commands

```bash
# Run tests
make test

# Run tests with race detector
RACE_DETECTOR=1 make test

# View coverage report
make cover

# Run linter
make lint

# Update dependencies
make mod

# View all commands
make help
```

---

## 🧪 Testing

Squad includes comprehensive unit tests covering:

- Basic lifecycle management
- Bootstrap initialization
- Background task failures
- Shutdown failures and timeouts
- Error propagation
- Concurrent operations

Run tests:

```bash
go test ./...

# With race detector
go test -race ./...

# With coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

---

## ❓ FAQ

### Q: What happens if a goroutine panics?

Squad uses `synx.CtxGroup` which recovers from panics and converts them to errors. The error will be returned from `Wait()`.

### Q: What's the difference between graceful period and shutdown timeout?

- **Graceful period**: Total time allowed for the entire shutdown process
- **Shutdown timeout**: Time reserved for executing cleanup functions

The signal handler waits `gracefulPeriod - shutdownTimeout` before canceling the squad context.

### Q: Can I use Squad without signal handling?

Yes! Simply don't use `WithSignalHandler()`. You'll need to manage context cancellation yourself.

### Q: How does RunConsumer differ from Run?

`RunConsumer` provides two contexts:
- `consumeContext`: Cancelled on shutdown (stop accepting new work)
- `handleContext`: Never cancelled (allow in-flight work to complete)

This enables graceful consumer shutdown without interrupting active message handlers.

### Q: Is Squad safe for concurrent use?

Yes, Squad is designed to be used concurrently. Internal state is protected by mutexes.

### Q: What's the default grace period?

30 seconds, matching Kubernetes pod termination grace period. This ensures compatibility with k8s deployments.

---

## 🤝 Contributing

Contributions are welcome! Please feel free to submit issues, fork the repository, and send pull requests.

### Guidelines

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes
4. Add tests for new functionality
5. Run tests and linter (`make test lint`)
6. Commit your changes
7. Push to the branch (`git push origin feature/amazing-feature`)
8. Open a Pull Request

---

## 📄 License

This project is dual-licensed under your choice of:

- **MIT License** - See [LICENSE-MIT](./LICENSE-MIT)
- **Apache License 2.0** - See [LICENSE-APACHE](./LICENSE-APACHE)

You may use this project under the terms of either license.

---

## 👤 Author

**Maxim Eryomenko**

- GitHub: [@moeryomenko](https://github.com/moeryomenko)
- Email: moeryomenko@gmail.com

---

<div align="center">

**If you find Squad useful, please consider giving it a ⭐️!**

Made with ❤️ by [Maxim Eryomenko](https://github.com/moeryomenko)

</div>
