package cmux

import (
	"errors"
	"net"
	"strings"
	"testing"
)

type prefixTestHandler string

func (prefixTestHandler) ServeConn(net.Conn) {}

func TestHandlePrefixRejectsEmptyBatch(t *testing.T) {
	for _, prefixes := range [][]string{{""}, {"", "GET ", "SSH-"}, {"GET ", "SSH-", ""}} {
		mux := NewCMux()
		if err := mux.HandlePrefix(prefixTestHandler("old"), "SSH-"); err != nil {
			t.Fatal(err)
		}
		if err := mux.HandlePrefix(prefixTestHandler("rejected"), prefixes...); err == nil {
			t.Fatalf("HandlePrefix(%q) accepted an empty prefix", prefixes)
		}
		if handler, _, err := mux.Handler(strings.NewReader("SSH-2.0-test\r\n")); err != nil || handler != prefixTestHandler("old") {
			t.Fatalf("rejected batch changed existing route: %v, %v", handler, err)
		}
		if handler, _, err := mux.Handler(strings.NewReader("GET / HTTP/1.1\r\n")); handler != nil || !errors.Is(err, ErrNotFound) {
			t.Fatalf("rejected batch registered GET: %v, %v", handler, err)
		}
		if err := mux.HandlePrefix(prefixTestHandler("valid"), "POST "); err != nil {
			t.Fatal(err)
		}
		if handler, _, err := mux.Handler(strings.NewReader("POST / HTTP/1.1\r\n")); err != nil || handler != prefixTestHandler("valid") {
			t.Fatalf("valid registration failed: %v, %v", handler, err)
		}
		if handler, _, err := mux.Handler(strings.NewReader("GET / HTTP/1.1\r\n")); handler != nil || !errors.Is(err, ErrNotFound) {
			t.Fatalf("rejected route reappeared: %v, %v", handler, err)
		}
		if err := mux.HandlePrefix(prefixTestHandler("unused")); err != nil {
			t.Fatalf("zero-prefix registration: %v", err)
		}
	}
}

func TestMuxListenerRejectsEmptyPrefix(t *testing.T) {
	root, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	mux := NewMuxListener(root)
	listener, err := mux.MatchPrefix("GET ", "")
	if listener != nil {
		listener.Close()
	}
	if err == nil || listener != nil {
		t.Fatalf("MatchPrefix() = %v, %v; want nil, error", listener, err)
	}
	if handler, _, err := mux.mux.Handler(strings.NewReader("GET / HTTP/1.1\r\n")); handler != nil || !errors.Is(err, ErrNotFound) {
		t.Fatalf("failed MatchPrefix registered a route: %v, %v", handler, err)
	}
}
