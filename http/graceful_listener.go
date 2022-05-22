package http

import (
	"context"
	"errors"
	"net"
)

type tcpGracefulListener struct {
	net.Listener
	ctx context.Context
}

// Accept waits for and returns the next connections to the listener.
func (l *tcpGracefulListener) Accept() (net.Conn, error) {
	if l.ctx.Err() != nil {
		return nil, ErrListenerStoppedAccept
	}
	return l.Listener.Accept()
}

// Close closes the listener.
// Any blocked Accept operations will be unblocked and return errors.
func (l *tcpGracefulListener) Close() error {
	return l.Listener.Close()
}

// Addr returns the listener's network address.
func (l *tcpGracefulListener) Addr() net.Addr {
	return l.Listener.Addr()
}

// ErrListenerStoppedAccept
var ErrListenerStoppedAccept = errors.New("listener stopped accepting connections")
