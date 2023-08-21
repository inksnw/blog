---
title: "CGroups限制"
date: 2023-08-21T09:48:35+08:00
tags: ["k8s"]
---

在Linux系统中，tc（Traffic Control）是一个用于控制网络流量的工具，它可以用来限制传输速率、模拟网络延迟、丢包等场景。主要的概念包括qdisc（队列调度器）、class（分类器）、和filter（过滤器）。qdisc又分为classless qdisc（无分类队列调度器）和classful qdisc（有分类队列调度器），而控制传输速度的方法主要有两种用法，区别主要在于**粒度**与**复杂性**具体如下：

1. **对于classless qdisc**：你可以直接将一个classless qdisc 添加到网络接口上，它会作为一个简单的限流工具，用于对整个接口上的流量进行限制。这意味着所有的流量都受到相同的限制，没有进一步的分类。
2. **对于classful qdisc**：在这种情况下，你可以使用class和filter来创建更复杂的流量控制策略。首先，你会创建一个classful qdisc，然后在这个qdisc下创建多个class（子类），然后使用filter将流量分配到这些class中。每个class可以有自己的限制规则，例如不同的带宽限制、延迟、或丢包率。在最后的subclass下，你可以挂载一个classless qdisc，用于实际的流量输出。

Kubernetes（k8s）的CNI（Container Network Interface）插件限速，通常会使用classful qdisc 的方法。

### 创建网络配置

在一个名为 `ns0` 的网络命名空间中创建一个虚拟网络设备 `veth0-ns`，分配 IP 地址，并设置默认路由，然后通过与默认命名空间中的虚拟网络设备 `veth0-br` 相连，实现跨命名空间的通信

```bash
# 创建一个网络空间
ip netns add ns0
# 创建一对虚拟以太网设备，其中一个设备 veth0-ns 将位于 ns0 命名空间中，另一个设备 veth0-br 位于默认的命名空间中
ip link add veth0-ns type veth peer name veth0-br
# 将之前创建的虚拟以太网设备 veth0-ns 移动到 ns0 命名空间中
ip link set veth0-ns netns ns0
# 在 ns0 命名空间中，启用回环接口
ip netns exec ns0 ip link set lo up
# 在 ns0 命名空间中, 启用之前移动到命名空间内的 veth0-ns 接口
ip netns exec ns0 ip link set veth0-ns up
# 在 ns0 命名空间中，为 veth0-ns 接口分配 IP 地址
ip netns exec ns0 ip addr add 10.233.1.1/32 dev veth0-ns
# 在 ns0 命名空间中，添加默认的路由，将所有流量发送到 veth0-ns 接口
ip netns exec ns0 ip route add default dev veth0-ns
# 默认命名空间中，启用之前创建的 veth0-br 接口
ip link set veth0-br up
# 在默认命名空间中，添加一条静态路由
route add 10.233.1.1 dev veth0-br
```

### 删除配置

你可以按照以下步骤来撤销之前设置的网络配置：

```bash
# 删除静态路由
ip route del 10.233.1.1
# 删除默认命名空间中的 veth0-br 设备
ip link del veth0-br
# 删除网络命名空间 ns0
ip netns del ns0
```

### 使用golang完成这个过程

```go
package main

import (
	"fmt"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"net"
	"runtime"
)

func main() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	// Save the current network namespace
	origns, err := netns.Get()
	if err != nil {
		panic(err)
	}
	defer origns.Close()
	// Create a new network namespace
	newns, err := netns.NewNamed("ns0")
	if err != nil {
		panic(err)
	}
	defer newns.Close()
	fmt.Println(newns.String())
	// Switch back to the original namespace
	err = netns.Set(origns)
	if err != nil {
		panic(err)
	}
	//	创建一对虚拟以太网设备
	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name: "veth0-ns",
		},
		PeerName: "veth0-br",
	}
	if err := netlink.LinkAdd(veth); err != nil {
		panic(err)
	}

	// 将 veth0-ns 移动到 ns0 命名空间
	if err := netlink.LinkSetNsFd(veth, int(newns)); err != nil {
		panic(err)
	}

	err = netns.Set(newns)
	if err != nil {
		panic(err)
	}
	
	vethNS, err := netlink.LinkByName("veth0-ns")
	if err != nil {
		panic(err)
	}
	// 在 ns0 命名空间中启用设备
	if err := netlink.LinkSetUp(vethNS); err != nil {
		panic(err)
	}
  
// 在 ns0 命名空间中设置 IP 地址和路由
	addr := &netlink.Addr{IPNet: &net.IPNet{IP: net.ParseIP("10.233.1.1"), Mask: net.CIDRMask(32, 32)}}
	if err := netlink.AddrAdd(vethNS, addr); err != nil {
		panic(err)
	}
	route := &netlink.Route{
		LinkIndex: vethNS.Attrs().Index,
		Dst:       nil,
		Gw:        net.ParseIP("10.233.1.1"),
	}
	if err := netlink.RouteAdd(route); err != nil {
		panic(err)
	}

	netns.Set(origns)
	// 在默认命名空间中启用 veth0-br
	vethBR, err := netlink.LinkByName("veth0-br")
	if err != nil {
		return
	}
	err = netlink.LinkSetUp(vethBR)
	if err != nil {
		panic(err)
	}
	// 在默认命名空间中添加静态路由
	defaultRoute := &netlink.Route{
		LinkIndex: vethBR.Attrs().Index,
		Dst: &net.IPNet{
			IP:   net.IPv4(10, 233, 1, 1),
			Mask: net.IPv4Mask(255, 255, 255, 255),
		},
		Gw: net.ParseIP("0.0.0.0"),
	}
	if err := netlink.RouteAdd(defaultRoute); err != nil {
		panic(err)
	}
	fmt.Println("Setup complete!")
}
```

### 限速

对于 ingress（入站）流量的限速，不需要创建额外的设备。然而，对于 egress（出站）流量的限速，通常情况下需要使用 ifb 设备将出站流量重新定向到一个虚拟的网络接口，然后在该接口上进行限速。

**限制 Ingress（入站）流量：**

```bash
tc qdisc add dev veth0-br handle 1:0 root tbf rate 1Mbit burst 10k latency 70ms
```

**限制 Egress（出站）流量：**

```bash
# 创建 egress 限制
ip link add ifb0 type ifb
ip link set dev ifb0 up
tc qdisc add dev veth0-br handle ffff: ingress
tc qdisc add dev ifb0 handle 1:0 root tbf rate 1Mbit burst 10k latency 70ms
tc filter add dev veth0-br parent ffff: protocol all pref 1 u32 match u32 0 0 flowid 1:1 action mirred egress redirect dev ifb0 pass
```

查看限速信息

```bash
root@base:~# tc -s qdisc show
qdisc tbf 1: dev veth0-br root refcnt 5 rate 1Mbit burst 10Kb lat 70ms 
 Sent 280 bytes 4 pkt (dropped 0, overlimits 0 requeues 0) 
 backlog 0b 0p requeues 0
qdisc ingress ffff: dev veth0-br parent ffff:fff1 ---------------- 
 Sent 56 bytes 1 pkt (dropped 0, overlimits 0 requeues 0) 
 backlog 0b 0p requeues 0
qdisc tbf 1: dev ifb0 root refcnt 2 rate 1Mbit burst 10Kb lat 70ms 
 Sent 210 bytes 3 pkt (dropped 0, overlimits 0 requeues 0) 
 backlog 0b 0p requeues 0
```

### golang实现

可以参考https://github.com/containernetworking/plugins 的`CreateEgressQdisc` 与`CreateIngressQdisc`函数

> https://github.com/containernetworking/plugins/blob/9d9ec6e3e18ea245b9cef0f8396e570247338d1f/plugins/meta/bandwidth/ifb_creator.go#L52
>
> https://github.com/containernetworking/plugins/blob/9d9ec6e3e18ea245b9cef0f8396e570247338d1f/plugins/meta/bandwidth/ifb_creator.go#L60

### 取消限速

```bash
# 停用 ifb 设备
ip link set dev ifb0 down
# 删除 ingress 队列调度规则
tc qdisc del dev veth0-br ingress
# 删除 ifb0 的根队列调度规则
tc qdisc del dev ifb0 root
# 删除 ingress 过滤器规则
tc filter del dev veth0-br parent ffff:
```

### 测速

#### egress

首先进行egress限制流量测试，打开一个会话，使用iperf创建一个server

```
ip netns exec ns0 iperf3 -s
```

再打开一个会话，创建一个client测试，结果如下：

```bash
iperf3 -c 10.233.1.1 -t 20
- - - - - - - - - - - - - - - - - - - - - - - - -
[ ID] Interval           Transfer     Bitrate         Retr
[  5]   0.00-20.00  sec  2.38 MBytes  1000 Kbits/sec    0             sender
[  5]   0.00-20.07  sec  2.29 MBytes   958 Kbits/sec                  receiver

iperf Done.
```

#### ingress

然后进行ingress限制流量测试，打开一个会话，使用iperf创建一个server

```
 iperf3 -s
```

再打开一个会话，创建一个client测试，`192.168.50.181` 是主机ip, 结果如下：

```bash
- - - - - - - - - - - - - - - - - - - - - - - - -
[ ID] Interval           Transfer     Bitrate         Retr
[  5]   0.00-20.00  sec  2.51 MBytes  1.05 Mbits/sec    5             sender
[  5]   0.00-20.09  sec  2.30 MBytes   958 Kbits/sec                  receiver

iperf Done.
```

可以看到流量进出网络命名空间 ns0 的时候流量大致被限制在了1Mbits/s。
