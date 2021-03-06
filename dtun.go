package dtun

import (
	"io"
	"log"
	"net"
	"os/exec"

	"github.com/lvht/dtun/ip"
	"github.com/songgao/water"
)

const MTU = 1500

func Tun(c net.Conn, local, peer net.IP) {
	defer ip.Release(local, peer)

	tun, err := water.New(water.Config{DeviceType: water.TUN})
	if err != nil {
		log.Println("create tun faild", err)
		return
	}
	defer tun.Close()

	log.Printf("%s -> %s", local, peer)

	args := []string{"link", "set", tun.Name(), "up"}
	if err = exec.Command(ipcmd, args...).Run(); err != nil {
		log.Println("link set up", err)
		return
	}

	args = []string{"addr", "add", local.String(), "peer", peer.String(), "dev", tun.Name()}
	if err = exec.Command(ipcmd, args...).Run(); err != nil {
		log.Println("addr add faild", err)
		return
	}

	go func() {
		defer tun.Close()
		buf := make([]byte, MTU)
		io.CopyBuffer(c, tun, buf)
	}()

	buf := make([]byte, MTU)
	io.CopyBuffer(tun, c, buf)
}

func setRoute() {
	// n, err := c.Read(buf)
	// br := bytes.NewReader(buf)
	//
	// if local := req.Header.Get("Local-Network"); local != "" {
	// 	if _, _, err = net.ParseCIDR(local); err != nil {
	// 		log.Println("parse local network faild", err)
	// 		return
	// 	}
	// 	args = []string{"route", "add", local, "via", clientIP.String()}
	// 	if err = exec.Command(ipcmd, args...).Run(); err != nil {
	// 		log.Println("route add faild", err)
	// 		return
	// 	}
	// }
}
