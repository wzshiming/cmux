package cmux

import (
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"testing/iotest"
	"time"
)

func TestUnreadReadsPrefixThenReader(t *testing.T) {
	reader := Unread(strings.NewReader("world"), []byte("hello"))
	if err := iotest.TestReader(reader, []byte("helloworld")); err != nil {
		t.Fatal(err)
	}
}

func TestUnreadByteAtATimeAcrossBoundary(t *testing.T) {
	reader := Unread(strings.NewReader("world"), []byte("hello"))
	buf := make([]byte, 1)
	var got []byte
	for {
		count, err := reader.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil || count != 1 {
			t.Fatalf("Read(1) after %q = %d, %v; want 1, nil", got, count, err)
		}
		got = append(got, buf[0])
	}
	if string(got) != "helloworld" {
		t.Fatalf("read %q; want helloworld", got)
	}
}

func TestUnreadZeroLengthReadKeepsPrefix(t *testing.T) {
	reader := Unread(strings.NewReader("world"), []byte("hello"))
	if count, err := reader.Read(nil); count != 0 || err != nil {
		t.Fatalf("Read(nil) = %d, %v; want 0, nil", count, err)
	}
	if _, prefix := UnwrapUnread(reader); string(prefix) != "hello" {
		t.Fatalf("prefix after Read(nil) = %q; want hello", prefix)
	}
}

func TestUnreadShortReadDoesNotTouchReader(t *testing.T) {
	errUnderlying := errors.New("underlying read")
	reader := Unread(iotest.ErrReader(errUnderlying), []byte("he"))
	buf := make([]byte, 5)
	if count, err := reader.Read(buf); count != 2 || err != nil || string(buf[:2]) != "he" {
		t.Fatalf("Read(5) = %d, %v, %q; want 2, nil, he", count, err, buf[:count])
	}
	if _, prefix := UnwrapUnread(reader); prefix != nil {
		t.Fatalf("drained prefix = %#v; want nil", prefix)
	}
	if count, err := reader.Read(buf); count != 0 || !errors.Is(err, errUnderlying) {
		t.Fatalf("Read after prefix = %d, %v; want 0, %v", count, err, errUnderlying)
	}
}

func TestUnreadStackingPrepends(t *testing.T) {
	inner := strings.NewReader("!")
	reader := Unread(Unread(inner, []byte("world")), []byte("hello"))
	if unwrapped, prefix := UnwrapUnread(reader); unwrapped != inner || string(prefix) != "helloworld" {
		t.Fatalf("UnwrapUnread = %T, %q; want *strings.Reader, helloworld", unwrapped, prefix)
	}
	if got, err := io.ReadAll(reader); err != nil || string(got) != "helloworld!" {
		t.Fatalf("ReadAll = %q, %v; want helloworld!, nil", got, err)
	}
}

func TestUnwrapUnreadReturnsRemainingPrefixAndReader(t *testing.T) {
	inner := strings.NewReader("world")
	reader := Unread(inner, []byte("hello"))
	if _, err := io.ReadFull(reader, make([]byte, 2)); err != nil {
		t.Fatal(err)
	}
	if unwrapped, prefix := UnwrapUnread(reader); unwrapped != inner || string(prefix) != "llo" {
		t.Fatalf("UnwrapUnread = %T, %q; want *strings.Reader, llo", unwrapped, prefix)
	}
}

func TestUnreadConnPreservesConnAndRemainingPrefix(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	if err := server.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	conn := UnreadConn(server, []byte("hello"))
	if _, err := io.ReadFull(conn, make([]byte, 2)); err != nil {
		t.Fatal(err)
	}
	if unwrapped, prefix := UnwrapUnreadConn(conn); unwrapped != server || string(prefix) != "llo" {
		t.Fatalf("UnwrapUnreadConn = %T, %q; want server pipe, llo", unwrapped, prefix)
	}
	if stacked := UnreadConn(conn, []byte("he")); stacked != conn {
		t.Fatalf("UnreadConn on wrapped conn = %T; want same conn", stacked)
	}
	go client.Write([]byte("world"))
	buf := make([]byte, 10)
	if _, err := io.ReadFull(conn, buf); err != nil || string(buf) != "helloworld" {
		t.Fatalf("ReadFull = %q, %v; want helloworld, nil", buf, err)
	}
}

func TestUnreadEmptyPrefixPassthrough(t *testing.T) {
	inner := strings.NewReader("x")
	if reader := Unread(inner, nil); reader != inner {
		t.Fatalf("Unread(inner, nil) = %T; want *strings.Reader", reader)
	}
	if unwrapped, prefix := UnwrapUnread(inner); unwrapped != inner || prefix != nil {
		t.Fatalf("UnwrapUnread(plain) = %T, %#v; want *strings.Reader, nil", unwrapped, prefix)
	}
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	if conn := UnreadConn(server, []byte{}); conn != server {
		t.Fatalf("UnreadConn(server, empty) = %T; want server pipe", conn)
	}
	if unwrapped, prefix := UnwrapUnreadConn(server); unwrapped != server || prefix != nil {
		t.Fatalf("UnwrapUnreadConn(plain) = %T, %#v; want server pipe, nil", unwrapped, prefix)
	}
}
