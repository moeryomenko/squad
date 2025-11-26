package squad

import (
	"cmp"
	"context"
	"errors"
	"net/http"
)

// RunServer is wrapper function for launch http server.
func (s *Squad) RunServer(srv *http.Server) {
	// Track the server in the context group
	s.wg.Go(func(ctx context.Context) error {
		startErr := make(chan error, 1)

		go func() {
			err := srv.ListenAndServe()
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				startErr <- err
			}
			close(startErr)
		}()

		select {
		case err := <-startErr:
			return err
		case <-cmp.Or(s.serverContext, ctx).Done():
			// Initiate graceful shutdown
			shutdownCtx, cancel := context.WithTimeout(context.Background(), s.cancellationDelay)
			defer cancel()

			return srv.Shutdown(shutdownCtx)
		}
	})
}
