---
title: "写一个cni插件"
date: 2022-08-29T13:53:04+08:00
tags: ["k8s"]
---

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

> cni的二进制在 /opt/cni/bin
>
> bridge： 桥 
>
> ipvlan: 在容器中添加一个ipvlan接口。
>
> loopback：设置loopback接口的状态为up。
>
> macvlan：创建一个新的 MAC 地址，将所有到该地址的流量转发到容器。
>
> ptp: 创建一个 veth 对。
>
> vlan：分配一个vlan设备。
>
> host-device：将已经存在的设备移动到容器中。

cni规范的地址

 https://github.com/containernetworking/cni/blob/main/SPEC.md

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
  "type": "mycni"
}
```

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
