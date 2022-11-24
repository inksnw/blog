---
title: "Docker+flannel跨主机通信"
date: 2022-11-24T19:43:57+08:00
draft: true
---

## 安装ETCD

flannel需要把网络信息存储在`etcd`中,我们先安装etcd

```bash
apt install etcd
```
修改etcd配置文件,开启对外访问

```bash
# vi /etc/default/etcd
ETCD_NAME="controller"
ETCD_DATA_DIR="/var/lib/etcd"
ETCD_INITIAL_CLUSTER_STATE="new"
ETCD_INITIAL_CLUSTER_TOKEN="etcd-cluster-01"
ETCD_INITIAL_CLUSTER="controller=http://0.0.0.0:2380"
ETCD_INITIAL_ADVERTISE_PEER_URLS="http://0.0.0.0:2380"
ETCD_ADVERTISE_CLIENT_URLS="http://0.0.0.0:2379"
ETCD_LISTEN_PEER_URLS="http://0.0.0.0:2380"
ETCD_LISTEN_CLIENT_URLS="http://0.0.0.0:2379"
```

```bash
systemctl start etcd
systemctl status etcd
```

向etcd中写入flannel配置信息

```bash
export ETCDCTL_API=3
etcdctl  put /flannel/network/config '{"Network":"10.3.0.0/16","SubnetLen":24,"SubnetMin":"10.3.20.0","SubnetMax":"10.3.100.0","Backend":{"Type":"vxlan"}}'
```

## 安装flannel

下载`flannel-v0.20.1-linux-amd64.tar.gz` 解压到 `/usr/local/bin/`下,配置etcd的访问地址

```bash
#  vi /lib/systemd/system/flanneld.service
[Unit]
Description=Flanneld
[Service]
User=root
ExecStart=/usr/local/bin/flanneld \
-etcd-endpoints=http://127.0.0.1:2379 \
-etcd-prefix=/flannel/network

ExecStartPost=/usr/local/bin/mk-docker-opts.sh -k DOCKER_NETWORK_OPTIONS -d /run/flannel/docker         
Restart=on-failure
[Install]
WantedBy=multi-user.target
```
启动flanneld
```bash
systemctl daemon-reload
systemctl start flanneld.service
systemctl status flanneld.service
```
查看`/run/flannel/docker`文件,如果文件不存在可尝试手动创建,文件中没有内容,可以尝试重启flannel

```bash
$ cat /run/flannel/docker
DOCKER_OPT_BIP="--bip=10.3.48.1/24"
DOCKER_OPT_IPMASQ="--ip-masq=true"
DOCKER_OPT_MTU="--mtu=1450"
DOCKER_NETWORK_OPTIONS=" --bip=10.3.48.1/24 --ip-masq=true --mtu=1450"
```

此时 `etcd` 中也会出现这台主机的信息, 这个图形管理工具是 `etcdkeeper`

<img src="http://inksnw.asuscomm.com:3001/blog/docker+flannel跨主机通信_6103c187ed1c704b3452b66f9bcef685.png" alt="image-20221124195408776" style="zoom:50%;" />



## 修改docker配置

```bash
$ vi /lib/systemd/system/docker.service

#在[Service]下加一行
EnvironmentFile=/run/flannel/docker
# 修改启动命令为
ExecStart=/usr/bin/dockerd $DOCKER_NETWORK_OPTIONS  -H fd:// --containerd=/run/containerd/containerd.sock
systemctl daemon-reload
systemctl restart docker
```

创建另一台主机或者克隆当前主机做同样的操作,跳过etcd的安装步骤,使用第一台的就可以

## 访问测试

在A主机上创建一个容器,查看它的ip地址为 `10.3.48.2`


```bash
# 在A主机上创建
$ docker run -it --name alpine1 -d alpine sh
$ docker exec -it 03 /bin/sh
$ ip addr
1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue state UNKNOWN qlen 1000
    link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
    inet 127.0.0.1/8 scope host lo
       valid_lft forever preferred_lft forever
11: eth0@if12: <BROADCAST,MULTICAST,UP,LOWER_UP,M-DOWN> mtu 1450 qdisc noqueue state UP 
    link/ether 02:42:0a:03:30:02 brd ff:ff:ff:ff:ff:ff
    inet 10.3.48.2/24 brd 10.3.48.255 scope global eth0
       valid_lft forever preferred_lft forever
```

在B主机执行同样的操作(略),查看到它的ip为 `10.3.42.2`

在A主机的容器中`ping` B主机的容器ip

```bash
$ ping 10.3.42.2
PING 10.3.42.2 (10.3.42.2): 56 data bytes
64 bytes from 10.3.42.2: seq=0 ttl=62 time=0.431 ms
64 bytes from 10.3.42.2: seq=1 ttl=62 time=0.292 ms
```

可以看到,已可以实现跨主机容器互通
