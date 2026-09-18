package cmux

import (
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

type tagHandler string

func (tagHandler) ServeConn(net.Conn) {}

// await fails the test if ch does not deliver within the timeout.
func await(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(10 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
	}
}

func TestCMuxConcurrentRegisterAndHandler(t *testing.T) {
	const writers, rounds = 4, 100
	m := NewCMux()
	if err := m.HandlePrefix(tagHandler("ssh"), "SSH-"); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < writers; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < rounds; j++ {
				if err := m.HandlePrefix(tagHandler("http"), fmt.Sprintf("W%d-%d ", i, j)); err != nil {
					t.Error(err)
				}
				if err := m.NotFound(tagHandler("fallback")); err != nil {
					t.Error(err)
				}
			}
		}(i)
		go func() {
			defer wg.Done()
			for j := 0; j < rounds; j++ {
				if h, _, err := m.Handler(strings.NewReader("SSH-2.0-x\r\n")); err != nil || h != tagHandler("ssh") {
					t.Errorf("Handler(SSH) = %v, %v; want ssh", h, err)
				}
				h, _, err := m.Handler(strings.NewReader("zzz"))
				if !(err == nil && h == tagHandler("fallback")) && !(h == nil && errors.Is(err, ErrNotFound)) {
					t.Errorf("Handler(unmatched) = %v, %v; want fallback or ErrNotFound", h, err)
				}
			}
		}()
	}
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	await(t, done, "concurrent registrations and lookups")
	if h, _, err := m.Handler(strings.NewReader("zzz")); err != nil || h != tagHandler("fallback") {
		t.Fatalf("Handler(unmatched) = %v, %v; want fallback", h, err)
	}
	for i := 0; i < writers; i++ {
		for j := 0; j < rounds; j++ {
			prefix := fmt.Sprintf("W%d-%d ", i, j)
			if h, _, err := m.Handler(strings.NewReader(prefix + "x")); err != nil || h != tagHandler("http") {
				t.Fatalf("Handler(%q) = %v, %v; want http", prefix, h, err)
			}
		}
	}
}

// blockingReader parks Read until release is closed and reports the first Read on entered.
type blockingReader struct {
	io.Reader
	entered chan struct{}
	release chan struct{}
}

func (r *blockingReader) Read(p []byte) (int, error) {
	select {
	case r.entered <- struct{}{}:
	default:
	}
	<-r.release
	return r.Reader.Read(p)
}

func TestCMuxRegistrationDuringBlockedLookup(t *testing.T) {
	m := NewCMux()
	if err := m.HandlePrefix(tagHandler("ssh"), "SSH-"); err != nil {
		t.Fatal(err)
	}
	r := &blockingReader{
		Reader:  strings.NewReader("GET / HTTP/1.1\r\n"),
		entered: make(chan struct{}, 1),
		release: make(chan struct{}),
	}
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(r.release) }) }
	defer release()

	var got Handler
	var gotErr error
	lookup := make(chan struct{})
	go func() {
		defer close(lookup)
		got, _, gotErr = m.Handler(r)
	}()
	await(t, r.entered, "the lookup to enter Read")

	var regErr error
	registered := make(chan struct{})
	go func() {
		defer close(registered)
		if regErr = m.HandlePrefix(tagHandler("http"), "GET "); regErr == nil {
			regErr = m.NotFound(tagHandler("fallback"))
		}
	}()
	await(t, registered, "registration while a lookup is blocked in Read")
	if regErr != nil {
		t.Fatal(regErr)
	}

	release()
	await(t, lookup, "the blocked lookup")
	// The in-flight lookup must keep the trie and fallback it started with.
	if got != nil || !errors.Is(gotErr, ErrNotFound) {
		t.Fatalf("in-flight lookup = %v, %v; want nil, ErrNotFound", got, gotErr)
	}
	if h, _, err := m.Handler(strings.NewReader("GET / HTTP/1.1\r\n")); err != nil || h != tagHandler("http") {
		t.Fatalf("Handler(GET) after registration = %v, %v; want http", h, err)
	}
	if h, _, err := m.Handler(strings.NewReader("zzz")); err != nil || h != tagHandler("fallback") {
		t.Fatalf("Handler(unmatched) after NotFound = %v, %v; want fallback", h, err)
	}
}
