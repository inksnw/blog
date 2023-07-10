---
title: "Gobgp使用"
date: 2023-07-10T21:00:55+08:00
tags: ["网络"]
---

### 安装环境

```
curl -fsSL https://get.docker.com -o get-docker.sh
sh get-docker.sh
```

```bash
wget https://github.com/osrg/gobgp/releases/download/v3.16.0/gobgp_3.16.0_linux_amd64.tar.gz
```
对于主机A:
```ini
[global.config]
  as = 65001
  router-id = "192.168.50.116"

[[neighbors]]
  [neighbors.config]
    neighbor-address = "192.168.50.199"
    peer-as = 65002
```

对于主机B:

```ini
[global.config]
  as = 65002
  router-id = "192.168.50.199"

[[neighbors]]
  [neighbors.config]
    neighbor-address = "192.168.50.116"
    peer-as = 65001
```

启动GoBGP daemon

```bash
gobgpd -f gobgpd.conf
```

查看bgp状态

```bash
root@base:~# gobgp neighbor
Peer              AS  Up/Down State       |#Received  Accepted
192.168.50.116 65001 00:00:12 Establ      |        0         0
```

### 创建并配置Docker网络

分别在两台主机上创建docker网络

```bash
docker network create --subnet=10.1.0.0/16 netA
docker network create --subnet=20.1.0.0/16 netB
```

分别创建两个容器

```
docker run -d --net=netA --ip=10.1.0.2 nginx
```

```
docker run -d --net=netB --ip=20.1.0.2 nginx
```

使用GoBGP广播这些Docker网络的路由。

在主机A上：

```
gobgp global rib add 20.1.0.0/16 nexthop 192.168.50.199 -a ipv4
```

在主机B上：

```
gobgp global rib add 10.1.0.0/16 nexthop 192.168.50.116 -a ipv4
```

在主机 A 上执行以下命令查看 GoBGP 的全局路由表信息：

```bash
root@base:~# gobgp global rib
   Network              Next Hop             AS_PATH              Age        Attrs
*> 10.1.0.0/16          192.168.50.116       65002                00:00:05   [{Origin: ?}]
*> 20.1.0.0/16          192.168.50.199                            00:00:09   [{Origin: ?}]
root@base:~# gobgp neighbor 192.168.50.199 adj-in
   ID  Network              Next Hop             AS_PATH              Age        Attrs
   0   10.1.0.0/16          192.168.50.116       65002                00:10:47   [{Origin: ?}]
```

可以使用 `gobgp global rib del` 命令来删除特定的路由项。

执行网络测试

```
docker exec -it fe991dc687e0 curl 20.1.0.2
```

访问失败, 查资料得知 `gobgp` 不负责更新系统的路由表, 需要借住`bird`等工具去动态修改, 这里就在两台主机上分别临时手动添加一条路由

```bash
ip route add 20.1.0.0/16 via 192.168.50.199
root@base:~# route -n
Destination     Gateway         Genmask         Flags Metric Ref    Use Iface
...
20.1.0.0        192.168.50.199  255.255.0.0     UG    0      0        0 enp1s0
```

再次执行

```
docker exec -it fe991dc687e0 curl 20.1.0.2
```

成功

```html
<!DOCTYPE html>
<html>
<head>
<title>Welcome to nginx!</title>
<style>
html { color-scheme: light dark; }
body { width: 35em; margin: 0 auto;
font-family: Tahoma, Verdana, Arial, sans-serif; }
</style>
</head>
<body>
<h1>Welcome to nginx!</h1>
<p>If you see this page, the nginx web server is successfully installed and
working. Further configuration is required.</p>

<p>For online documentation and support please refer to
<a href="http://nginx.org/">nginx.org</a>.<br/>
Commercial support is available at
<a href="http://nginx.com/">nginx.com</a>.</p>

<p><em>Thank you for using nginx.</em></p>
</body>
</html>
```

### bgp的意义

即使我不使用bgp协议, 直接配置路由表也能实现docker两个容器访问, 那bgp的意义是什么

GPT:

BGP (Border Gateway Protocol) 的价值在于其动态的、分布式的、自治的特性，让其可以在复杂的网络环境中寻找到最优的路由路径，以及动态应对网络环境的变化。以下是一些详细的解释：

1. **动态路由分发**：在你的示例中，确实可以通过手动配置路由表来实现 Docker 容器之间的通信。然而，当你的网络环境变得更为复杂，有更多的容器和更多的网络需要配置时，手动配置路由表就会变得非常困难。BGP 协议可以动态地学习和分发路由信息，这意味着当网络拓扑发生变化时，BGP 能够自动调整路由信息，保持网络通信的畅通。
2. **最优路由选择**：BGP 不仅可以自动地分发路由信息，它还可以根据各种策略选择最优的路由路径。例如，它可以选择最短的路径，或者根据网络拥塞情况选择最佳的路径。这些特性对于维持大规模网络的稳定性和性能至关重要。
3. **自治系统间路由**：在互联网环境中，BGP 作为自治系统间的路由协议，可以实现各个自治系统之间的路由信息交换和路由策略控制，从而实现全球互联网的稳定运行。每个自治系统可以根据自己的策略决定如何选择和分发路由信息。

总的来说，BGP 在简单的网络环境中，可能显得有些过于复杂，然而在大规模、复杂的网络环境中，BGP 的这些特性使其成为一个非常强大的工具。
