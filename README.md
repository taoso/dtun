# dtun

非常简单的 IP 隧道，基于 DTLS。

## 使用

```bash
go get github.com/lvht/dtun/cmd/dtun

# 服务端
go run main.go -key foo

# 客户端
# key 参数需要跟服务端保持一致
# id  参数不能跟其他隧道冲突，不然其他隧道会断线
go run main.go -host ${server} -key foo -id demo
```

## 设计

简单列表下几个考虑的因素。

<details>
<summary>UDP vs TCP</summary>

实现稳定靠的传输层数据加密并不容易，所以我一直用 TLS 作为底层协议。

TLS 使用 TCP，被加密的数据很大程度上也是 TCP 数据。这样传输一个上层的数据包就
需要内外两层 TCP 连接确认。这种 TCP over TCP 的实现问题还不小，具体参见：
参考 http://sites.inka.de/~bigred/devel/tcp-tcp.html

所以说，最好还是用 UDP 传输。加密自然要用 DTLS 了。目前 DTLS 还不支持 1.3，而且
go 语言官方还不支持，只能使用这个三方实现 https://github.com/pion/dtls
</details>

<details>
<summary>加密 vs 鉴权</summary>

不论是 TLS 还是 DTLS，一般都需要创建证书。这个过程现在可以不用花钱了，但配置起
来还是很复杂。另外，证书只解决了加密问题，并没有解决鉴权问题。通常只有客户端校
验服务端证书。也可以让服务端校验客户端证书，但这样太麻烦了。

除此之外，DTLS 是无连接的，不能像 TCP 连接那样只在创建连接的时候做鉴权就可以了。

所以，最好能在 DTLS 握手的时候同时完成两端的鉴权。所以我选用 Pre-Shared Key(PSK)
模式。我们只需在两端使用`-key`指定主密钥，就可以完成双端鉴权。如果客户端不知道
PSK就无法建立DTLS会话。

另外，PSK还需要指定一个 hint 参数。大家可以简单看作是PSK的名字。DTLS没有连接，
不好判断客户端是不是已经下线。我决定让每个客户端都使用唯一的 hint 参数。服务端
针对 hint 分配 tun 设备。如果客户端断线重连，也不会创建多个 tun 设备。但副作用
就是同一个 hinit 的客户端不能同时登录。
</details>

<details>
<summary>客户端路由</summary>

为了支持 macos，tun 设备只能设置成点对点模式。如果我们想做透明路由转发，看下图
```
pc <-----------> router <====== dtun ======> pc2 <---------> www
10.0.0.2/16    10.0.0.1/16   10.1.0.1/16      10.1.0.2/16
```

我们希望 pc 发出的包经路由器 router 转发给 pc2 再转发到外部网络。一般我们会在
router 上做一次 nat，再在 pc2 上做一次 nat。这样做的好处是 pc2 不需要感知 pc 到
router 网络配置。但坏处也很明显，有两次 nat。路由器的性能一般也不强，nat 还是要
尽量避免的。

所以我的方案是直接将 pc 所在的网段 10.0.0.0/16 推给 pc2，并在 pc2 上添加路由
```
ip route add 10.0.0.0/16 via 10.1.0.1
```

这样 router 可以把来自 pc 的包原样转发给 pc2，只需在 pc2 上做一次 nat 就可以了。
</details>

<details>
<summary>路由分流</summary>

有时候我们需要指定路由白名单。在白名单里的网段走默认路由，其他的通过隧道转发。

我们可以在 router 先添加白名单路由，下一跳设成 router 默认路由。
然后指定 pc2 的公网IP走 router 的默认路由（关键！）。
最后添加
```
ip route add 0.0.0.0/1 via 10.1.0.2
ip route add 128.0.0.0/1 via 10.1.0.2
```
这里的 0.0.0.0/1 和 128.0.0.1/1 正好覆盖整个网段，效果赞同于 default，但又不会
覆盖默认路由。如果隧道异常关闭，所有相关路由会自动删除，非常稳定。

你可以写成一个脚本，使用`-up`参数指定运行。我的脚本如下：
```bash
#!/bin/sh

#curl -S https://cdn.jsdelivr.net/gh/misakaio/chnroutes2@master/chnroutes.txt|grep -v '#'|xargs -I % ip route add % via $DEFAULT_GW 2>/dev/null
VPN_IP=$(ping your-server-name -c 1|grep from|cut -d' ' -f4|cut -d: -f1)
DEFAULT_GW=$(ip route|grep default|cut -d' ' -f3)
ip route add $VPN_IP/32 via $DEFAULT_GW
ip route add 0.0.0.0/1 via $PEER_IP
ip route add 128.0.0.0/1 via $PEER_IP
```
</details>
