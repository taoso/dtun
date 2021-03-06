package main

import (
	"flag"
	"fmt"
	"log"
	"net"

	"github.com/lvht/dtun"
	"github.com/lvht/dtun/ip"
	"github.com/pion/dtls/v2"
)

var listen, host, key, id string
var port int

func init() {
	flag.StringVar(&listen, "listen", "0.0.0.0", "server listen address")
	flag.StringVar(&host, "host", "", "server address(client only)")
	flag.StringVar(&key, "key", "", "pre-shared key(psk)")
	flag.StringVar(&id, "id", "", "psk hint")
	flag.IntVar(&port, "port", 443, "server port")
}

func main() {
	flag.Parse()

	if host != "" {
		dial()
	} else {
		listenTUN()
	}
}

func dial() {
	config := &dtls.Config{
		PSK: func(hint []byte) ([]byte, error) {
			log.Printf("Server's hint: %s \n", string(hint))
			return []byte(key), nil
		},
		PSKIdentityHint:      []byte(id),
		CipherSuites:         []dtls.CipherSuiteID{dtls.TLS_PSK_WITH_AES_128_CCM_8},
		ExtendedMasterSecret: dtls.RequireExtendedMasterSecret,
	}

	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		panic(err)
	}
	addr.Port = port

	c, err := dtls.Dial("udp", addr, config)
	if err != nil {
		panic(err)
	}

	buf := make([]byte, dtun.MTU)
	if _, err := c.Read(buf); err != nil {
		log.Panic(err)
	}

	local := net.IP(buf[:4])
	peer := net.IP(buf[4:8])

	dtun.Tun(c, local, peer)
}

func listenTUN() {
	addr := &net.UDPAddr{IP: net.ParseIP(listen), Port: port}

	config := &dtls.Config{
		PSK: func(hint []byte) ([]byte, error) {
			log.Printf("Client's hint: %s \n", string(hint))
			return []byte(key), nil
		},
		PSKIdentityHint:      []byte(id),
		CipherSuites:         []dtls.CipherSuiteID{dtls.TLS_PSK_WITH_AES_128_CCM_8},
		ExtendedMasterSecret: dtls.RequireExtendedMasterSecret,
	}

	ln, err := dtls.Listen("udp", addr, config)
	if err != nil {
		panic(err)
	}

	for {
		c, err := ln.Accept()
		if err != nil {
			panic(err)
		}

		local := ip.Reserve()
		peer := ip.Reserve()

		_, err = c.Write(append(peer.To4(), local.To4()...))
		if err != nil {
			log.Println(err)
			continue
		}

		go dtun.Tun(c, local, peer)
	}
}
