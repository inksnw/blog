---
title: "Ebpf双向传输"
date: 2023-10-24T11:38:41+08:00
tags: ["ebpf"]
---

提供一个eBPF中内核态和用户态之间的双向数据传输的demo

### 环境准备

准备一个Ubuntu 22.04

```bash
sudo apt-get update
sudo apt-get install -y gcc-multilib llvm clang
go install github.com/cilium/ebpf/cmd/bpf2go@latest
```

接下来，我们需要一些eBPF相关的头文件。您可以从[cilium/ebpf](https://github.com/cilium/ebpf/tree/master/examples/headers)下载它们，并将它们保存到`headers`目录。

---

### 生成命令介绍

使用`bpf2go`工具，我们可以从C代码生成Go代码，这样我们就可以在Go程序中使用eBPF程序和映射。以下是生成命令：

```bash
//go:generate bpf2go -cc clang -target bpfel demo kern/tproxy.c -- -I kern/headers
```

参数说明：
- `-target bpfel`：生成小端（例如，x86 CPU）的代码。
- `demo`：生成的Go文件和结构的名称。
- `tproxy.c`：包含内核逻辑的C文件。

这将生成eBPF程序和map，其中map用于内核态和用户态之间的数据交换，而函数用于实现具体的网络逻辑。

---

### 内核态逻辑

#### 内核发送到用户

当数据包到达XDP挂载点时，eBPF程序开始解析数据包的头部信息，例如来源IP。解析完成后，它会将这些信息通过map发送到用户态。

#### 用户发送到内核

eBPF程序还会检查数据包的源IP是否在一个特定的map（“deny_ips_map”）中。如果存在，则数据包将被丢弃。

---

### 用户态逻辑

#### 读取内核信息

在用户态，我们可以使用以下代码从ring buffer中读取数据包信息：

```go
rd, err := ringbuf.NewReader(XdpObj.IpMap)
if err != nil {
    log.Fatal(err)
}
record, err := rd.Read()
if err != nil {
    log.Fatal(err)
}
fmt.Println(record)
```

#### 发送信息到内核

要将特定的IP添加到黑名单，您可以使用以下代码：

```go
ipUint32 := binary.BigEndian.Uint32(ip.To4())
err = control.XdpObj.DenyIpsMap.Put(ipUint32, uint8(1))
if err != nil {
    log.Fatal(err)
}
```

### 部分代码

#### 目录结构

```
.
├── CMakeLists.txt
├── Makefile
├── go.mod
├── go.sum
├── main.go
├── pkg
│   ├── api
│   │   └── rest.go
│   └── control
│       ├── control.go
│       ├── demo_bpfel.go
│       ├── demo_bpfel.o
│       └── kern
│           ├── headers
│           │   ├── common.h
│           │   ├── tcp.h
│           │   ...
│           └── tproxy.c
└── readme.md
```

#### tproxy.c

```c
//go:build ignore
#include <common.h>
#include <linux/tcp.h>
#include <bpf_endian.h>

struct ip_data {
    __u32 sip;     //来源IP
    __u32 dip;     //目标IP
    __u32 pkt_sz;  //包大小
    __u32 iii;     //  ingress_ifindex
    __be16 sport;  //来源端口
    __be16 dport;  //目的端口
};
struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 1 << 20);
} ip_map SEC(".maps");

// 黑名单
struct bpf_map_def SEC("maps") deny_ips_map = {
        .type = BPF_MAP_TYPE_HASH,
        .key_size = sizeof(__u32),
        .value_size = sizeof(__u8),
        .max_entries = 1024,
};

SEC("xdp")
int my_pass(struct xdp_md *ctx) {
    void *data = (void *) (long) ctx->data;
    void *data_end = (void *) (long) ctx->data_end;
    int pkt_sz = data_end - data;

    struct ethhdr *eth = data;  // 链路层
    if ((void *) eth + sizeof(*eth) > data_end) {  //如果包不完整、或者被篡改， 直接DROP
        bpf_printk("Invalid ethernet header\n");
        return XDP_DROP;
    }

    struct iphdr *ip = data + sizeof(*eth); // 得到了 ip层
    if ((void *) ip + sizeof(*ip) > data_end) {
        bpf_printk("Invalid IP header\n");
        return XDP_DROP;
    }
    if (ip->protocol != 6) { //如果不是TCP 就不处理了
        return XDP_PASS;
    }
    struct tcphdr *tcp = (void *) ip + sizeof(*ip);
    if ((void *) tcp + sizeof(*tcp) > data_end) {
        return XDP_DROP;
    }
    //下面构建数据发送的内容
    struct ip_data *ipdata = NULL;
    ipdata = bpf_ringbuf_reserve(&ip_map, sizeof(*ipdata), 0);
    if (!ipdata) {
        return XDP_PASS;
    }
    ipdata->sip = bpf_ntohl(ip->saddr);// 网络字节序 转换成 主机字节序  32位
    ipdata->dip = bpf_ntohl(ip->daddr);
    ipdata->pkt_sz = pkt_sz;
    ipdata->iii = ctx->ingress_ifindex;
    ipdata->sport = bpf_ntohs(tcp->source); //16位
    ipdata->dport = bpf_ntohs(tcp->dest);
    //  bpf_printk("Sport: %u, Dport: %u\n", ipdata->sport, ipdata->dport);
    bpf_ringbuf_submit(ipdata, 0);
    __u32 sip = bpf_ntohl(ip->saddr);

    __u8 *allow = bpf_map_lookup_elem(&deny_ips_map, &sip);
    if (allow && *allow == 1) {
        return XDP_DROP;
    }

    return XDP_PASS;
}

char __license[] SEC("license") = "GPL";
```

#### control.go

```go
package control

import (
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/perf"
	"github.com/cilium/ebpf/ringbuf"
	"log"
	"net"
	"unsafe"
)

//go:generate bpf2go -cc clang  -target bpfel demo kern/tproxy.c -- -I kern/headers

type IpData struct {
	SIP   uint32
	DIP   uint32
	PktSz uint32
	III   uint32
	Sport uint16
	Dport uint16
}

func resolveIP(input_ip uint32, isbig bool) net.IP {
	ipNetworkOrder := make([]byte, 4)
	if isbig {
		binary.BigEndian.PutUint32(ipNetworkOrder, input_ip)
	} else {
		binary.LittleEndian.PutUint32(ipNetworkOrder, input_ip)
	}
	return ipNetworkOrder
}

var XdpObj *demoObjects

func LoadXDP() {

	XdpObj = &demoObjects{}
	err := loadDemoObjects(XdpObj, nil)
	if err != nil {
		log.Fatalln("加载出错:", err)
	}
	defer XdpObj.Close()

	iface, err := net.InterfaceByName("enp1s0")
	if err != nil {
		log.Fatalln(err)
	}

	l, err := link.AttachXDP(link.XDPOptions{
		Program:   XdpObj.MyPass,
		Interface: iface.Index,
	})
	if err != nil {
		log.Fatalln(err)
	}
	defer l.Close()

	//创建reader 用来读取  内核Map
	rd, err := ringbuf.NewReader(XdpObj.IpMap)

	if err != nil {
		log.Fatalf("creating event reader: %s", err)
	}
	defer rd.Close()

	fmt.Println("开始监听xdp")
	for {
		record, err := rd.Read()
		if err != nil {
			if errors.Is(err, perf.ErrClosed) {
				log.Println("Received signal, exiting..")
				return
			}
			log.Printf("reading from reader: %s", err)
			continue
		}

		if len(record.RawSample) > 0 {
			data := (*IpData)(unsafe.Pointer(&record.RawSample[0]))

			// 转换为网络字节序
			ipAddr1 := resolveIP(data.SIP, true)
			ipAddr2 := resolveIP(data.DIP, true)

			if ipAddr1.To4().String() != "10.8.0.2" {
				fmt.Printf("来源IP:%s,目标IP:%s,包大小:%d,入口网卡index:%d,来源端口:%d,目标端口:%d\n",
					ipAddr1.To4().String(),
					ipAddr2.To4().String(),
					data.PktSz,
					data.III, data.Sport, data.Dport)
			}
		}
	}
}
```

