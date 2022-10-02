---
title: "runc的使用"
date: 2022-05-24T21:24:41+08:00
tags: ["k8s"]
---

runc 是一个命令行客户端，用于运行根据 Open Container Initiative (OCI) 格式打包的应用程序

![2021-01-27_cri-containerd2](http://inksnw.asuscomm.com:3001/blog/runc_cb5954ad2c7242b97e5cf81611d377b5.png)

上图是k8s调用`containerd`到拉起pod进程的流程,而拉起pod这个过程是由`runc`完成的

**疑问**

这个图来自[官方地址](https://github.com/containerd/containerd/blob/main/docs/cri/architecture.md)

> 从图中看,一个Pod中有多个容器时,会有多个shim进程,实际创建一个nginx+redis的pod,查看进程发现,pause容器与两个业务容器都是共用一同一个shim父进程

```bash
root     35602     1  0 11:52 ?        00:00:00 /usr/bin/containerd-shim-runc-v2 -namespace k8s.io -id 40098e2fb7e3397xxx -address /run/containerd/containerd.sock
65535    35628 35602  0 11:52 ?        00:00:00 /pause
root     35677 35602  0 11:52 ?        00:00:00 nginx: master process nginx -g daemon off;
systemd+ 35927 35602  0 11:52 ?        00:00:00 redis-server *:6379
```

本文介绍如何使用runc来启动容器,并共享网络空间与进程空间

# runc简单使用

生成busybox文件,建议在**另一台主机**上操作避免干扰

```bash
mkdir -p ~/busybox/
docker export $(docker create busybox) | tar -C ~/busybox/rootfs -xvf -
```
将busybox目录拷贝到实际操作的主机上

下载runc二进制

```bash
curl -LJO https://github.com/opencontainers/runc/releases/download/v1.1.2/runc.amd64
chmod +x runc.amd64
mv runc.amd64 /usr/local/bin/rc
rc -v
```

生成配置文件 

```bash
cd ~/busybox
rc spec
tree -L 2
#目录结构
.
└── busybox
    ├── config.json
    └── rootfs
```

启动容器

```bash
cd ~/busybox
rc run abc
```
查看运行的容器列表
```bash
rc list
ID          PID         STATUS      BUNDLE          CREATED                          OWNER
abc         5147        running     /root/busybox   2022-05-24T07:20:42.573306874Z   root
```

# runc挂载程序使用

写一个最简的程序,编译为main

```go
package main

import (
   "fmt"
   "net/http"
)

func HelloHandler(w http.ResponseWriter, r *http.Request) {
   fmt.Fprintf(w, "Hello World")
}

func main () {
   http.HandleFunc("/", HelloHandler)
   http.ListenAndServe(":8000", nil)
}
```

将main程序上传到主机的**/root/app** 目录

修改配置文件config.json


```diff
- "terminal": true,
+ "terminal": false,
```

```diff
- "readonly": true
+ "readonly": false
```

```diff
+{
+			"destination": "/app",
+			"type": "bind",
+			"source": "/root/app",
+			"options" : ["rbind","rw"]
+		},
```

```diff
- "sh"
+ "/app/main"
```

运行

```bash
cd ~/busybox
rc run abc 
```

# runc共享网络空间

## 配置容器网络

创建网桥

```bash
#创建网桥,启动
brctl addbr mybr
ip link set mybr up
ip addr add 10.12.0.1/24 dev mybr
```

创建veth设备

```bash
# veth0-host连接到主机
# veth0-ns 连接到容器
ip link add name veth0-host type veth peer name veth0-ns
ip link set veth0-host up 
#一端连接到网桥上
brctl addif mybr veth0-host
brctl show
#另一端加入到ns中
#添加网络ns
ip netns add mycontainer
ip netns list
# 将veth的另一端加入到ns中
ip link set veth0-ns netns mycontainer
```
配置ns中的网卡
```bash
# 查看网络信息
ip netns exec mycontainer ip a
# 改个名字
ip netns exec mycontainer ip link set veth0-ns name eth0
# 设置ip地址
ip netns exec mycontainer ip addr add 10.12.0.2/24 dev eth0
# 启动网卡
ip netns exec mycontainer ip link set eth0 up
# 给回环地址设置ip
ip netns exec mycontainer ip addr add 127.0.0.1 dev lo
# 启动lo
ip netns exec mycontainer ip link set lo up
# 查看ns的路由
ip netns exec mycontainer route -n
# 给ns内指定路由,让请求都转发到网桥上
ip netns exec mycontainer ip route add default via 10.12.0.1
# 测试
ip netns exec mycontainer ping 127.0.0.1
ip netns exec mycontainer ping 10.12.0.1
ip netns exec mycontainer ping 10.12.0.2
```

## 关联容器与网络空间

修改config.json

```diff
{
"type": "network",
+ "path":"/var/run/netns/mycontainer"
}
```

```bash
#再次启动容器
rc run abc
#在宿主机配置iptables
iptables -t nat -I PREROUTING -p tcp -m tcp --dport 9999 -j DNAT --to-destination 10.12.0.2:8000
#查看
iptables -t nat -nvL
```

访问宿主机

```bash
http://192.168.50.209:9999/
```

## 多容器共享网络空间

共享的关键是使用config.json配置同一份namespaces下的network空间

```bash
# 修改启动参数
"args": ["/app/main","-p","8081"]
rc run -d web1 > web1.out 2>&1
# 修改启动参数
"args": ["/app/main","-p","8082"]
rc run -d web2 > web2.out 2>&1
```

# runc共享进程空间

解压pause容器

```bash
docker pull mirrorgooglecontainers/pause-amd64:3.1
docker tag mirrorgooglecontainers/pause-amd64:3.1 pause:3.1
mkdir -p pause/rootfs
docker export $(docker create pause:3.1)|tar -C pause/rootfs -xvf -
cd pause
rc spec
```

修改config.json文件的启动命令为/pause

修改 "terminal": false,

启动

```
rc run -d pause > pause.out 2>&1
```

共享资源的本质

如果 /proc/{pid}/ns/ 下的某些资源是同一份,就说明这两个进程共享了这些资源

如果想让其它容器使用pause的网络名称空间,可如下操作后,再执行上面的veth相关操作

```bash
ln -s /proc/{pause的进程号}/ns/net /var/run/netns/abc
```

进入不同的ns

```
nsenter -t {pid} -n 
```

共享pid&ipc

在pid/ipc下添加如下命令即可,进程号为pause的进程号

```diff
+ "path": "/proc/36817/ns/pid"
+ "path": "/proc/36817/ns/ipc"
```









