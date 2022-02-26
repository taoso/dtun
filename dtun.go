package dtun

import (
	"encoding/json"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"

	"github.com/pion/dtls/v2"
	"github.com/songgao/water"
	"inet.af/netaddr"
)

const MTU = 1500

var ipcmd string

func init() {
	var err error
	ipcmd, err = exec.LookPath("ip")
	if err != nil {
		panic(err)
	}
}

type TUN struct {
	id     string
	local4 netaddr.IPPrefix
	local6 netaddr.IPPrefix
	peer4  netaddr.IPPrefix
	peer6  netaddr.IPPrefix
	c      *dtls.Conn
	Tun    *water.Interface
}

func (t *TUN) Name() string {
	return t.Tun.Name()
}

func (t *TUN) Close() {
	t.c.Close()
	t.Tun.Close()
}

type Meta struct {
	Local4 string
	Peer4  string
	Local6 string
	Peer6  string
	Routes string
}

func (m *Meta) Read(c io.Reader) error {
	buf := make([]byte, MTU)
	n, err := c.Read(buf)
	if err != nil {
		return err
	}

	return json.Unmarshal(buf[:n], m)
}

func (m *Meta) Send(c io.Writer) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	_, err = c.Write(b)
	return err
}

func (t *TUN) SendIP() error {
	m := Meta{
		Local4: t.peer4.String(),
		Peer4:  t.local4.String(),
		Local6: t.peer6.String(),
		Peer6:  t.local6.String(),
	}
	return m.Send(t.c)
}

func (t *TUN) SetRoute() error {
	var m Meta

	if err := m.Read(t.c); err != nil {
		return err
	}

	if _, _, err := net.ParseCIDR(m.Routes); err != nil {
		log.Println("parse local network error", err)
		return err
	}
	args := []string{"route", "add", m.Routes, "via", t.peer4.String()}
	if err := exec.Command(ipcmd, args...).Run(); err != nil {
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
		io.CopyBuffer(t.c, t.Tun, buf)
	}()

	buf := make([]byte, MTU)
	io.CopyBuffer(t.Tun, t.c, buf)
}

func NewTUN(c *dtls.Conn, local4, peer4, local6, peer6 netaddr.IPPrefix, client bool) *TUN {
	id := string(c.ConnectionState().IdentityHint)

	tun, err := water.New(water.Config{DeviceType: water.TUN})
	if err != nil {
		panic(err)
	}

	cmd("link", "set", tun.Name(), "up", "mtu", "1280")
	cmd("addr", "add", local4.IP().String()+"/32", "peer", peer4.IP().String(), "dev", tun.Name())
	if client {
		cmd("addr", "add", local6.String(), "dev", tun.Name())
	} else {
		cmd("addr", "add", local6.IP().String()+"/128", "peer", peer6.IP().String(), "dev", tun.Name())
	}

	return &TUN{
		id:     id,
		local4: local4,
		local6: local6,
		peer4:  peer4,
		peer6:  peer6,
		c:      c,
		Tun:    tun,
	}
}

func cmd(args ...string) {
	log.Println(ipcmd, strings.Join(args, " "))
	cmd := exec.Command(ipcmd, args...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	if err := cmd.Run(); err != nil {
		panic(err)
	}
}
