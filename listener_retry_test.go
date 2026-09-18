package cmux

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

// retryErr is a net.Error with a scripted Temporary result.
type retryErr struct{ temporary bool }

func (e retryErr) Error() string   { return fmt.Sprintf("retry err (temporary=%t)", e.temporary) }
func (e retryErr) Timeout() bool   { return false }
func (e retryErr) Temporary() bool { return e.temporary }

// retryConn is an inert conn whose payload matches the "retry" prefix.
type retryConn struct {
	net.Conn
	r *strings.Reader
}

func newRetryConn() *retryConn { return &retryConn{r: strings.NewReader("retry")} }

// Read mirrors net.Conn: a zero-length read yields 0, nil rather than io.EOF.
func (c *retryConn) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	return c.r.Read(p)
}

func (c *retryConn) Close() error { return nil }

type retryStep struct {
	conn net.Conn
	err  error
}

// retryListener replays scripted Accept results, then reports net.ErrClosed.
type retryListener struct {
	steps []retryStep
	times []time.Time
}

func (l *retryListener) Accept() (net.Conn, error) {
	l.times = append(l.times, time.Now())
	if len(l.steps) == 0 {
		return nil, net.ErrClosed
	}
	s := l.steps[0]
	l.steps = l.steps[1:]
	return s.conn, s.err
}

func (l *retryListener) Close() error   { return nil }
func (l *retryListener) Addr() net.Addr { return nil }

// startRetry registers the "retry" handler and ErrHandler, then runs the accept loop.
func startRetry(steps []retryStep, errHandler func(error) bool) (*retryListener, chan net.Conn, chan struct{}) {
	l := &retryListener{steps: steps}
	m := NewMuxListener(l)
	m.ErrHandler = errHandler
	served := make(chan net.Conn, len(steps))
	m.mux.HandlePrefix(HandlerFunc(func(c net.Conn) { served <- c }), "retry")
	done := make(chan struct{})
	go func() {
		m.run()
		close(done)
	}()
	return l, served, done
}

func waitDone(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("run did not exit")
	}
}

func waitServed(t *testing.T, served <-chan net.Conn, want net.Conn) {
	t.Helper()
	select {
	case got := <-served:
		if got, _ = UnwrapUnreadConn(got); got != want {
			t.Fatalf("served conn %v, want %v", got, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("accepted conn was not served")
	}
}

func TestMuxListenerRunRetriesTemporaryAcceptError(t *testing.T) {
	temp := retryErr{temporary: true}
	for name, err := range map[string]error{"bare": temp, "wrapped": fmt.Errorf("wrapped: %w", temp)} {
		t.Run(name, func(t *testing.T) {
			conn := newRetryConn()
			l, served, done := startRetry([]retryStep{{err: err}, {conn: conn}}, nil)
			waitDone(t, done)
			if len(l.times) != 3 {
				t.Fatalf("Accept called %d times, want 3", len(l.times))
			}
			waitServed(t, served, conn)
		})
	}
}

func TestMuxListenerRunBacksOffTemporaryAcceptErrors(t *testing.T) {
	temp := retryStep{err: retryErr{temporary: true}}
	l, _, done := startRetry([]retryStep{temp, temp, temp}, nil)
	waitDone(t, done)
	if len(l.times) != 4 {
		t.Fatalf("Accept called %d times, want 4", len(l.times))
	}
	for i, want := range []time.Duration{5 * time.Millisecond, 10 * time.Millisecond, 20 * time.Millisecond} {
		if got := l.times[i+1].Sub(l.times[i]); got < want {
			t.Fatalf("retry %d waited %v, want at least %v", i+1, got, want)
		}
	}
}

func TestMuxListenerRunResetsBackoffAfterAccept(t *testing.T) {
	temp := retryStep{err: retryErr{temporary: true}}
	conn := newRetryConn()
	l, served, done := startRetry([]retryStep{temp, temp, temp, temp, temp, temp, {conn: conn}, temp}, nil)
	waitDone(t, done)
	if len(l.times) != 9 {
		t.Fatalf("Accept called %d times, want 9", len(l.times))
	}
	waitServed(t, served, conn)
	// Sixth retry waits 160ms; the retry after the conn must restart at 5ms, not 320ms.
	if before, after := l.times[6].Sub(l.times[5]), l.times[8].Sub(l.times[7]); after >= before {
		t.Fatalf("retry after accept waited %v, want less than %v", after, before)
	}
}

func TestNextDelayCapsAtOneSecond(t *testing.T) {
	var delay time.Duration
	want := []time.Duration{
		5 * time.Millisecond, 10 * time.Millisecond, 20 * time.Millisecond, 40 * time.Millisecond,
		80 * time.Millisecond, 160 * time.Millisecond, 320 * time.Millisecond, 640 * time.Millisecond,
		time.Second, time.Second,
	}
	for i, w := range want {
		if delay = nextDelay(delay); delay != w {
			t.Fatalf("delay %d = %v, want %v", i, delay, w)
		}
	}
}

func TestMuxListenerRunStopsOnNonTemporaryAcceptError(t *testing.T) {
	for _, err := range []error{retryErr{}, errors.New("plain")} {
		l, served, done := startRetry([]retryStep{{err: err}, {conn: newRetryConn()}}, nil)
		waitDone(t, done)
		if len(l.times) != 1 || len(served) != 0 {
			t.Fatalf("%v: Accept called %d times and served %d conns, want 1 and 0", err, len(l.times), len(served))
		}
	}
}

func TestMuxListenerErrHandlerFalseStopsTemporaryAcceptError(t *testing.T) {
	calls := 0
	steps := []retryStep{{err: retryErr{temporary: true}}, {conn: newRetryConn()}}
	l, served, done := startRetry(steps, func(error) bool { calls++; return false })
	waitDone(t, done)
	if calls != 1 || len(l.times) != 1 || len(served) != 0 {
		t.Fatalf("ErrHandler called %d times, Accept %d times, served %d conns; want 1, 1, 0", calls, len(l.times), len(served))
	}
}

func TestMuxListenerErrHandlerTrueRetriesAnyAcceptErrorWithoutDelay(t *testing.T) {
	conn := newRetryConn()
	var steps []retryStep
	for i := 0; i < 12; i++ {
		steps = append(steps, retryStep{err: retryErr{temporary: i%3 != 2}})
	}
	steps = append(steps, retryStep{err: errors.New("plain")}, retryStep{conn: conn})
	calls := 0
	start := time.Now()
	// True for every scripted error; false for the trailing net.ErrClosed.
	l, served, done := startRetry(steps, func(error) bool { calls++; return calls < len(steps) })
	waitDone(t, done)
	waitServed(t, served, conn)
	if calls != len(steps) || len(l.times) != len(steps)+1 {
		t.Fatalf("ErrHandler called %d times, Accept %d times; want %d and %d", calls, len(l.times), len(steps), len(steps)+1)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("retries took %v, want no default backoff", elapsed)
	}
}
