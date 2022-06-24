package squad

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_DelayedContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	delayedCtx := WithDelay(ctx, 100*time.Millisecond)
	cancel()

	counter := 0
	func() {
		for {
			select {
			case <-delayedCtx.Done():
				return
			default:
				counter++
			}
		}
	}()
	assert.NotEqual(t, 0, counter)
}
