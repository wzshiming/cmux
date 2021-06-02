// +build ignore

package main

import (
	"bytes"
	"os"
	"strconv"

	"github.com/wzshiming/crun"
)

func main() {
	os.WriteFile("pattern.go", Generate(), 0666)
}

func Generate() []byte {
	buf := bytes.NewBuffer(nil)
	buf.WriteString("package pattern\n\n")
	buf.WriteString("//go:generate go run pattern_gen.go\n\n")
	buf.WriteString("var Pattern = map[string][]string{\n")
	for _, pattern := range patterns {
		key := pattern[0]
		reg := crun.MustCompile(pattern[1])

		buf.WriteString("\t")
		buf.WriteString(strconv.Quote(key))
		buf.WriteString(": {\n")
		reg.Range(func(s string) bool {
			buf.WriteString("\t\t")
			buf.WriteString(strconv.Quote(s))
			buf.WriteString(",\n")
			return true
		})
		buf.WriteString("\t},\n")
	}
	buf.WriteString("}\n")

	return buf.Bytes()
}

// RegisterRegexp pattern
var patterns = [...][2]string{

	// tls
	// 0       1       2       3       4       5       6       7       8
	// +-------+-------+-------+-------+-------+-------+-------+-------+-------+
	// |record |    version    |                   ...                         |
	// +-------+---------------+---------------+-------------------------------+
	{"tls", "^\x16\x03(\x00|\x01|\x02|\x03)"},

	// socks
	// 0       1       2       3       4       5       6       7       8
	// +-------+-------+-------+-------+-------+-------+-------+-------+-------+
	// |version|command|                       ...                             |
	// +-------+-------+-------------------------------------------------------+
	{"socks4", "^\x04(\x01|\x02)"},
	{"socks5", "^\x05(\x01|\x02|\x03)"},

	{"http", "^(GET|HEAD|POST|PUT|PATCH|DELETE|CONNECT|OPTIONS|TRACE) "},
	{"http2", "^PRI \\* HTTP/2\\.0"},

	{"ssh", "^SSH-"},
}
