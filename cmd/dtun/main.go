package main

import (
	"flag"
	"fmt"
	"log"
	"net"

	"github.com/lvht/dtun"
	"github.com/pion/dtls/v2"
)

var listen, host, key, id, peernet string
var port int

func init() {
	flag.StringVar(&peernet, "peernet", "empty", "client local network")
	flag.StringVar(&listen, "listen", "0.0.0.0", "server listen address")
	flag.StringVar(&host, "host", "", "server address(client only)")
	flag.StringVar(&key, "key", "", "pre-shared key(psk)")
	flag.StringVar(&id, "id", "dtun", "psk hint")
	flag.IntVar(&port, "port", 443, "server port")
}

func main() {
	flag.Parse()

	if key == "" {
		panic("key is required")
	}

	if host != "" {
		dialTUN()
	} else {
		listenTUN()
	}
}

func dialTUN() {
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

	log.Println("dialing to ", addr)

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

	tun := dtun.NewTUN(c, local, peer)

	// send local network, so the peer can ping
	if _, err = c.Write([]byte(peernet)); err != nil {
		log.Panic(err)
	}

	tun.Loop()
}

func listenTUN() {
	addr := &net.UDPAddr{IP: net.ParseIP(listen), Port: port}

	config := &dtls.Config{
		PSK: func(hint []byte) ([]byte, error) {
			dtun.CleanTUN(string(hint))
			log.Printf("Client's hint: %s \n", string(hint))
			return []byte(key), nil
		},
		PSKIdentityHint:      []byte(id),
		CipherSuites:         []dtls.CipherSuiteID{dtls.TLS_PSK_WITH_AES_128_CCM_8},
		ExtendedMasterSecret: dtls.RequireExtendedMasterSecret,
	}

	log.Println("listening on ", addr)

	ln, err := dtls.Listen("udp", addr, config)
	if err != nil {
		panic(err)
	}

	for {
		c, err := ln.Accept()
		if err != nil {
			panic(err)
		}

		cc := c.(*dtls.Conn)

		tun := dtun.NewTUN(cc, nil, nil)

		if err := tun.SendIP(); err != nil {
			log.Println("SendIP error", err)
			tun.Close()
			continue
		}

		if err := tun.SetRoute(); err != nil {
			log.Println("SetRoute error", err)
			tun.Close()
			continue
		}

		go tun.Loop()
	}
}
