package cmux

import (
	"fmt"
	"net"
	"sync/atomic"
)

var ErrListenerClosed = fmt.Errorf("listener closed")

// MuxListener is a multiplexer for network connections
type MuxListener struct {
	listener   net.Listener
	notFound   *muxListener
	mux        *CMux
	isStart    uint32
	ErrHandler func(err error) bool
}

// NewMuxListener create a new MuxListener.
func NewMuxListener(listener net.Listener) *MuxListener {
	return &MuxListener{
		listener: listener,
		mux:      NewCMux(),
	}
}

// Unmatched returns the net.Listener that unmatched
func (m *MuxListener) Unmatched() (net.Listener, error) {
	ml := m.muxListener()
	err := m.mux.NotFound(ml)
	if err != nil {
		return nil, err
	}
	return ml, nil
}

// MatchRegexp returns the net.Listener that matches the regular
func (m *MuxListener) MatchRegexp(pattern string) (net.Listener, error) {
	ml := m.muxListener()
	err := m.mux.HandleRegexp(pattern, ml)
	if err != nil {
		return nil, err
	}
	return ml, nil
}

// MatchPrefix returns the net.Listener that matches the prefix
func (m *MuxListener) MatchPrefix(prefix string) (net.Listener, error) {
	ml := m.muxListener()
	err := m.mux.HandlePrefix(prefix, ml)
	if err != nil {
		return nil, err
	}
	return ml, nil
}

func (m *MuxListener) run() {
	for {
		conn, err := m.listener.Accept()
		if err != nil {
			if m.ErrHandler != nil && m.ErrHandler(err) {
				continue
			}
			return
		}
		m.mux.ServeConn(conn)
	}
}

func (m *MuxListener) muxListener() *muxListener {
	if atomic.CompareAndSwapUint32(&m.isStart, 0, 1) {
		go m.run()
	}
	return &muxListener{
		addr: m.listener.Addr(),
		mux:  m,
		ch:   make(chan net.Conn),
	}
}

type muxListener struct {
	addr    net.Addr
	mux     *MuxListener
	ch      chan net.Conn
	isClose uint32
}

func (l *muxListener) ServeConn(conn net.Conn) {
	select {
	case l.ch <- conn:
	default:
		conn.Close()
	}
}

func (l *muxListener) Accept() (net.Conn, error) {
	c, ok := <-l.ch
	if !ok {
		return nil, ErrListenerClosed
	}
	return c, nil
}

func (l *muxListener) Close() error {
	close(l.ch)
	return nil
}

func (l *muxListener) Addr() net.Addr {
	return l.addr
}
