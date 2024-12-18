---
title: "runc的使用"
date: 2022-05-24T21:24:41+08:00
tags: ["k8s"]
---

runc 是一个命令行客户端，用于运行根据 Open Container Initiative (OCI) 格式打包的应用程序

<img src="https://inksnw.asuscomm.com:3001/blog/runc_cb5954ad2c7242b97e5cf81611d377b5.png" alt="2021-01-27_cri-containerd2" style="zoom: 50%;" />

上图是k8s调用`containerd`到拉起pod进程的流程,拉起pod这个过程是由调用二进制的`runc`完成的,拉起后`runc`退出,不作为常驻进程

**疑问**

这个图来自[官方地址](https://github.com/containerd/containerd/blob/main/docs/cri/architecture.md)

> 从图中看,一个Pod中有多个容器时,会有多个shim进程,实际创建一个nginx+redis的pod,查看进程发现,pause容器与两个业务容器都是共用一同一个shim父进程

```bash
root     35602     1  0 11:52 ?        00:00:00 /usr/bin/containerd-shim-runc-v2 -namespace k8s.io -id 40098e2fb7e3397xxx -address /run/containerd/containerd.sock
65535    35628 35602  0 11:52 ?        00:00:00 /pause
root     35677 35602  0 11:52 ?        00:00:00 nginx: master process nginx -g daemon off;
systemd+ 35927 35602  0 11:52 ?        00:00:00 redis-server *:6379
```

下面介绍一下如何使用runc来启动容器,并共享网络空间与进程空间

# runc简单使用

环境准备

一台纯净的主机,只安装了docker

开启ip转发

```bash
echo 1 > /proc/sys/net/ipv4/ip_forward
```

永久生效

```bash
vi /etc/sysctl.conf
#添加如下值
net.ipv4.ip_forward = 1
sysctl -p /etc/sysctl.conf
```

```bash
➜ mkdir -p ~/busybox/rootfs
➜ docker export $(docker create busybox) | tar -C ~/busybox/rootfs -xvf -
```
下载runc二进制

```bash
curl -LJO https://github.com/opencontainers/runc/releases/download/v1.1.4/runc.amd64
chmod +x runc.amd64
mv runc.amd64 /usr/local/bin/rc
rc -v
```

生成配置文件 

```bash
➜ cd ~/busybox
➜ rc spec
➜ tree -L 2
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
另外开启一个终端,查看运行的容器列表
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
在mount段下添加挂载,在物理机的`/root/app`目录下放置刚编译的程序`main`
```diff
+{
+			"destination": "/app",
+			"type": "bind",
+			"source": "/root/app",
+			"options" : ["rbind","rw"]
+	},
```
修改`process.args`内容
```diff
- "sh"
+ "/app/main"
```

目录结构

```bash
➜ chmod +x ~/app/main
➜ tree -L 2
.
├── app
│   └── main
└── busybox
    ├── config.json
    └── rootfs
```

运行

```bash
cd ~/busybox
rc run abc 
```

# runc共享网络空间

## 配置容器网络

容器网络基本原理

<img src="https://inksnw.asuscomm.com:3001/blog/runc_582804a69ecbeb334c544888b2521d08.png" alt="rancher_blog_image12-1" style="zoom: 67%;" />

创建网桥

```bash
#创建网桥,启动
➜ brctl addbr mybr
➜ brctl show
bridge name     bridge id               STP enabled     interfaces
docker0         8000.02429ea3d5dd       no
mybr            8000.000000000000       no
➜ ip link set mybr up
➜ ip addr add 10.12.0.1/24 dev mybr
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
cd ~/busybox
rc run abc
#在宿主机配置iptables
iptables -t nat -I PREROUTING -p tcp -m tcp --dport 9999 -j DNAT --to-destination 10.12.0.2:8000
#查看
iptables -t nat -nvL
```

访问宿主机

```bash
➜ curl http://192.168.50.231:9999/
Hello world
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

# 限制cpu的使用

在/sys/fs/cgroup/cpu 目录中,有cpu的信息,linux通过`cfs` Completely Fair Scheduler 完全公平调度器来实现cpu调度

cfs_quota_us/cfs_period_us=0.1 好比只能只用0.1个cpu

```bash
# 一个cfs调度时间周期长度,默认为100000微秒
echo 100000 > cpu.cfs_period_us
# 在上面的一个周期内,允许运行的时间,-1为不限制
echo 10000 > cpu.cfs_quota_us
```

在runc的配置文件中添加以下内容实现cpu限制

```json
{
    "linux": {
        "resources": {
            "cpu": {
                "quota": 10000,
                "period": 100000
            }
        }
    }
}
```

进入容器查看相应文件

```bash
cat /sys/fs/cgroup/cpu/cpu.cfs_period_us
cat /sys/fs/cgroup/cpu/cpu.cfs_quota_us
```









