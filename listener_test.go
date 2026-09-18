package cmux

import (
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/wzshiming/cmux/pattern"
)

// closeRoot serves pushed conns and blocks Accept until Close.
type closeRoot struct {
	conns  chan net.Conn
	closed chan struct{}
	once   sync.Once
}

func newCloseRoot() *closeRoot {
	return &closeRoot{conns: make(chan net.Conn, 4), closed: make(chan struct{})}
}

func (l *closeRoot) Accept() (net.Conn, error) {
	select {
	case conn := <-l.conns:
		return conn, nil
	case <-l.closed:
		return nil, net.ErrClosed
	}
}

func (l *closeRoot) Close() error {
	l.once.Do(func() { close(l.closed) })
	return nil
}

func (l *closeRoot) Addr() net.Addr { return &net.TCPAddr{} }

func closeMux(t *testing.T) (*MuxListener, *closeRoot) {
	root := newCloseRoot()
	t.Cleanup(func() { root.Close() })
	return NewMuxListener(root), root
}

func closeMatch(t *testing.T, m *MuxListener, prefixes ...string) net.Listener {
	t.Helper()
	l, err := m.MatchPrefix(prefixes...)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

type closeResult struct {
	conn net.Conn
	err  error
}

func closeAccept(l net.Listener) <-chan closeResult {
	ch := make(chan closeResult, 1)
	go func() {
		conn, err := l.Accept()
		ch <- closeResult{conn, err}
	}()
	return ch
}

func closeWait(t *testing.T, pending <-chan closeResult) (net.Conn, error) {
	t.Helper()
	select {
	case r := <-pending:
		return r.conn, r.err
	case <-time.After(2 * time.Second):
		t.Fatal("Accept did not return within 2s")
		return nil, nil
	}
}

func closeWantErr(t *testing.T, pending <-chan closeResult) {
	t.Helper()
	conn, err := closeWait(t, pending)
	if conn != nil || !errors.Is(err, ErrListenerClosed) || !errors.Is(err, net.ErrClosed) {
		t.Fatalf("Accept() = %v, %v; want ErrListenerClosed wrapping net.ErrClosed", conn, err)
	}
}

func TestMuxSubListenerCloseWakesBlockedAccept(t *testing.T) {
	m, _ := closeMux(t)
	child := closeMatch(t, m, "GET ")
	pending := closeAccept(child)
	time.Sleep(20 * time.Millisecond) // let Accept block before closing
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := child.Close(); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	closeWantErr(t, pending)
	closeWantErr(t, closeAccept(child))
}

func closeServe(child net.Listener, conn net.Conn) <-chan struct{} {
	served := make(chan struct{})
	go func() {
		defer close(served)
		child.(Handler).ServeConn(conn)
	}()
	return served
}

func closeWantEOF(t *testing.T, conn net.Conn) {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if n, err := conn.Read(make([]byte, 1)); err != io.EOF {
		t.Fatalf("Read() = %d, %v; want EOF from the closed peer", n, err)
	}
}

func TestMuxSubListenerCloseReleasesUndeliveredConn(t *testing.T) {
	m, _ := closeMux(t)
	child := closeMatch(t, m, "GET ")
	client, server := net.Pipe()
	defer client.Close()
	served := closeServe(child, server)
	time.Sleep(20 * time.Millisecond) // let ServeConn block with no Accept
	child.Close()
	select {
	case <-served:
	case <-time.After(2 * time.Second):
		t.Fatal("ServeConn still blocked after Close")
	}
	closeWantEOF(t, client)
}

func TestMuxSubListenerClosedRejectsAcceptAndServeConn(t *testing.T) {
	m, _ := closeMux(t)
	child := closeMatch(t, m, "GET ")
	child.Close()
	for i := 0; i < 50; i++ {
		client, server := net.Pipe()
		served := closeServe(child, server)
		if conn, err := child.Accept(); conn != nil || !errors.Is(err, ErrListenerClosed) {
			t.Fatalf("Accept() after Close = %v, %v", conn, err)
		}
		<-served
		closeWantEOF(t, client)
		client.Close()
	}
	if err := child.Close(); err != nil {
		t.Fatalf("second Close() = %v", err)
	}
}

func TestMuxListenerRootExitClosesChildren(t *testing.T) {
	m, root := closeMux(t)
	child := closeMatch(t, m, "GET ")
	other, err := m.Unmatched()
	if err != nil {
		t.Fatal(err)
	}
	pending, pendingOther := closeAccept(child), closeAccept(other)
	root.Close()
	closeWantErr(t, pending)
	closeWantErr(t, pendingOther)
	closeWantErr(t, closeAccept(closeMatch(t, m, "SSH-"))) // born closed after exit
}

func TestMuxListenerChildOfClosedRootIsBornClosed(t *testing.T) {
	m, root := closeMux(t)
	root.Close()
	closeWantErr(t, closeAccept(closeMatch(t, m, "GET ")))
}

// closePush hands the root a pipe whose client end writes payload.
func closePush(t *testing.T, root *closeRoot, payload string) net.Conn {
	client, server := net.Pipe()
	t.Cleanup(func() { client.Close() })
	go client.Write([]byte(payload))
	root.conns <- server
	return client
}

func closeAcceptOne(t *testing.T, child net.Listener) net.Conn {
	t.Helper()
	conn, err := closeWait(t, closeAccept(child))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func closeRead(t *testing.T, conn net.Conn, want string) {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	got := make([]byte, len(want))
	if _, err := io.ReadFull(conn, got); err != nil || string(got) != want {
		t.Fatalf("read %q, %v; want %q", got, err, want)
	}
}

func TestMuxListenerAcceptedConnSurvivesClose(t *testing.T) {
	m, root := closeMux(t)
	httpL := closeMatch(t, m, pattern.Pattern[pattern.HTTP]...)
	other, err := m.Unmatched()
	if err != nil {
		t.Fatal(err)
	}
	const req, junk = "GET / HTTP/1.1\r\nHost: x\r\n\r\n", "zzz-unmatched\r\n"
	client := closePush(t, root, req)
	conn := closeAcceptOne(t, httpL)
	closeRead(t, conn, req) // sniffed prefix is replayed to the accepted conn
	closePush(t, root, junk)
	closeRead(t, closeAcceptOne(t, other), junk)

	httpL.Close()
	root.Close()
	closeWantErr(t, closeAccept(other))
	go conn.Write([]byte("pong"))
	closeRead(t, client, "pong")
	go client.Write([]byte("ping"))
	closeRead(t, conn, "ping")
}
