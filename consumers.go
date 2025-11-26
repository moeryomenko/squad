// helpers for run consumer workers.
package squad

import "context"

// ConsumerLoop is interface for run graceful consumer, which take context different
// context for consumer events/messages and handle them.
type ConsumerLoop func(consumeContext, handleContext context.Context) error

// RunConsumer is wrapper function for run consumer worker
// after receiving shutdowning signal stop context for consumer events/messages
// without interrupting any active handler.
func (s *Squad) RunConsumer(consumer ConsumerLoop) {
	s.wg.Go(func(ctx context.Context) error {
		return consumer(ctx, context.WithoutCancel(ctx))
	})
}
