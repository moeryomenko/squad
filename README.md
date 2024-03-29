# Squad [![Go Reference](https://pkg.go.dev/badge/github.com/moeryomenko/squad.svg)](https://pkg.go.dev/github.com/moeryomenko/squad)

Squad is package contains a shared shutdown primitive.

## Usage

Simple service with signal handler and healthchecker.

```go
package main

import (
	"context"

	"github.com/moeryomenko/squad"
)

func main() {
	s := squad.NewSquad(squad.WithSignalHandler())

	// s.Run(...) // run your code.

	s.Wait()
}
```

## License

Squad is primarily distributed under the terms of both the MIT license and the Apache License (Version 2.0).

See [LICENSE-APACHE](LICENSE-APACHE) and/or [LICENSE-MIT](LICENSE-MIT) for details.
