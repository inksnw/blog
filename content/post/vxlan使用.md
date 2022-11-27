---
title: "Vxlan使用"
date: 2022-05-25T11:05:09+08:00
tags: ["k8s"]
---

# vxlan

VXLAN是Virtual eXtensible Local Area Network的缩写,是一个在传统Layer 3网络上架设出来的Layer 2 overlay网络

## 点对点模式

<img src="http://inksnw.asuscomm.com:3001/blog/vxlan使用_55abc651d2676d86057d5a780c071c3c.jpg" alt="vxlan" style="zoom:50%;" />

在逻辑上形成的VXLAN overlay网络环境如上图，虚线部分示意出来的Overlay Network和VXLAN Tunnel都是逻辑上的概念。容器不用感知底层物理网络，看起来对端是和自己在同一个二层环境里，就是像是在VTEP设备的上面直接构建了一条VXLAN Tunnel，把Overlay网络里的网络接口直接在二层打通。

在IP地址分配后，Linux系统的路由表就会创建一条路由，去往`10.0.0.0/24`网段的报文走网络接口`vxlan0`出去。vm1上去往10.0.0.0/24的报文，在`vxlan0`上会做VXLAN封装，内层地址是`10.16.0.3`，外层地址是`192.168.50.29`。VXLAN报文通过物理网络达到对端vm2上的VETP vxlan0做VXLAN协议的解封装，从而结束整个过程。

具体操作命令

```bash
# 在A机器上
ip link add vxlan0 type vxlan id 120 dstport 4789 remote 192.168.50.29 dev enp1s0
ip addr add 10.16.0.2/24 dev vxlan0
ip link set vxlan0 up 
route -n
```
上述命令创建了一个Linux上类型为`vxlan`的网络接口，名为`vxlan0`。

- id: VNI标识是120。
- remote: 作为一个VTEP设备来封装和解封VXLAN报文，需要知道将封装好的VXLAN报文发送到哪个对端VTEP。可以利用group指定组播组地址，或者利用remote指定对端单播地址。这里点对点的对端IP地址为192.168.50.29。
- dstport: 指定目的端口为4789。
- dev: 指定VTEP通过哪个物理device来通信，这里是使用enp1s0。

查看网卡信息

```bash
$ ip addr
4: vxlan0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1450 qdisc noqueue state UNKNOWN group default qlen 1000
    link/ether 92:bc:67:b8:1f:95 brd ff:ff:ff:ff:ff:ff
    inet 10.16.0.2/24 scope global vxlan0
       valid_lft forever preferred_lft forever
    inet6 fe80::90bc:67ff:feb8:1f95/64 scope link 
       valid_lft forever preferred_lft forever
$ route -n 
Kernel IP routing table
Destination     Gateway         Genmask         Flags Metric Ref    Use Iface
0.0.0.0         192.168.50.1    0.0.0.0         UG    100    0        0 enp1s0
10.16.0.0       0.0.0.0         255.255.255.0   U     0      0        0 vxlan0
```

看到路由表中多了一个通过`vxlan0` 的路由,按子网掩码最长到最短的顺序匹配,请求10.0.0.0/24网络的地址和`255.255.255.0`按位与,请求会转发到0.0.0.0上,再由`enp1s0`出去

在B机器上做相同的操作

```bash
#在B机器上
ip link add vxlan0 type vxlan id 120 dstport 4789 remote 192.168.50.233 dev enp1s0
ip addr add 10.16.0.3/24 dev vxlan0
ip link set vxlan0 up 
route -n
```

以上简单的命令就完成了所有配置,在vm1上ping overlay网络的对端IP地址10.16.0.3，可以ping通。

```bash
root@base:~# ping 10.16.0.3
PING 10.16.0.3 (10.16.0.3) 56(84) bytes of data.
64 bytes from 10.16.0.3: icmp_seq=1 ttl=64 time=0.602 ms
64 bytes from 10.16.0.3: icmp_seq=2 ttl=64 time=0.314 ms
```

在ping包的同时，用tcpdump抓vm1 eth0网卡的包。因为报文到达enp1s0前经过了网络接口vxlan0, 完成了VXLAN的封装，所以在抓包结果里应该能看到完整的VXLAN报文。

抓包时可以只抓和对端`192.168.50.29`通信的报文，如下：

```bash
tcpdump -i enp1s0 host 192.168.50.29 -s0 -v -w vxlan.pcap
```

导入`wireshark`查看

<img src="http://inksnw.asuscomm.com:3001/blog/vxlan使用_5886c9b8455b9de8ee387a511d7029e4.png" alt="Snipaste_2022-11-27_22-16-24" style="zoom:50%;" />

## 广播模式

过arp泛洪来学习mac地址,即在vxlan子网内广播arp请求,对应节点响应.group指定多播组的地址,group的值保持一致就可以,有一定范围要求

```bash
# 在A机器上
ip link delete vxlan0
ip link add vxlan0 type vxlan id 120 dstport 4789 group 229.1.1.1 dev enp1s0
ip addr add 10.16.0.2/24 dev vxlan0
ip link set vxlan0 up 
```

```bash
# 在A机器上
ip link delete vxlan0
ip link add vxlan0 type vxlan id 120 dstport 4789 group 229.1.1.1 dev enp1s0
ip addr add 10.16.0.3/24 dev vxlan0
ip link set vxlan0 up 
```

## 容器跨主机互通

清理掉两台主机在上文中的vxlan0

```
ip link delete vxlan0
```
安装了docker后，可以看到多了一个docker0的网络接口，默认在172.17.0.0/16网段。这个是连接本地多个容器的网桥。

在两台主机上创建一个自定义网络，指定网段10.17.0.0/24。

```bash
docker network create --driver=bridge --subnet=10.17.0.0/24 mylan
```

利用`docker network ls`查看，可以看到一个新的bridge网络被创建，名称为mylan。

```bash
$ docker network ls
NETWORK ID     NAME      DRIVER    SCOPE
6386f405d676   bridge    bridge    local
28ac27a849f8   host      host      local
23de81e9cde3   mylan     bridge    local
fbb68e5e1225   none      null      local
```

利用`ip addr`可以看到多了一个网络接口，名字不是dockerXX，而直接以br开头，是一个网桥。

用`brctl show`查看

```bash
$ brctl show
bridge name     				bridge id               STP enabled     interfaces
br-23de81e9cde3         8000.02421e76cdbe       no
docker0         				8000.0242574058ec       no
```

分别创建docker容器

```bash
# A机器
docker run -d --name ngx1 --network=mylan --ip 10.17.0.8 nginx
docker exec -it ngx1 sh -c "echo ngx1 >/usr/share/nginx/html/index.html"
# B机器
docker run -d --name ngx1 --network=mylan --ip 10.17.0.9 nginx
docker exec -it ngx1 sh -c "echo ngx2 >/usr/share/nginx/html/index.html"
```
此时查看`brctl show`可以看到有一个`veth818c3b2`设备已经连接到了网桥上

```bash
$ root@base:~# brctl show
bridge name     				bridge id               STP enabled     interfaces
br-23de81e9cde3         8000.02421e76cdbe       no              veth818c3b2
docker0         				8000.0242574058ec       no
```

### 创建VXLAN接口接入docker网桥

```bash
# 两台主机,创建vxlan并搭在网桥上
ip link add vxlan0 type vxlan id 120 dstport 4789 group 229.1.1.1 dev enp1s0
brctl addif br-e4c356e71cd0 vxlan0
ip link set vxlan0 up 
```
查看网桥信息,可以看到`br-23de81e9cde3`网桥上接了vxlan 端和docker容器网卡端
```bash
$ brctl show
bridge name     				bridge id               STP enabled     interfaces
br-23de81e9cde3         8000.02421e76cdbe       no              veth818c3b2
                                                        				vxlan0
docker0         				8000.0242574058ec       no
```

此时的网络拓扑

<img src="http://inksnw.asuscomm.com:3001/blog/vxlan使用_e3661f8f12dff2de75b4b064fa116bce.jpg" alt="vxlan docker" style="zoom:50%;" />

有了VXLAN接口的连接后，从vm1上docker容器发出的包到达docker网桥后，可以从网桥的VXLAN接口出去，从而报文在VETP(VXLAN接口)处被封装成VXLAN报文，再从物理网络上到达对端VETP所在的主机vm2。对端VTEP能正确解包VXLAN报文的话，随后即可将报文通过vm2上的docker网桥送到上层的docker容器中。


```bash
# 从A主机容器访问B主机容器
$ docker exec -it ngx1 curl 10.17.0.9
ngx2
```

