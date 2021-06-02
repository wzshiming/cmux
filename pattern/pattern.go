package pattern

//go:generate go run pattern_gen.go

var Pattern = map[string][]string{
	"tls": {
		"\x16\x03\x00",
		"\x16\x03\x01",
		"\x16\x03\x02",
		"\x16\x03\x03",
	},
	"socks4": {
		"\x04\x01",
		"\x04\x02",
	},
	"socks5": {
		"\x05\x01",
		"\x05\x02",
		"\x05\x03",
	},
	"http": {
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
	"http2": {
		"PRI * HTTP/2.0",
	},
	"ssh": {
		"SSH-",
	},
}
