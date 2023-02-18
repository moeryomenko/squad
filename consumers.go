// helpers for run consumer workers.
package squad

import "context"

// ConsumerLoop is interface for run graceful consumer, which take context different
// context for consumer events/messages and handle them.
type ConsumerLoop func(consumeContext, handleContext context.Context) error
