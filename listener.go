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

// MatchPrefix returns the net.Listener that matches the prefix
func (m *MuxListener) MatchPrefix(prefixes ...string) (net.Listener, error) {
	ml := m.muxListener()
	err := m.mux.HandlePrefix(ml, prefixes...)
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
		ch:   make(chan net.Conn),
	}
}

type muxListener struct {
	addr    net.Addr
	ch      chan net.Conn
	isClose uint32
}

func (l *muxListener) ServeConn(conn net.Conn) {
	if atomic.LoadUint32(&l.isClose) == 1 {
		conn.Close()
		return
	}
	l.ch <- conn
}

func (l *muxListener) Accept() (net.Conn, error) {
	if atomic.LoadUint32(&l.isClose) == 1 {
		return nil, ErrListenerClosed
	}
	return <-l.ch, nil
}

func (l *muxListener) Close() error {
	atomic.StoreUint32(&l.isClose, 1)
	return nil
}

func (l *muxListener) Addr() net.Addr {
	return l.addr
}
