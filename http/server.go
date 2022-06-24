package http

import (
	"context"
	"errors"
	"net"
	gohttp "net/http"
	"time"

	"github.com/moeryomenko/squad"
)

type Server struct {
	gohttp.Server
}

func (srv *Server) ListenAndServe(ctx context.Context) error {
	addr := srv.Addr
	if addr == "" {
		addr = ":http"
	}
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	return srv.Serve(ln)
}

func RunServer(srv *Server) (up, down func(context.Context) error) {
	return func(ctx context.Context) error {
			srv.ConnContext = func(ctx context.Context, _ net.Conn) context.Context {
				return squad.WithDelay(ctx, ctx.Value(squad.GracePeriod{}).(time.Duration))
			}
			err := srv.ListenAndServe(ctx)
			if errors.Is(err, gohttp.ErrServerClosed) {
				return nil
			}
			return err
		}, func(ctx context.Context) error {
			err := srv.Shutdown(ctx)
			if errors.Is(err, gohttp.ErrServerClosed) {
				return nil
			}
			return err
		}
}
