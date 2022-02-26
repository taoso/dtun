package dtun

import (
	"testing"

	"inet.af/netaddr"
)

func TestPool(t *testing.T) {
	for _, c := range []struct{ r1, s1, s2 string }{
		{r1: "10.0.0.0/30", s1: "10.0.0.1", s2: "10.0.0.2"},
		{r1: "fc00::/126", s1: "fc00::1", s2: "fc00::2"},
	} {
		p := NewAddrPool(c.r1)

		ip1 := p.Next()
		if ip1 != netaddr.MustParseIP(c.s1) {
			t.Fatal("invalid ip", ip1)
		}

		ip2 := p.Next()
		if ip2 != netaddr.MustParseIP(c.s2) {
			t.Fatal("invalid ip", ip2)
		}

		if ip3 := p.Next(); !ip3.IsZero() {
			t.Fatal("none empty ip", ip3)
		}

		p.Release(ip2)
		ip4 := p.Next()
		if ip4 != netaddr.MustParseIP(c.s2) {
			t.Fatal("invalid ip", ip4)
		}
	}
}
