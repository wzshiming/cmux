package cmux

import (
	"io"
	"net"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"testing"
	"time"
)

// workerRoot is a net.Listener fed by the test; Close wakes a blocked Accept.
type workerRoot struct {
	conns chan net.Conn
	done  chan struct{}
}

func (l *workerRoot) Accept() (net.Conn, error) {
	select {
	case conn := <-l.conns:
		return conn, nil
	case <-l.done:
		return nil, ErrListenerClosed
	}
}

func (l *workerRoot) Close() error {
	close(l.done)
	return nil
}

func (l *workerRoot) Addr() net.Addr { return &net.TCPAddr{} }

func TestMuxListenerWorkerGoroutinesExitAfterRun(t *testing.T) {
	const n = 8
	base := runtime.NumGoroutine()
	root := &workerRoot{conns: make(chan net.Conn, n), done: make(chan struct{})}
	m := NewMuxListener(root)
	served := make(chan string, n)
	m.mux.HandlePrefix(HandlerFunc(func(conn net.Conn) {
		defer conn.Close()
		b, _ := io.ReadAll(conn)
		served <- string(b)
	}), "W")

	runDone := make(chan struct{})
	go func() {
		defer close(runDone)
		m.run()
	}()

	want := make([]string, n)
	for i := 0; i < n; i++ {
		payload := "W" + strconv.Itoa(i)
		want[i] = payload
		client, server := net.Pipe()
		root.conns <- server
		go func() {
			defer client.Close()
			client.Write([]byte(payload))
		}()
	}
	got := make([]string, 0, n)
	timeout := time.After(5 * time.Second)
	for len(got) < n {
		select {
		case p := <-served:
			got = append(got, p)
		case <-timeout:
			t.Fatalf("served %d of %d conns: %q", len(got), n, got)
		}
	}
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("served %q, want %q", got, want)
	}

	root.Close()
	select {
	case <-runDone:
	case <-time.After(5 * time.Second):
		t.Fatal("run did not return after root Close")
	}
	deadline := time.Now().Add(2 * time.Second)
	for runtime.NumGoroutine() > base {
		if time.Now().After(deadline) {
			buf := make([]byte, 1<<16)
			t.Fatalf("goroutines = %d, want <= %d after run returned:\n%s", runtime.NumGoroutine(), base, buf[:runtime.Stack(buf, true)])
		}
		time.Sleep(10 * time.Millisecond)
	}
}
