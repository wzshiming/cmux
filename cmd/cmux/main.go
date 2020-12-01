package main

import (
	"flag"
	"log"
	"net"
	"sync"

	"github.com/wzshiming/cmux"
	"github.com/wzshiming/cmux/pattern"
)

var address string

func init() {
	flag.StringVar(&address, "a", ":8080", "listen on the address")
	flag.Parse()
}

func main() {
	log.Println("listen", address)
	n, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalln(err)
	}
	mux := cmux.NewMuxListener(n)

	wg := sync.WaitGroup{}

	regs := map[string]string{
		"tls":       pattern.TLS,
		"socks4":    pattern.Socks4,
		"socks5":    pattern.Socks5,
		"http":      pattern.HTTP,
		"http2":     pattern.HTTP2,
		"ssh":       pattern.SSH,
		"unmatched": "",
	}
	wg.Add(len(regs))
	for key, reg := range regs {
		var listener net.Listener
		if reg == "" {
			listener, err = mux.Unmatched()
			if err != nil {
				log.Fatalln(key, err)
			}
		} else {
			listener, err = mux.MatchRegexp(reg)
			if err != nil {
				log.Fatalln(key, err)
			}
		}

		go func(key string, listener net.Listener) {
			defer wg.Done()
			for {
				conn, err := listener.Accept()
				if err != nil {
					log.Println(key, err)
					return
				}
				log.Println(key, conn.RemoteAddr())
				conn.Close()
			}
		}(key, listener)
	}
	wg.Wait()
}
