package dtun

import (
	"io"
	"log"
	"net"
	"os/exec"
	"sync"

	"github.com/pion/dtls/v2"
	"github.com/songgao/water"
	"github.com/taoso/dtun/ip"
)

const MTU = 1500

var tuns = sync.Map{}

var ipcmd string

func init() {
	var err error
	ipcmd, err = exec.LookPath("ip")
	if err != nil {
		panic(err)
	}
}

type TUN struct {
	id    string
	local net.IP
	peer  net.IP
	c     *dtls.Conn
	tun   *water.Interface
}

func (t *TUN) Name() string {
	return t.tun.Name()
}

func (t *TUN) Close() {
	t.c.Close()
	t.tun.Close()
	ip.Release(t.local, t.peer)
}

func (t *TUN) SendIP() error {
	_, err := t.c.Write(append(t.peer.To4(), t.local.To4()...))
	return err
}

func (t *TUN) SetRoute() error {
	buf := make([]byte, MTU)
	n, err := t.c.Read(buf)
	if err != nil {
		return err
	}

	local := string(buf[:n])
	if local == "empty" {
		return nil
	}

	if _, _, err = net.ParseCIDR(local); err != nil {
		log.Println("parse local network faild", err)
		return err
	}
	args := []string{"route", "add", local, "via", t.peer.String()}
	if err = exec.Command(ipcmd, args...).Run(); err != nil {
		log.Println("route add faild", err)
		return err
	}
	return nil
}

func (t *TUN) Loop() {
	defer t.Close()

	go func() {
		defer t.Close()
		buf := make([]byte, MTU)
		io.CopyBuffer(t.c, t.tun, buf)
	}()

	buf := make([]byte, MTU)
	io.CopyBuffer(t.tun, t.c, buf)
}

func NewTUN(c *dtls.Conn, local, peer net.IP) *TUN {
	id := string(c.ConnectionState().IdentityHint)

	if local == nil || peer == nil {
		local = ip.Reserve()
		peer = ip.Reserve()
	}

	tun, err := water.New(water.Config{DeviceType: water.TUN})
	if err != nil {
		panic(err)
	}

	log.Printf("%s -> %s", local, peer)

	args := []string{"link", "set", tun.Name(), "up"}
	if err := exec.Command(ipcmd, args...).Run(); err != nil {
		panic(err)
	}

	args = []string{"addr", "add", local.String(), "peer", peer.String(), "dev", tun.Name()}
	if err := exec.Command(ipcmd, args...).Run(); err != nil {
		panic(err)
	}

	t := &TUN{id: id, local: local, peer: peer, c: c, tun: tun}
	tuns.Store(id, t)
	return t
}

func CleanTUN(id string) {
	if v, ok := tuns.Load(id); ok {
		tun := v.(*TUN)
		tun.Close()
		tuns.Delete(id)
	}
}
