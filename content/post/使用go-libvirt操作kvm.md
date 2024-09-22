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

启动

```bash
systemctl start libvirtd
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

查询网卡信息, 需要先安装`qemu-guest-agent`

```bash
apt-get install qemu-guest-agent
systemctl start qemu-guest-agent
```

```go
interfaces, err := client.DomainInterfaceAddresses(d, uint32(libvirt.DomainInterfaceAddressesSrcAgent), 0)
		if err != nil {
			fmt.Println(err)
		}
		for _, i := range interfaces {
			fmt.Println(i)
		}
```

这个库还提供了事件机制

```go
	events, err := l.SubscribeEvents(ctx, libvirt.DomainEventIDDeviceAdded, []libvirt.Domain{})
	if err != nil {
		log.Fatal(err)
	}
```

又想搞个kvm管理平台了...

### libvirt-go库

```go
package main

import (
	"fmt"
	"github.com/libvirt/libvirt-go"
)

func main() {
	conn, err := libvirt.NewConnect("qemu+tcp://192.168.50.20/system")
	if err != nil {
		fmt.Println("Failed to connect to qemu:///system:", err)
		return
	}
	defer conn.Close()
	domains, err := conn.ListAllDomains(libvirt.CONNECT_LIST_DOMAINS_ACTIVE | libvirt.CONNECT_LIST_DOMAINS_INACTIVE)
	if err != nil {
		fmt.Println("Failed to list all domains:", err)
		return
	}
	fmt.Println("Virtual Machines:")
	for _, domain := range domains {
		name, err := domain.GetName()
		if err != nil {
			fmt.Println("Failed to get domain name:", err)
			continue
		}
		// 获取虚拟机状态
		state, _, err := domain.GetState()
		if err != nil {
			fmt.Println("Failed to get domain state:", err)
			continue
		}
		// 获取虚拟机UUID
		uuid, err := domain.GetUUIDString()
		if err != nil {
			fmt.Println("Failed to get domain UUID:", err)
			continue
		}

		maxMem, err := domain.GetMaxMemory()
		if err != nil {
			fmt.Println("Failed to get domain max memory:", err)
			continue
		}
		fmt.Printf(" - Name: %s\n", name)
		fmt.Printf("   UUID: %s\n", uuid)
		fmt.Printf("   State: %d\n", state)
		fmt.Printf("   Max Memory: %d KB\n", maxMem)
		domain.Free() // 释放资源
	}
}

```

