package cmux

import (
	"errors"
	"io"
	"net"

	"github.com/wzshiming/trie"
)

var (
	ErrNotFound = trie.ErrNotFound
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
func (m *CMux) Handler(r io.Reader) (handler Handler, prefix []byte, err error) {
	handler, prefix, err = m.trie.MatchWithReader(r)
	if err != nil {
		if m.notFound != nil && errors.Is(err, ErrNotFound) {
			return m.notFound, prefix, nil
		}
		return nil, prefix, err
	}
	return handler, prefix, nil
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
