---
title: "使用nsenter调试容器网络"
date: 2022-11-27T15:15:23+08:00
tags: ["k8s"]
---

## 获取容器 ID

使用 kubectl 获取 pod 中任意 cotnainer 的 id:

```bash
kubectl -n test describe pod debug-685b48bcf5-ggn5d
```

containerd运行时

```txt
Containers:
  debug:
    Container ID:   containerd://529bbd5c935562a9ba66fc9b9ffa95d486c6324f26d8253d744ffe3dfd728289
```

dockerd运行时

```txt
Containers:
  debug:
    Container ID:   docker://e64939086488a9302821566b0c1f193b755c805f5ff5370d5ce5e6f154ffc648 
```

## 获取 PID

拿到 container id 后，我们登录到 pod **所在节点**上去获取其主进程 pid。

containerd 运行时使用 crictl 命令获取:

```bash
➜ crictl inspect 529bbd5c935562a9ba66fc9b9ffa95d486c6324f26d8253d744ffe3dfd728289 | grep -i pid
    "pid": 2266462,
            "pid": 1
            "type": "pid"
```

> 此例中 pid 为 2266462

dockerd 运行时使用 docker 命令获取:

```bash
➜ docker inspect e64939086488a9302821566b0c1f193b755c805f5ff5370d5ce5e6f154ffc648 | grep -i pid
            "Pid": 910351,
            "PidMode": "",
            "PidsLimit": 0,
```

> 此例中 pid 为 910351

## 使用 nsenter 进入容器 netns

在节点上使用 nsenter 进入 pod 的 netns:

```bash
nsenter --target 910351 --mount --uts --ipc --net --pid 
```

nsenter指令中进程id之后的参数的含义：

- – mount 参数是进去到mount namespace中
- – uts 参数是进入到uts namespace中
- – ipc 参数是进入到System V IPC namaspace中
- – net 参数是进入到network namespace中
- – pid 参数是进入到pid namespace中
- – user 参数是进入到user namespace中

## 调试网络

成功进入容器的 netns，可以使用节点上的网络工具进行调试网络，可以首先使用 `ip a` 验证下 ip 地址是否为 pod ip:

```bash
➜ ip a
1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue state UNKNOWN group default qlen 1000
    link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
    inet 127.0.0.1/8 scope host lo
       valid_lft forever preferred_lft forever
3: eth0@if8: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UP group default
    link/ether 6a:c6:6f:67:dd:6c brd ff:ff:ff:ff:ff:ff link-netnsid 0
    inet 172.18.0.67/26 brd 172.18.0.127 scope global eth0
       valid_lft forever preferred_lft forever
```

如果要抓包也可以利用节点上的 tcpdump 工具抓包。
