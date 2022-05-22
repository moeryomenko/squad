package http

import (
	"context"
	"net"
	"net/http"
)

// ListenAndServe listens on the TCP network address addr and then calls
// Serve with handler to handle requests on incoming connections.
// Accepted connections are configured to enable TCP keep-alives.
//
// The handler is typically nil, in which case DefaultServeMux is used.
//
// ListenAndServe always returns a non-nil error.
func ListenAndServe(ctx context.Context, addr string, handler http.Handler) error {
	server := &http.Server{Addr: addr, Handler: handler}
	if addr == "" {
		addr = ":http"
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer ln.Close()

	return server.Serve(&tcpGracefulListener{ln, ctx})
}

// ListenAndServeTLS acts identically to ListenAndServe, except that it
// expects HTTPS connections. Additionally, files containing a certificate and
// matching private key for the server must be provided. If the certificate
// is signed by a certificate authority, the certFile should be the concatenation
// of the server's certificate, any intermediates, and the CA's certificate.
func ListenAndServeTLS(ctx context.Context, addr, certFile, keyFile string, handler http.Handler) error {
	server := &http.Server{Addr: addr, Handler: handler}
	if addr == "" {
		addr = ":http"
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer ln.Close()

	return server.ServeTLS(&tcpGracefulListener{ln, ctx}, certFile, keyFile)
}
