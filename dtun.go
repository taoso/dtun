package dtun

import (
	"encoding/json"
	"io"
	"log"
	"net"
	"os"
	"os/exec"

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
	local4 netaddr.IP
	peer4  netaddr.IP
	local6 netaddr.IP
	peer6  netaddr.IP
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

func NewTUN(c *dtls.Conn, local4, peer4, local6, peer6 netaddr.IP) *TUN {
	id := string(c.ConnectionState().IdentityHint)

	tun, err := water.New(water.Config{DeviceType: water.TUN})
	if err != nil {
		panic(err)
	}

	log.Printf("%s -> %s", local4, peer4)
	log.Printf("%s -> %s", local6, peer6)

	cmd("link", "set", tun.Name(), "up")
	cmd("addr", "add", local4.String()+"/32", "peer", peer4.String(), "dev", tun.Name())
	cmd("addr", "add", local6.String()+"/128", "peer", peer6.String(), "dev", tun.Name())

	return &TUN{
		id:     id,
		local4: local4,
		peer4:  peer4,
		local6: local6,
		peer6:  peer6,
		c:      c,
		Tun:    tun,
	}
}

func cmd(args ...string) {
	cmd := exec.Command(ipcmd, args...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	if err := cmd.Run(); err != nil {
		panic(err)
	}
}
