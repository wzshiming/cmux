package cmux

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
	"sync/atomic"

	"github.com/wzshiming/crun"
	"github.com/wzshiming/trie"
)

var (
	ErrNotFound = fmt.Errorf("error not found")
)

type Handler interface {
	ServeConn(conn net.Conn)
}

type HandlerFunc func(conn net.Conn)

func (h HandlerFunc) ServeConn(conn net.Conn) {
	h(conn)
}

// CMux is an Applicative protocol multiplexer
// It matches the prefix of each incoming reader against a list of registered patterns
// and calls the handler for the pattern that most closely matches the Handler.
type CMux struct {
	trie         *trie.Trie
	prefixLength int
	size         uint32
	handlers     map[uint32]Handler
	notFound     Handler
}

// NewCMux create a new CMux.
func NewCMux() *CMux {
	p := &CMux{
		trie:     trie.NewTrie(),
		handlers: map[uint32]Handler{},
	}

	return p
}

// NotFound handle the handler that unmatched
func (m *CMux) NotFound(handler Handler) error {
	m.notFound = handler
	return nil
}

// HandleRegexp handle the handler that matches the regular
func (m *CMux) HandleRegexp(pattern string, handler Handler) error {
	if !strings.HasPrefix(pattern, "^") {
		return fmt.Errorf("only prefix matching is supported, change to %q", "^"+pattern)
	}
	r, err := crun.Compile(pattern)
	if err != nil {
		return err
	}

	if size := r.Size(); size > 1000 {
		return fmt.Errorf("regular is too large: %d", size)
	}

	buf := m.setHandler(handler)
	r.Range(func(prefix string) bool {
		m.handle(prefix, buf)
		return true
	})
	return nil
}

// HandlePrefix handle the handler that matches the prefix
func (m *CMux) HandlePrefix(prefix string, handler Handler) error {
	buf := m.setHandler(handler)
	m.handle(prefix, buf)
	return nil
}

// Handler returns most matching handler and prefix bytes data to use for the given reader.
func (m *CMux) Handler(r io.Reader) (handler Handler, prefix []byte, err error) {
	if m.prefixLength == 0 {
		return nil, nil, ErrNotFound
	}
	parent := m.trie.Mapping()
	off := 0
	prefix = make([]byte, m.prefixLength)
	for {
		i, err := r.Read(prefix[off:])
		if err != nil {
			return nil, nil, err
		}
		if i == 0 {
			break
		}

		data, next, _ := parent.Get(prefix[off : off+i])
		if len(data) != 0 {
			conn, ok := m.getHandler(data)
			if ok {
				handler = conn
			}
		}

		off += i
		if next == nil {
			break
		}
		parent = next
	}

	if handler == nil {
		if m.notFound == nil {
			return nil, prefix[:off], ErrNotFound
		}
		handler = m.notFound
	}
	return handler, prefix[:off], nil
}

func (m *CMux) handle(prefix string, buf []byte) {
	m.trie.Put([]byte(prefix), buf)
	if m.prefixLength < len(prefix) {
		m.prefixLength = len(prefix)
	}
}

func (m *CMux) setHandler(hand Handler) []byte {
	k := atomic.AddUint32(&m.size, 1)
	m.handlers[k] = hand
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, k)
	return buf
}

func (m *CMux) getHandler(index []byte) (Handler, bool) {
	c, ok := m.handlers[binary.BigEndian.Uint32(index)]
	return c, ok
}

// ServeConn dispatches the reader to the handler whose pattern most closely matches the reader.
func (m *CMux) ServeConn(conn net.Conn) {
	connector, buf, err := m.Handler(conn)
	if err != nil {
		conn.Close()
		return
	}
	conn = UnreadConn(conn, buf)
	connector.ServeConn(conn)
}
