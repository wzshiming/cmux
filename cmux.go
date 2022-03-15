package cmux

import (
	"fmt"
	"io"
	"net"

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
	trie     *trie.Trie[Handler]
	notFound Handler
}

// NewCMux create a new CMux.
func NewCMux() *CMux {
	p := &CMux{
		trie: trie.NewTrie[Handler](),
	}
	return p
}

// NotFound handle the handler that unmatched
func (m *CMux) NotFound(handler Handler) error {
	m.notFound = handler
	return nil
}

// HandlePrefix handle the handler that matches the prefix
func (m *CMux) HandlePrefix(handler Handler, prefixes ...string) error {
	for _, prefix := range prefixes {
		m.trie.Put([]byte(prefix), handler)
	}
	return nil
}

// Handler returns most matching handler and prefix bytes data to use for the given reader.
func (m *CMux) Handler(r io.Reader) (Handler, []byte, error) {
	if m.trie.Size() == 0 {
		if m.notFound == nil {
			return nil, nil, ErrNotFound
		}
		return m.notFound, nil, nil
	}
	prefix := make([]byte, m.trie.Depth())
	n, err := io.ReadFull(r, prefix)
	if err != nil {
		return nil, nil, err
	}
	prefix = prefix[:n]
	handler, _, _ := m.trie.Get(prefix)
	if handler != nil {
		return handler, prefix, nil
	}
	if m.notFound == nil {
		return nil, prefix, ErrNotFound
	}
	return m.notFound, prefix, nil
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
