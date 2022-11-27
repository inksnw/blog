---
title: "Vxlan使用"
date: 2022-05-25T11:05:09+08:00
tags: ["k8s"]
---

# vxlan

VXLAN是Virtual eXtensible Local Area Network的缩写,是一个在传统Layer 3网络上架设出来的Layer 2 overlay网络

```bash
docker run --name ngx1 -p 8080:80 --privileged -d nginx
docker exec -it ngx1 sh -c "echo ngx1 >/usr/share/nginx/html/index.html"
docker run --name ngx1 -p 8080:80 --privileged -d nginx
docker exec -it ngx1 sh -c "echo ngx2 >/usr/share/nginx/html/index.html"
```

## 点对点模式

![vxlan.drawio](http://inksnw.asuscomm.com:3001/blog/vxlan使用_6017f9a085ac01c69fa6ff45825a4bee.svg)

```bash
# 在A机器上
ip link add vxlan0 type vxlan id 120 dstport 4789 remote 192.168.50.41 local 192.168.50.40 dev enp1s0
ip addr add 10.16.0.2/24 dev vxlan0
ip link set vxlan0 up 
route -n
```

```bash
#在B机器上
ip link add vxlan0 type vxlan id 120 dstport 4789 remote 192.168.50.40 local 192.168.50.41 dev enp1s0
ip addr add 10.16.0.3/24 dev vxlan0
ip link set vxlan0 up 
route -n
```

广播模式

```bash
# 在A机器上
ip link delete vxlan0
# 通过arp泛洪来学习mac地址,即在vxlan子网内广播arp请求,对应节点响应.group指定多播组的地址
ip link add vxlan0 type vxlan id 120 dstport 4789 group 229.1.1.1 dev enp1s0
ip addr add 10.16.0.2/24 dev vxlan0
ip link set vxlan0 up 
```

```bash
# 在A机器上
ip link delete vxlan0
# 通过arp泛洪来学习mac地址,即在vxlan子网内广播arp请求,对应节点响应.group指定多播组的地址
ip link add vxlan0 type vxlan id 120 dstport 4789 group 229.1.1.1 dev enp1s0
ip addr add 10.16.0.3/24 dev vxlan0
ip link set vxlan0 up 
```

# 容器跨主机互通

```bash
#新建网桥,两台主机
docker network create --driver=bridge --subnet=10.16.0.0/24 mylan
# A机器
docker run --name ngx1 --network=mylan --ip 10.6.0.8 nginx
docker exec -it ngx1 sh -c "echo ngx1 >/usr/share/nginx/html/index.html"
# B机器
docker run --name ngx1 --network=mylan --ip 10.6.0.9 nginx
docker exec -it ngx1 sh -c "echo ngx2 >/usr/share/nginx/html/index.html"
```

```bash
# 两台主机,创建vxlan并搭在网桥上
ip link add vxlan0 type vxlan id 120 dstport 4789 group 229.1.1.1 dev enp1s0
brctl addif br-xxxx vxlan0
ip link set vxlan0 up 
```

```bash
# 从A主机容器访问B主机容器
docker exec -it ngx1 curl 10.16.0.9
```

