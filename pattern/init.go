package pattern

func init() {
	for _, pattern := range patterns {
		Register(pattern[0], pattern[1])
	}
}

// RegisterRegexp pattern
var patterns = [...][2]string{

	// tls
	// 0       1       2       3       4       5       6       7       8
	// +-------+-------+-------+-------+-------+-------+-------+-------+-------+
	// |record |    version    |                   ...                         |
	// +-------+---------------+---------------+-------------------------------+
	{keyTLS, "^\u0016\u0003(\u0000|\u0001|\u0002|\u0003)"},

	// socks
	// 0       1       2       3       4       5       6       7       8
	// +-------+-------+-------+-------+-------+-------+-------+-------+-------+
	// |version|command|                       ...                             |
	// +-------+-------+-------------------------------------------------------+
	{keySocks4, "^\u0004(\u0001|\u0002)"},
	{keySocks5, "^\u0005(\u0001|\u0002|\u0003)"},

	{keyHTTP, "^(GET|HEAD|POST|PUT|PATCH|DELETE|CONNECT|OPTIONS|TRACE) "},
	{keyHTTP2, "^PRI \\* HTTP/2\\.0"},

	{keySSH, "^SSH-"},
}

var (
	keyTLS    = "tls"
	keySocks4 = "socks4"
	keySocks5 = "socks5"
	keyHTTP   = "http"
	keyHTTP2  = "http2"
	keySSH    = "ssh"

	TLS    string
	Socks4 string
	Socks5 string
	HTTP   string
	HTTP2  string
	SSH    string
)

func init() {
	TLS, _ = Get(keyTLS)
	Socks4, _ = Get(keySocks4)
	Socks5, _ = Get(keySocks5)
	HTTP, _ = Get(keyHTTP)
	HTTP2, _ = Get(keyHTTP2)
	SSH, _ = Get(keySSH)
}
