---
title: "Iptables与ipvs"
date: 2022-05-25T11:07:01+08:00
tags: ["k8s"]
---

# iptables

## filter表

```bash
iptables -F #清空（flush）
iptables -X #删除指定表中用户自定义的规则链（delete-chain）
iptables -nvL -t filter --line-number
```

```bash
#追加
iptables -A INPUT -p tcp --dport 8000 -j DROP 
#删除
iptables -D INPUT -p tcp --dport 8000 -j DROP 
#按行号删除
iptables -D INPUT 1
```

```bash
#开启关闭日志
iptables -A INPUT -j LOG
iptables -D INPUT -j LOG
```

## nat表

```bash
iptables -nvL -t nat --line-number
iptables -t nat -A PREROUTING -p tcp -d 192.168.50.18 --dport 80 -j DNAT --to-destination 192.168.50.18:8000
iptables -t nat -D PREROUTING 1 #按num删除
```

### 负载均衡

```bash
iptables -A PREROUTING -t nat -p tcp -d 192.168.50.18 --dport 80 -m statistic --mode random --probability 0.5 -j DNAT --to-destination 192.168.50.18:8000
```

probability公式
probability=1/(n-i+1)
n:后端的数量
i:序号(从1开始)

### docker端口映射

```bash
iptables -t nat -A PREROUTING -p tcp -d 192.168.50.18 --dport 80 -j DNAT --to-destination 172.17.0.2:80
iptables -A FORWARD -i enp1s0 -o docker0 -j ACCEPT
```

### 限流

```bash
iptables -A INPUT -p tcp --dport 9090 -m connlimit --connlimit-above 2 -j REJECT --reject-with tcp-reset
```

## 自定义链

```bash
iptables -t filter -N test
iptables -A INPUT -p tcp --dport 8000 -j test 
iptables -A test -p tcp --dport 8000 -j DROP
#删除引用计数为0并且不包含任何规则的自定义链
iptables -X test 
```

# ipvs

在 Kubernetes 集群中，每个 Node 运行一个 `kube-proxy` 进程。`kube-proxy` 负责为 Service 实现了一种 VIP（虚拟 IP）的形式。

ipvs是工作在内核态的4层负载均衡，基于内核底层netfilter实现，netfilter主要通过各个链的钩子实现包处理和转发。ipvs由ipvsadm提供简单的CLI接口进行ipvs配置。由于ipvs工作在内核态，只处理四层协议，因此只能基于路由或者[NAT](https://zh.wikipedia.org/zh-cn/网络地址转换)进行数据转发，可以把ipvs当作一个特殊的路由器网关，这个网关可以根据一定的算法自动选择下一跳。

### IPVS vs IPTABLES

- iptables 使用链表，ipvs 使用哈希表；
- iptables 只支持随机、轮询两种负载均衡算法而 ipvs 支持的多达 8 种；
- ipvs 还支持 realserver 运行状况检查、连接重试、端口映射、会话保持等功能。

### IPVS用法

IPVS可以通过ipvsadm 命令进行配置，如-L列举，-A添加，-D删除。

如下命令创建一个service实例`127.0.0.1:32016`，`-t`指定监听的为`TCP`端口，`-s`指定算法为轮询算法rr(Round Robin)，ipvs支持简单轮询(rr)、加权轮询(wrr)、最少连接(lc)、源地址或者目标地址散列(sh、dh)等10种调度算法。

```bash
ipvsadm -A -t 127.0.0.1:32016 -s rr
```

在添加调度算法的时候还需要用-r指定server地址，-w指定权值，-m指定转发模式，-m设置masquerading表示NAT模式（-g为`gatewaying`，即直连路由模式），如下所示：

```bash
ipvsadm -a -t 127.0.0.1:32016 -r 127.0.0.1:8000 -m -w 1
ipvsadm -a -t 127.0.0.1:32016 -r 127.0.0.1:8001 -m -w 1
```

这个配置的意思是访问本机的`127.0.0.1:32016` 都会轮询的转发到 `127.0.0.1:8000` 和`127.0.0.1:8001` 上

开两个终端

```bash
python3 -m http.server 8000
python3 -m http.server 8001
```

访问

```bash
curl 127.0.0.1:32016
```

本例中ip都使用了`127.0.0.1` 实际中,可以转发不同网段的网络, 如 k8s 的pod网络和svc网络

删除所有规则

```
ipvsadm --clear
```

快速创建一个应用实例

```bash
kubectl create deployment demo --image=nginx --port=80
kubectl expose deployment demo
```

创建好之后，看看svc

```bash
➜ kubectl get svc
NAME         TYPE        CLUSTER-IP      EXTERNAL-IP   PORT(S)   AGE
demo         ClusterIP   10.233.63.220   <none>        80/TCP    15h
```

查看ipvs配置情况：

```bash
# -S表示输出所保存的规则，-n表示以数字的形式输出ip和端口,不转义。可以看到ipvs的LB IP为ClusterIP，算法为rr。模式为NAT模式。
➜ ipvsadm -S -n | grep 10.233.63.220
-A -t 10.233.63.220:80 -s rr
-a -t 10.233.63.220:80 -r 10.233.92.3:80 -m -w 1
```

当我们创建Service之后，kube-proxy 首先会在宿主机上创建一个虚拟网卡（叫作：kube-ipvs0），并为它 **增加** 一个 Service VIP 作为 IP 地址，如下所示：`10.233.63.220` 是我们demo服务的ip, `10.233.0.1` 和 `10.233.0.3` 分别是 kubernetes 和 coredns 的ip

```bash
➜ ip addr show kube-ipvs0
4: kube-ipvs0: <BROADCAST,NOARP> mtu 1500 qdisc noop state DOWN group default 
    link/ether d6:03:0d:92:93:c5 brd ff:ff:ff:ff:ff:ff
    inet 10.233.63.220/32 scope global kube-ipvs0
       valid_lft forever preferred_lft forever
    inet 10.233.0.1/32 scope global kube-ipvs0
       valid_lft forever preferred_lft forever
    inet 10.233.0.3/32 scope global kube-ipvs0
       valid_lft forever preferred_lft forever
```
