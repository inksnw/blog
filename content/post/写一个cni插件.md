---
title: "写一个cni插件"
date: 2022-09-19T09:53:04+08:00
tags: ["k8s"]
---

## 什么是CNI

CNI是Kubernetes提供的网络接口。要使用CNI，需要在kubelet上加参数–network-plugin=cni，需要需要也可以设置–cni-conf-dir和–cni-bin-dir参数。对于Kubernetes来说，network plugin就是一个二进制文件。



容器网络功能的实现最终是通过CNI插件来完成的。每个CNI插件本质上就是一个可执行文件，而CNI的执行流程就是从容器管理系统和配置文件获取配置信息，然后将这些信息以环境变量和标准输入的形式传输给插件，再运行插件完成具体的容器网络配置，最后将配置结果通过标志输出返回。

## 准备工作

先卸载已安装的cni插件,k8s是使用kubekey安装的,查看源码发现,生成的calico配置文件在 /etc/kubernetes/network-plugin.yaml,删除

```bash
kubectl delete -f  /etc/kubernetes/network-plugin.yaml
# 有一个pod无法删除,强制删除
kubectl delete pod calico-kube-controllers-f9f9bbcc9-pgjs9 -n kube-system --force --grace-period=0
```

删除默认的网络配置文件

```
rm -rf /etc/cni/net.d/*
```
cni插件目录简介
> bridge： 桥 
> ipvlan: 在容器中添加一个ipvlan接口。
> loopback：设置loopback接口的状态为up。
> macvlan：创建一个新的 MAC 地址，将所有到该地址的流量转发到容器。
> ptp: 创建一个 veth 对。
> vlan：分配一个vlan设备。
> host-device：将已经存在的设备移动到容器中。

cni规范的地址

 https://github.com/containernetworking/cni/blob/main/SPEC.md

## 测试

安装cnitool

```
go install github.com/containernetworking/cni/cnitool@latest
export PATH=$PATH:/root/go/bin/
```

添加网络

```
ip netns add testing
```

配置文件

```json
{
    "cniVersion": "0.4.0",
    "name": "mynet",
    "type": "mycni",
    "bridge": "mybr0",
    "ipam": {
        "type": "host-local",
        "subnet": "10.16.0.0/16",
        "dataDir": "/tmp/cni-host",
        "routes": [
            {
                "dst": "0.0.0.0/0"
            }
        ]
    }
}
```

### cniVersion

代表CNI规范所用的版本。截止本文撰写期间，CNI规范的最新版本是0.4.0。

### name

目标网络的名称。

### type

所用插件的类型。在我们的例子，用的是自己编写的名为`mycni`的插件。

### bridge和isGateway

这两个都是和bridge插件相关的特定参数：

- bridge：我们通过它告诉bridge插件，将要创建的bridge(网桥)名称。
- isGateway：为true就是告诉插件，作为网关，给我们的bridge指定一个IP地址。这样，连接到bridge的容器就可以拿它当网关来用了。

### ipMasq

为目标网络配上Outbound Masquerade(地址伪装)，即：由容器内部通过网关向外发送数据包时，对数据包的源IP地址进行修改。

当我们的容器以宿主机作为网关时，这个参数是必须要设置的。否则，从容器内部发出的数据包就没有办法通过网关路由到其他网段。因为容器内部的IP地址无法被目标网段识别，所以这些数据包最终会被丢弃掉。

### ipam

IPAM(IP Adderss Management)即IP地址管理，提供了一系列方法用于对IP和路由进行管理。实际上，它对应的是由CNI提供的一组标准IPAM插件，比如像host-local，dhcp，static等。

以host-local插件为例，只要我们为它提供配置信息，定义好期望的子网与网关信息，以及允许的IP地址范围（可选），插件就会帮我们自动在目标网段里分配好IP地址。为了保证把IP地址不冲突，它把IP地址的分配信息保存在了宿主机的本地文件系统里，这样可以确保在同一台宿主机上运行的所有容器，IP地址一定都是彼此唯一的。

另一个插件dhcp，则会在宿主机上启动一个DHCP daemon守护进程。跑在宿主机上的容器，可以通过它向网络上的DHCP服务器发送请求，以获得相应的IP地址。

下面我们来看一下具体的参数配置：

- type：指定所用IPAM插件的名称，在我们的例子里，用的是host-local。
- subnet：为目标网络分配网段，包括网络ID和子网掩码，以CIDR形式标记。在我们的例子里为`10.15.10.0/24`，也就是目标网段为`10.15.10.0`，子网掩码为`255.255.255.0`。
- routes：用于指定路由规则，插件会为我们在容器的路由表里生成相应的规则。其中，dst表示希望到达的目标网段，以CIDR形式标记。gw对应网关的IP地址，也就是要到达目标网段所要经过的“next hop(下一跳)”。如果省略gw的话，那么插件会自动帮我们选择默认网关。在我们的例子里，gw选择的是默认网关，而dst为`0.0.0.0/0`则代表“任何网络”，表示数据包将通过默认网关发往任何网络。实际上，这对应的是一条默认路由规则，即：当所有其他路由规则都不匹配时，将选择该路由。
- rangeStart：允许分配的IP地址范围的起始值
- rangeEnd：允许分配的IP地址范围的结束值
- gateway：为网关（也就是我们将要在宿主机上创建的bridge）指定的IP地址。如果省略的话，那么插件会自动从允许分配的IP地址范围内选择起始值作为网关的IP地址。

cni插件代码

http://inksnw.asuscomm.com:3000/inksnw/cni

目录结构

```bash
root@node1:~/test# tree
.
├── 10-mycni.conf
└── bin
    └── mycni
```

执行测试

```bash
NETCONFPATH=./  CNI_PATH=./bin cnitool add mynet /var/run/netns/testing
```

输出

```bash
执行了Add
{
    "cniVersion": "0.4.0",
    "dns": {}
}
```

cni插件的工作模式本质就是调用二进制的程序,约定了输入输出,默认配置目录在`/etc/cni/net.d ` 默认二进制在 `/opt/cni/bin`

有哪些可用ip及已分配了的ip存储示例中使用的是`host-local`插件https://www.cni.dev/plugins/current/ipam/host-local/
