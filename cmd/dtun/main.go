package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/pion/dtls/v2"
	"github.com/taoso/dtun"
	"inet.af/netaddr"
)

var listen, connect, key, id string
var peernet, up string
var pool6, pool4 string

func init() {
	flag.StringVar(&listen, "listen", "0.0.0.0:443", "server listen address(server)")
	flag.StringVar(&pool6, "pool6", "fc00::/120", "client ipv6 pool(server)")
	flag.StringVar(&pool4, "pool4", "10.0.0.0/24", "client ipv4 pool(server)")
	flag.StringVar(&connect, "connect", "", "server address(client)")
	flag.StringVar(&peernet, "peernet", "empty", "client local ipv4 network")
	flag.StringVar(&up, "up", "", "client up script")
	flag.StringVar(&key, "key", "", "pre-shared key(psk)")
	flag.StringVar(&id, "id", "dtun", "psk hint")
}

func main() {
	flag.Parse()

	if key == "" {
		panic("key is required")
	}

	if connect != "" {
		dialTUN()
	} else {
		listenTUN()
	}
}

var tun *dtun.TUN

func dialTUN() {
	config := &dtls.Config{
		PSK: func(hint []byte) ([]byte, error) {
			log.Printf("Server's hint: %s \n", string(hint))
			return []byte(key), nil
		},
		PSKIdentityHint: []byte(id),
		CipherSuites:    []dtls.CipherSuiteID{dtls.TLS_PSK_WITH_AES_128_CCM_8},
	}

	addr, err := net.ResolveUDPAddr("udp", connect)
	if err != nil {
		panic(err)
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		if tun != nil {
			tun.Close()
		}
		os.Exit(0)
	}()

	goto dial // skip sleep for first time
loop:
	time.Sleep(5 * time.Second)
dial:
	log.Println("dialing to", addr)
	c, err := dtls.Dial("udp", addr, config)
	if err != nil {
		log.Println("Dial error", err)
		goto loop
	}

	var m dtun.Meta
	if err := m.Read(c); err != nil {
		log.Println("Meta Read error", err)
		goto loop
	}

	local4, err := netaddr.ParseIP(m.Local4)
	if err != nil {
		log.Println("parse local4 error", err)
		goto loop
	}
	peer4, err := netaddr.ParseIP(m.Peer4)
	if err != nil {
		log.Println("parse peer4 error", err)
		goto loop
	}
	local6, err := netaddr.ParseIP(m.Local6)
	if err != nil {
		log.Println("parse local6 error", err)
		goto loop
	}
	peer6, err := netaddr.ParseIP(m.Peer6)
	if err != nil {
		log.Println("parse peer6 error", err)
		goto loop
	}

	tun = dtun.NewTUN(c, local4, peer4, local6, peer6)

	r := dtun.Meta{Routes: peernet}

	if err = r.Send(c); err != nil {
		log.Println("Meta Send error", err)
		goto loop
	}

	if up != "" {
		cmd := exec.Command(up)
		cmd.Env = []string{
			fmt.Sprintf("TUN=%s", tun.Name()),
			fmt.Sprintf("PEER_IP4=%s", peer4),
			fmt.Sprintf("LOCAL_IP4=%s", local4),
			fmt.Sprintf("PEER_IP6=%s", peer6),
			fmt.Sprintf("LOCAL_IP6=%s", local6),
		}
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err = cmd.Run(); err != nil {
			log.Panic(err)
		}
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		buf := make([]byte, dtun.MTU)
		io.CopyBuffer(c, tun.Tun, buf)
	}()

	buf := make([]byte, dtun.MTU)
	io.CopyBuffer(tun.Tun, c, buf)
	tun.Close()

	wg.Wait()
	goto loop
}

func listenTUN() {
	config := &dtls.Config{
		PSK: func(hint []byte) ([]byte, error) {
			log.Printf("Client's hint: %s \n", string(hint))
			return []byte(key), nil
		},
		PSKIdentityHint: []byte(id),
		CipherSuites:    []dtls.CipherSuiteID{dtls.TLS_PSK_WITH_AES_128_CCM_8},
	}

	addr, err := net.ResolveUDPAddr("udp", listen)
	if err != nil {
		panic(err)
	}

	log.Println("listening on", addr)

	ln, err := dtls.Listen("udp", addr, config)
	if err != nil {
		panic(err)
	}

	v4Pool := dtun.NewAddrPool(pool4)
	v6Pool := dtun.NewAddrPool(pool6)

	v4gw := v4Pool.Next()
	v6gw := v6Pool.Next()

	for {
		c, err := ln.Accept()
		if err != nil {
			log.Println("Accept error", err)
			continue
		}

		cc := c.(*dtls.Conn)

		v4 := v4Pool.Next()
		v6 := v6Pool.Next()

		t := dtun.NewTUN(cc, v4gw, v4, v6gw, v6)

		go func() {
			defer v4Pool.Release(v4)
			defer v6Pool.Release(v6)

			if err := t.SendIP(); err != nil {
				fmt.Println("SendIP", err)
				return
			}

			if err := t.SetRoute(); err != nil {
				fmt.Println("SetRoute", err)
				return
			}

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()
				buf := make([]byte, dtun.MTU)
				io.CopyBuffer(c, t.Tun, buf)
			}()

			buf := make([]byte, dtun.MTU)
			io.CopyBuffer(t.Tun, c, buf)
			t.Close()

			wg.Wait()
		}()
	}
}
