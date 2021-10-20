package pattern

//go:generate go run pattern_gen.go
//go:generate go fmt .

const (
	TLS    = "tls"
	SOCKS4 = "socks4"
	SOCKS5 = "socks5"
	HTTP   = "http"
	HTTP2  = "http2"
	SSH    = "ssh"
)

var Pattern = map[string][]string{
	TLS: {
		"\x16\x03\x00",
		"\x16\x03\x01",
		"\x16\x03\x02",
		"\x16\x03\x03",
		"\x16\x03\x04",
	},
	SOCKS4: {
		"\x04\x01",
		"\x04\x02",
	},
	SOCKS5: {
		"\x05\x01",
		"\x05\x02",
		"\x05\x03",
	},
	HTTP: {
		"GET ",
		"HEAD ",
		"POST ",
		"PUT ",
		"PATCH ",
		"DELETE ",
		"CONNECT ",
		"OPTIONS ",
		"TRACE ",
	},
	HTTP2: {
		"PRI * HTTP/2.0",
	},
	SSH: {
		"SSH-",
	},
}
