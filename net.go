package dtun

import (
	"sync"

	"inet.af/netaddr"
)

// AddrPool 网络地址池
type AddrPool struct {
	num  int
	next netaddr.IP
	p    netaddr.IPPrefix
	r    netaddr.IPRange
	m    sync.Mutex
	used map[netaddr.IP]bool
}

// Next 返回一个可用的网络地址
// 如果地址耗尽则返回空地址。
func (p *AddrPool) Next() netaddr.IP {
	p.m.Lock()
	defer p.m.Unlock()

	if len(p.used) == p.num {
		return netaddr.IP{}
	}

	for p.used[p.next] {
		p.next = p.next.Next()
		if p.next == p.r.To() {
			p.next = p.r.From().Next()
		}
	}
	p.used[p.next] = true

	return p.next
}

// NextPrefix 返回一个可用的网络地址，包含网段
// 如果地址耗尽则返回空地址。
func (p *AddrPool) NextPrefix() netaddr.IPPrefix {
	ip := p.Next()
	if ip.IsZero() {
		return netaddr.IPPrefix{}
	}

	return netaddr.IPPrefixFrom(ip, p.p.Bits())
}

// Release 释放之前占用的地址。
func (p *AddrPool) Release(ip netaddr.IP) {
	p.m.Lock()
	defer p.m.Unlock()
	delete(p.used, ip)
}

// NewAddrPool 初始化网络地址池。
// 支持 IPv4/IPv6
func NewAddrPool(prefix string) *AddrPool {
	p := netaddr.MustParseIPPrefix(prefix)
	r := p.Range()

	var n int
	if p.IP().Is4() {
		n = 2 << (32 - p.Bits())
	} else {
		n = 2 << (128 - p.Bits())
	}

	used := map[netaddr.IP]bool{
		r.From(): true,
		r.To():   true,
	}

	return &AddrPool{
		num:  n - 2,
		used: used,
		p:    p,
		r:    r,
		next: r.From().Next(),
	}
}
