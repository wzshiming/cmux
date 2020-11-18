package shunt

import (
	"io"
	"net"
)

func UnwrapUnreadConn(conn net.Conn) (net.Conn, []byte) {
	if us, ok := conn.(*unreadConn); ok {
		_, prefix := UnwrapUnread(us.Reader)
		return us.Conn, prefix
	}
	return conn, nil
}

func UnreadConn(conn net.Conn, prefix []byte) net.Conn {
	if len(prefix) == 0 {
		return conn
	}
	if us, ok := conn.(*unreadConn); ok {
		us.Reader = Unread(us.Reader, prefix)
		return us
	}
	return &unreadConn{
		Reader: Unread(conn, prefix),
		Conn:   conn,
	}
}

type unreadConn struct {
	io.Reader
	net.Conn
}

func (c *unreadConn) Read(p []byte) (n int, err error) {
	return c.Reader.Read(p)
}

func UnwrapUnread(reader io.Reader) (io.Reader, []byte) {
	if u, ok := reader.(*unread); ok {
		return u.reader, u.prefix
	}
	return reader, nil
}

func Unread(reader io.Reader, prefix []byte) io.Reader {
	if len(prefix) == 0 {
		return reader
	}
	if ur, ok := reader.(*unread); ok {
		ur.prefix = append(prefix, ur.prefix...)
		return reader
	}
	return &unread{
		prefix: prefix,
		reader: reader,
	}
}

type unread struct {
	prefix []byte
	reader io.Reader
}

func (u *unread) Read(p []byte) (n int, err error) {
	if len(u.prefix) == 0 {
		return u.reader.Read(p)
	}
	n = copy(p, u.prefix)
	if n <= len(u.prefix) {
		u.prefix = u.prefix[n:]
		return n, nil
	}
	a, err := u.reader.Read(p[n:])
	if err == io.EOF {
		err = nil
	}
	n += a
	return n, err
}
