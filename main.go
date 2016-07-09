package main

import (
	"flag"
	"log"
	"os"
	"strings"
	"time"

	"github.com/tonyluj/socks5-relay/client"
	"github.com/tonyluj/socks5-relay/server"
)

func main() {
	var (
		mode       string
		addr       string
		serverAddr string
	)

	flag.StringVar(&mode, "mode", "", "choose run mode")
	flag.StringVar(&addr, "client", "", "choose client listen addr and port")
	flag.StringVar(&serverAddr, "server", "", "choose server addr and port")
	flag.Parse()
	if mode == "" || addr == "" || serverAddr == "" {
		flag.PrintDefaults()
		os.Exit(0)
	}

	switch strings.ToLower(mode) {
	case "client":
		client, err := client.New(addr, serverAddr, time.Second*5*60)
		if err != nil {
			log.Fatal(err)
		}
		err = client.Listen()
		if err != nil {
			log.Println(err)
		}
	case "server":
		server, err := server.New(addr, time.Second*5*60)
		if err != nil {
			log.Fatal(err)
		}
		err = server.Listen()
		if err != nil {
			log.Println(err)
		}
	}
}
