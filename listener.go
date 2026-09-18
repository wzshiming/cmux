package cmux

import (
	"errors"
	"fmt"
	"net"
	"sync/atomic"
	"time"
)

var ErrListenerClosed = fmt.Errorf("listener closed")

// MuxListener is a multiplexer for network connections
type MuxListener struct {
	listener   net.Listener
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
	var delay time.Duration
	for {
		conn, err := m.listener.Accept()
		if err != nil {
			if m.ErrHandler != nil {
				if m.ErrHandler(err) {
					continue
				}
				return
			}
			var ne net.Error
			if !errors.As(err, &ne) || !ne.Temporary() {
				return
			}
			delay = nextDelay(delay)
			time.Sleep(delay)
			continue
		}
		delay = 0
		go m.mux.ServeConn(conn)
	}
}

// nextDelay doubles the accept retry delay from 5ms up to 1s.
func nextDelay(delay time.Duration) time.Duration {
	if delay == 0 {
		return 5 * time.Millisecond
	}
	if delay *= 2; delay > time.Second {
		return time.Second
	}
	return delay
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
