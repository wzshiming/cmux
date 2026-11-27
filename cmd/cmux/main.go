package main

import (
	"flag"
	"log"
	"net"
	"sort"
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

	handle := func(key string, listener net.Listener) {
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
	}

	regs := pattern.Pattern
	wg.Add(len(regs) + 1)
	listener, err := mux.Unmatched()
	if err != nil {
		log.Fatalln("unmatched", err)
	}
	go handle("unmatched", listener)
	// Register in a stable order, so that http_proxy is registered after http
	// and takes the prefixes they share.
	keys := make([]string, 0, len(regs))
	for key := range regs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		reg := regs[key]
		listener, err := mux.MatchPrefix(reg...)
		if err != nil {
			log.Fatalln(key, err)
		}
		go handle(key, listener)
	}
	wg.Wait()
}
