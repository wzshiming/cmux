package cmux

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

var ErrListenerClosed = fmt.Errorf("listener closed: %w", net.ErrClosed)

// MuxListener is a multiplexer for network connections
type MuxListener struct {
	listener   net.Listener
	notFound   *muxListener
	mux        *CMux
	isStart    uint32
	ErrHandler func(err error) bool
	ch         chan net.Conn

	mu       sync.Mutex
	children []*muxListener
	exited   bool
}

// NewMuxListener create a new MuxListener.
func NewMuxListener(listener net.Listener) *MuxListener {
	return &MuxListener{
		listener: listener,
		mux:      NewCMux(),
		ch:       make(chan net.Conn),
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
	defer m.closeChildren()
	for {
		conn, err := m.listener.Accept()
		if err != nil {
			if m.ErrHandler != nil && m.ErrHandler(err) {
				continue
			}
			return
		}
		select {
		case m.ch <- conn:
		default:
			go m.handleConn()
			m.ch <- conn
		}
	}
}

func (m *MuxListener) handleConn() {
	for {
		select {
		case conn, ok := <-m.ch:
			if !ok {
				return
			}
			m.mux.ServeConn(conn)
		case <-time.After(time.Minute):
		}
	}
}

func (m *MuxListener) closeChildren() {
	m.mu.Lock()
	m.exited = true
	children := m.children
	m.children = nil
	m.mu.Unlock()
	for _, ml := range children {
		ml.Close()
	}
}

func (m *MuxListener) muxListener() *muxListener {
	ml := &muxListener{
		addr: m.listener.Addr(),
		ch:   make(chan net.Conn),
		done: make(chan struct{}),
	}
	m.mu.Lock()
	if m.exited {
		ml.Close()
	} else {
		m.children = append(m.children, ml)
	}
	m.mu.Unlock()
	if atomic.CompareAndSwapUint32(&m.isStart, 0, 1) {
		go m.run()
	}
	return ml
}

type muxListener struct {
	addr net.Addr
	ch   chan net.Conn
	done chan struct{}
	once sync.Once
}

func (l *muxListener) ServeConn(conn net.Conn) {
	select {
	case <-l.done:
		conn.Close()
		return
	default:
	}
	select {
	case l.ch <- conn:
	case <-l.done:
		conn.Close()
	}
}

func (l *muxListener) Accept() (net.Conn, error) {
	select {
	case <-l.done:
		return nil, ErrListenerClosed
	default:
	}
	select {
	case conn := <-l.ch:
		return conn, nil
	case <-l.done:
		return nil, ErrListenerClosed
	}
}

func (l *muxListener) Close() error {
	l.once.Do(func() { close(l.done) })
	return nil
}

func (l *muxListener) Addr() net.Addr {
	return l.addr
}
