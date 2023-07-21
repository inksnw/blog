---
title: "使用go-libvirt操作kvm"
date: 2023-07-21T19:03:29+08:00
tags: ["虚拟化"]
---

### 配置libvird远程连接

修改`/etc/libvirt/libvirtd.conf`中

```bash
listen_tls = 0
listen_tcp = 1
tcp_port = "16509"
listen_addr = "0.0.0.0"
auth_tcp = "none"
```

修改`/etc/default/libvirtd`

```bash
LIBVIRTD_ARGS="-l -f /etc/libvirt/libvirtd.conf"
```

修改`/lib/systemd/system/libvirtd.service`

网上的文章大多没提这点, [参考这里](https://bugzilla.redhat.com/show_bug.cgi?id=1741403), 大意是手动监听socket就不能用systemd的了

```bash
# 注释掉这几个服务
[Unit]
Description=Virtualization daemon
#Requires=virtlogd.socket
#Requires=virtlockd.socket
#Wants=libvirtd.socket
#Wants=libvirtd-ro.socket
#Wants=libvirtd-admin.socket
[Install]
WantedBy=multi-user.target
#Also=virtlockd.socket
#Also=virtlogd.socket
#Also=libvirtd.socket
#Also=libvirtd-ro.socket
```

停止自启动

```bash
systemctl stop virtlogd.socket
systemctl stop virtlockd.socket
systemctl stop libvirtd.socket
systemctl stop libvirtd-ro.socket
systemctl stop libvirtd-admin.socket 
systemctl disable virtlogd.socket
systemctl disable virtlockd.socket
systemctl disable libvirtd.socket
systemctl disable libvirtd-ro.socket
systemctl disable libvirtd-admin.socket 
```

### 代码访问

```go
package main

import (
	"fmt"
	"log"
	"net"
	"time"

	"github.com/digitalocean/go-libvirt"
)

func main() {
	// This dials libvirt on the local machine, but you can substitute the first
	// two parameters with "tcp", "<ip address>:<port>" to connect to libvirt on
	// a remote machine.
	c, err := net.DialTimeout("tcp", "192.168.50.20:16509", 2*time.Second)
	if err != nil {
		log.Fatalf("failed to dial libvirt: %v", err)
	}

	l := libvirt.New(c)
	if err := l.Connect(); err != nil {
		log.Fatalf("failed to connect: %v", err)
	}

	v, err := l.Version()
	if err != nil {
		log.Fatalf("failed to retrieve libvirt version: %v", err)
	}
	fmt.Println("Version:", v)

	domains, err := l.Domains()
	if err != nil {
		log.Fatalf("failed to retrieve domains: %v", err)
	}

	fmt.Println("ID\tName\t\tUUID")
	fmt.Printf("--------------------------------------------------------\n")
	for _, d := range domains {
		fmt.Printf("%d\t%s\t%x\n", d.ID, d.Name, d.UUID)
	}

	if err := l.Disconnect(); err != nil {
		log.Fatalf("failed to disconnect: %v", err)
	}
}
```

结果

```bash
Version: 9.0.0
ID      Name            UUID
--------------------------------------------------------
3       node3   d5674adaa9104778a8785f1c6411b8f7
4       node1   40adc7c6973a494380f87ad784a72dcb
2       node2   d647d11ba9da4bdba43aeddbab732d5d
```

这个库还提供了事件机制

```go
	events, err := l.SubscribeEvents(ctx, libvirt.DomainEventIDDeviceAdded, []libvirt.Domain{})
	if err != nil {
		log.Fatal(err)
	}
```

又想搞个kvm管理平台了...
