package cmux

import (
	"errors"
	"io"
	"net"
	"sync"

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
	mu       sync.Mutex
	prefixes []prefixHandler
	// trie is rebuilt on every registration and never mutated after publish.
	trie     *trie.Trie[Handler]
	notFound Handler
}

type prefixHandler struct {
	prefix  string
	handler Handler
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
	m.mu.Lock()
	m.notFound = handler
	m.mu.Unlock()
	return nil
}

// HandlePrefix handle the handler that matches the prefix
func (m *CMux) HandlePrefix(handler Handler, prefixes ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, prefix := range prefixes {
		// trie.Put ignores empty keys.
		if prefix == "" {
			continue
		}
		m.prefixes = append(m.prefixes, prefixHandler{prefix: prefix, handler: handler})
	}
	t := trie.NewTrie[Handler]()
	for _, p := range m.prefixes {
		t.Put([]byte(p.prefix), p.handler)
	}
	m.trie = t
	return nil
}

// Handler returns most matching handler and prefix bytes data to use for the given reader.
func (m *CMux) Handler(r io.Reader) (handler Handler, prefix []byte, err error) {
	m.mu.Lock()
	t, notFound := m.trie, m.notFound
	m.mu.Unlock()
	handler, prefix, err = t.MatchWithReader(r)
	if err != nil {
		if notFound != nil && errors.Is(err, ErrNotFound) {
			return notFound, prefix, nil
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
