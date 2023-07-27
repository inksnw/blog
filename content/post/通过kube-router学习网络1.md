---
title: "通过kube Router学习网络1"
date: 2023-07-27T10:57:51+08:00
tags: ["cni"]
---

一直以来想详细的梳理清楚cni这块, 但知识总是比较零碎, 恰好看到了`kube-router` 这个项目, cni,bgp,ipvs全囊括了, 好吧就它了, 让我们一起把整个cni都梳理一遍

## 环境准备

这里用`kubekey`快速安装k8s,搞一个快速重建的shell脚本

```bash
#!/bin/bash

# Step 1: Delete cluster
echo "Step 1: Deleting cluster..."
echo "yes" | kk delete cluster -f config-simple.yaml  # 自动回复yes
echo "Step 1: Cluster deletion completed."

HOSTS=("node1" "node2" "node3")
COMMAND1="ip link delete kube-dummy-if"
COMMAND2="ip link delete kube-bridge"
for HOST in "${HOSTS[@]}"; do
  ssh ${HOST} "${COMMAND1}"
  ssh ${HOST} "${COMMAND2}"
done
echo "删除网卡完成"

# Step 2: Create cluster
echo "Step 2: Creating cluster..."
echo "yes" | kk create cluster -f config-simple.yaml --with-local-storage --skip-pull-images  # 自动回复yes
echo "Step 2: Cluster creation completed."

kubectl get pod -A
```

config-simple.yaml

```yaml
apiVersion: kubekey.kubesphere.io/v1alpha2
kind: Cluster
metadata:
  name: sample
spec:
  hosts:
  - {name: node1, address: 192.168.50.50, internalAddress: 192.168.50.50, user: root, password: "123"}
  - {name: node2, address: 192.168.50.51, internalAddress: 192.168.50.51, user: root, password: "123"}
  - {name: node3, address: 192.168.50.52, internalAddress: 192.168.50.52, user: root, password: "123"}
  roleGroups:
    etcd:
    - node1
    control-plane: 
    - node1
    worker:
    - node1
    - node2
    - node3
  controlPlaneEndpoint:
    domain: lb.kubesphere.local
    address: ""
    port: 6443
  kubernetes:
    version: v1.26.5
    clusterName: cluster.local
    autoRenewCerts: true
    containerManager: containerd
    disableKubeProxy: true
    nodelocaldns: false
  etcd:
    type: kubekey
  network:
    plugin: fake
    kubeServiceCIDR: 10.233.0.0/18
    multusCNI:
      enabled: false
  registry:
    privateRegistry: ""
    namespaceOverride: ""
    registryMirrors: []
    insecureRegistries: []
  addons: []
```

安装完成后, 可以看到一个没有`kube-proxy`,没有`cni`的集群

```bash
root@node1:~# kubectl get pod -A
NAMESPACE     NAME                                          READY   STATUS    RESTARTS   AGE
kube-system   coredns-7f647946c8-4pf2t                      0/1     Pending   0          11m
kube-system   coredns-7f647946c8-5zw5f                      0/1     Pending   0          11m
kube-system   kube-apiserver-node1                          1/1     Running   0          11m
kube-system   kube-controller-manager-node1                 1/1     Running   0          11m
kube-system   kube-scheduler-node1                          1/1     Running   0          11m
kube-system   openebs-localpv-provisioner-7cc4c84b9-t4gr4   0/1     Pending   0          11m
```

### 路由信息

CNI (Container Network Interface) 的主要任务之一是确保不同主机上的容器网络能够互相通信。实现这一目标通常有两种方法：`overlay` 和 `underlay`。

在 `overlay` 网络中，网络包被封装在其他网络包中，然后通过底层的物理网络进行传输。这种方法对底层网络的要求较低，但可能会引入一些额外的性能开销。

相比之下，`underlay` 网络则直接利用底层的物理网络。在这种情况下，我们需要在路由表中添加路由规则，以确保数据包可以正确地路由到目标容器。然而，随着主机的变动，手动配置路由表通常是不切实际的。为了解决这个问题，我们可以使用 BGP (Border Gateway Protocol) 等动态路由协议，让路由器能够自动地学习和配置路由规则。

查看此时主机上的路由表, 并没有pod `cidr`相关的路由

> 清空路由表命令 ip route flush table main , 注意清空可能导致无法远程连接, 重启后恢复一些默认的

```bash
root@node1:~# route -n
Kernel IP routing table
Destination     Gateway         Genmask         Flags Metric Ref    Use Iface
0.0.0.0         192.168.50.1    0.0.0.0         UG    100    0        0 enp1s0
192.168.50.0    0.0.0.0         255.255.255.0   U     0      0        0 enp1s0
192.168.50.1    0.0.0.0         255.255.255.255 UH    100    0        0 enp1s0
```
### 网卡信息

CNI（Container Network Interface）插件的任务之一就是负责在容器中创建和配置网络接口。所以，当一个新的 Pod 被创建时，CNI 插件通常会在这个 Pod 的网络命名空间中添加一个或多个新的网络接口（也就是虚拟网卡）。

这些网络接口是 Pod 与外界通信的桥梁。每个接口都会被分配一个 IP 地址（通常来自节点的 Pod CIDR 范围）

```
root@node1:~# ip addr
1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue state UNKNOWN group default qlen 1000
    link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
    inet 127.0.0.1/8 scope host lo
       valid_lft forever preferred_lft forever
    inet6 ::1/128 scope host 
       valid_lft forever preferred_lft forever
2: enp1s0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc fq_codel state UP group default qlen 1000
    link/ether 52:54:00:34:d1:8a brd ff:ff:ff:ff:ff:ff
    inet 192.168.50.50/24 brd 192.168.50.255 scope global dynamic enp1s0
       valid_lft 74620sec preferred_lft 74620sec
    inet6 fe80::5054:ff:fe34:d18a/64 scope link 
       valid_lft forever preferred_lft forever
root@node1:~# kubectl get pod -A
NAMESPACE     NAME                                          READY   STATUS    RESTARTS   AGE
kube-system   coredns-7f647946c8-4hr6d                      0/1     Pending   0          3h9m
kube-system   coredns-7f647946c8-lngk8                      0/1     Pending   0          3h9m
kube-system   kube-apiserver-node1                          1/1     Running   0          3h9m
kube-system   kube-controller-manager-node1                 1/1     Running   0          3h9m
kube-system   kube-scheduler-node1                          1/1     Running   0          3h9m
kube-system   openebs-localpv-provisioner-7cc4c84b9-d5tpw   0/1     Pending   0          3h8m
root@node1:~# kubectl get node
NAME    STATUS     ROLES                  AGE    VERSION
node1   NotReady   control-plane,worker   3h9m   v1.26.5
node2   NotReady   worker                 3h9m   v1.26.5
node3   NotReady   worker                 3h9m   v1.26.5
root@node1:~# arp -n
Address                  HWtype  HWaddress           Flags Mask            Iface
192.168.50.51            ether   52:54:00:8b:47:fd   C                     enp1s0
192.168.50.22            ether   40:ec:99:bb:84:b0   C                     enp1s0
192.168.50.52            ether   52:54:00:19:ae:7d   C                     enp1s0
192.168.50.1             ether   24:4b:fe:d4:81:00   C                     enp1s0
```
### iptables与ipvs

```bash
root@node1:~# iptables -L
Chain INPUT (policy ACCEPT)
target     prot opt source               destination         
KUBE-FIREWALL  all  --  anywhere             anywhere            

Chain FORWARD (policy ACCEPT)
target     prot opt source               destination         

Chain OUTPUT (policy ACCEPT)
target     prot opt source               destination         
KUBE-FIREWALL  all  --  anywhere             anywhere            

Chain KUBE-FIREWALL (2 references)
target     prot opt source               destination         
DROP       all  -- !localhost/8          localhost/8          /* block incoming localnet connections */ ! ctstate RELATED,ESTABLISHED,DNAT
DROP       all  --  anywhere             anywhere             /* kubernetes firewall for dropping marked packets */ mark match 0x8000/0x8000

Chain KUBE-KUBELET-CANARY (0 references)
target     prot opt source               destination         
root@node1:~# ipvsadm
IP Virtual Server version 1.2.1 (size=4096)
Prot LocalAddress:Port Scheduler Flags
  -> RemoteAddress:Port           Forward Weight ActiveConn InActConn
```

## 安装插件

```bash
kubectl --namespace=kube-system create configmap kube-proxy  --from-file=kubeconfig.conf=/root/.kube/config
kubectl create -f https://raw.githubusercontent.com/cloudnativelabs/kube-router/master/daemonset/kubeadm-kuberouter-all-features-dsr.yaml
```

### 查看路由表

```bash
root@node1:~# route -n
Kernel IP routing table
Destination     Gateway         Genmask         Flags Metric Ref    Use Iface
0.0.0.0         192.168.50.1    0.0.0.0         UG    100    0        0 enp1s0
10.233.65.0     192.168.50.52   255.255.255.0   UG    0      0        0 enp1s0
10.233.66.0     192.168.50.51   255.255.255.0   UG    0      0        0 enp1s0
192.168.50.0    0.0.0.0         255.255.255.0   U     0      0        0 enp1s0
192.168.50.1    0.0.0.0         255.255.255.255 UH    100    0        0 enp1s0
```

> Calico 的 IP 地址管理（IPAM）策略默认并不会为每个节点分配一个固定的 IP 地址范围。在默认情况下，Calico 会从一个全局的 CIDR 池中为每个新创建的 Pod 分配 IP 地址。这意味着一个节点上的 Pod 可以获得任何可用的 IP 地址，而不仅仅是来自某个特定的子网。

在路由表中，主机 192.168.50.52 对应的 Pod CIDR 是 10.233.65.0/24，这意味着这个节点上的 Pod 可以使用的 IP 地址范围是 10.233.65.1 到 10.233.65.254。

实际环境中，受到各种因素的限制, 每个节点上可以运行的 Pod 数量可能会小于这个数字

### 查看网卡

```
root@node1:~# ip addr
...
3: kube-bridge: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UNKNOWN group default qlen 1000
    link/ether ea:87:60:eb:65:e8 brd ff:ff:ff:ff:ff:ff
    inet6 fe80::e887:60ff:feeb:65e8/64 scope link 
       valid_lft forever preferred_lft forever
4: kube-dummy-if: <BROADCAST,NOARP,UP,LOWER_UP> mtu 1500 qdisc noqueue state UNKNOWN group default 
    link/ether aa:dc:35:92:2a:31 brd ff:ff:ff:ff:ff:ff
    inet 10.233.0.1/32 scope link kube-dummy-if
       valid_lft forever preferred_lft forever
    inet 10.233.0.3/32 scope link kube-dummy-if
       valid_lft forever preferred_lft forever
    inet6 fe80::a8dc:35ff:fe92:2a31/64 scope link 
       valid_lft forever preferred_lft forever
```

## 操作验证

### 关闭网络策略功能

```bash
# - --run-firewall=true改为false
kubectl edit ds kube-router -n kube-system
```

查看ipset信息, 此时即使创建网络策略, 也不会有效果, ipset结果中不会有网络策略相关的信息 

```basic
root@node1:~# ipset --list
# 子网信息, 即上文的路由信息
Name: kube-router-pod-subnets
Type: hash:net
Revision: 6
Header: family inet hashsize 1024 maxelem 65536 timeout 0
Size in memory: 736
References: 2
Number of entries: 3
Members: 
10.233.64.0/24 timeout 0
10.233.65.0/24 timeout 0
10.233.66.0/24 timeout 0

# 节点信息
Name: kube-router-node-ips
Type: hash:ip
Revision: 4
Header: family inet hashsize 1024 maxelem 65536 timeout 0
Size in memory: 488
References: 1
Number of entries: 3
Members:
192.168.50.51 timeout 0
192.168.50.50 timeout 0
192.168.50.52 timeout 0

# 本机信息
Name: kube-router-local-ips
Type: hash:ip
Revision: 4
Header: family inet hashsize 1024 maxelem 65536 timeout 0
Size in memory: 392
References: 1
Number of entries: 2
Members:
127.0.0.1 timeout 0
192.168.50.50 timeout 0

# service 信息
Name: kube-router-service-ips
Type: hash:ip
Revision: 4
Header: family inet hashsize 1024 maxelem 65536 timeout 0
Size in memory: 392
References: 1
Number of entries: 2
Members:
10.233.0.1 timeout 0
10.233.0.3 timeout 0

# svc转发信息, 10.233.0.1 是kubernetes的clusterip, 10.233.0.3是coredns的clusterip
Name: kube-router-ipvs-services
Type: hash:ip,port
Revision: 5
Header: family inet hashsize 1024 maxelem 65536 timeout 0
Size in memory: 576
References: 1
Number of entries: 4
Members:
10.233.0.1,tcp:443 timeout 0
10.233.0.3,tcp:53 timeout 0
10.233.0.3,tcp:9153 timeout 0
10.233.0.3,udp:53 timeout 0
```

### 删除节点

当前路由

```bash
root@node1:~# route -n 
Kernel IP routing table
Destination     Gateway         Genmask         Flags Metric Ref    Use Iface
0.0.0.0         192.168.50.1    0.0.0.0         UG    100    0        0 enp1s0
10.233.65.0     192.168.50.52   255.255.255.0   UG    0      0        0 enp1s0
10.233.66.0     192.168.50.51   255.255.255.0   UG    0      0        0 enp1s0
192.168.50.0    0.0.0.0         255.255.255.0   U     0      0        0 enp1s0
192.168.50.1    0.0.0.0         255.255.255.255 UH    100    0        0 enp1s0
```

删除节点后, 可以看到, 相关的路由已被清理

```bash
kubectl delete node node3
root@node1:~# route -n
Kernel IP routing table
Destination     Gateway         Genmask         Flags Metric Ref    Use Iface
0.0.0.0         192.168.50.1    0.0.0.0         UG    100    0        0 enp1s0
10.233.66.0     192.168.50.51   255.255.255.0   UG    0      0        0 enp1s0
192.168.50.0    0.0.0.0         255.255.255.0   U     0      0        0 enp1s0
192.168.50.1    0.0.0.0         255.255.255.255 UH    100    0        0 enp1s0
```

添加回节点, 查看路由表可以看到已经添加了相关信息

> 当你使用 `kubectl delete node <node-name>` 命令时，Kubernetes 会从集群状态中移除节点，但并不会停止节点上的 kubelet 服务。因此，当你重启 kubelet 服务时，kubelet 会尝试重新将节点注册到 Kubernetes 集群。这就是为什么重启 kubelet 会使节点自动加回来的原因。

```bash
systemctl restart kubelet
kubectl label nodes node3  node-role.kubernetes.io/worker=
```

### 抓包查看BGP信息

删除node3上的 `kube-router` pod

```
kubectl delete pod kube-router-nf9pb -n kube-system
```

在node1上抓包查看node3上的 `kube-router` pod被重新拉起的数据包

```bash
tcpdump -i any 'port 179 and host 192.168.50.52' -v -n
tcpdump: listening on any, link-type LINUX_SLL (Linux cooked v1), capture size 262144 bytes
08:11:50.770802 IP (tos 0x0, ttl 64, id 26890, offset 0, flags [DF], proto TCP (6), length 60)
    192.168.50.50.54651 > 192.168.50.52.179: Flags [S], cksum 0xe5e5 (incorrect -> 0x2d26), seq 3986906876, win 64240, options [mss 1460,sackOK,TS val 3629793029 ecr 0,nop,wscale 10], length 0
08:11:50.771428 IP (tos 0x0, ttl 255, id 0, offset 0, flags [DF], proto TCP (6), length 60)
    192.168.50.52.179 > 192.168.50.50.54651: Flags [S.], cksum 0xe5e5 (incorrect -> 0x96d6), seq 3060303424, ack 3986906877, win 65160, options [mss 1460,sackOK,TS val 2666051348 ecr 3629793029,nop,wscale 10], length 0
08:11:50.771502 IP (tos 0x0, ttl 64, id 26891, offset 0, flags [DF], proto TCP (6), length 52)
    192.168.50.50.54651 > 192.168.50.52.179: Flags [.], cksum 0xe5dd (incorrect -> 0xc3ee), ack 1, win 63, options [nop,nop,TS val 3629793030 ecr 2666051348], length 0
08:11:50.772053 IP (tos 0x0, ttl 64, id 26892, offset 0, flags [DF], proto TCP (6), length 122)
    192.168.50.50.54651 > 192.168.50.52.179: Flags [P.], cksum 0xe623 (incorrect -> 0xf381), seq 1:71, ack 1, win 63, options [nop,nop,TS val 3629793031 ecr 2666051348], length 70: BGP
        Open Message (1), length: 70
          Version 4, my AS 64512, Holdtime 90s, ID 192.168.50.50
          Optional parameters, length: 41
            Option Capabilities Advertisement (2), length: 39
              Route Refresh (2), length: 0
              Unknown (73), length: 7
                no decoder for Capability 73
                0x0000:  056e 6f64 6531 00
              Multiprotocol Extensions (1), length: 4
                AFI IPv4 (1), SAFI Unicast (1)
              32-Bit AS Number (65), length: 4
                 4 Byte AS 64512
              Graceful Restart (64), length: 6
                Restart Flags: [none], Restart Time 90s
                  AFI IPv4 (1), SAFI Unicast (1), Forwarding state preserved: no
              Extended Next Hop Encoding (5), length: 6
                no decoder for Capability 5
                0x0000:  0001 0001 0002
08:11:50.772597 IP (tos 0x0, ttl 255, id 38615, offset 0, flags [DF], proto TCP (6), length 52)
    192.168.50.52.179 > 192.168.50.50.54651: Flags [.], cksum 0xe5dd (incorrect -> 0xc3a5), ack 71, win 64, options [nop,nop,TS val 2666051349 ecr 3629793031], length 0
08:11:50.772907 IP (tos 0x0, ttl 255, id 38616, offset 0, flags [DF], proto TCP (6), length 122)
    192.168.50.52.179 > 192.168.50.50.54651: Flags [P.], cksum 0xe623 (incorrect -> 0x70b6), seq 1:71, ack 71, win 64, options [nop,nop,TS val 2666051350 ecr 3629793031], length 70: BGP
        Open Message (1), length: 70
          Version 4, my AS 64512, Holdtime 90s, ID 192.168.50.52
          Optional parameters, length: 41
            Option Capabilities Advertisement (2), length: 39
              Route Refresh (2), length: 0
              Unknown (73), length: 7
                no decoder for Capability 73
                0x0000:  056e 6f64 6533 00
              Multiprotocol Extensions (1), length: 4
                AFI IPv4 (1), SAFI Unicast (1)
              32-Bit AS Number (65), length: 4
                 4 Byte AS 64512
              Graceful Restart (64), length: 6
                Restart Flags: [R], Restart Time 90s
                  AFI IPv4 (1), SAFI Unicast (1), Forwarding state preserved: yes
              Extended Next Hop Encoding (5), length: 6
                no decoder for Capability 5
                0x0000:  0001 0001 0002
08:11:50.772977 IP (tos 0x0, ttl 64, id 26893, offset 0, flags [DF], proto TCP (6), length 52)
    192.168.50.50.54651 > 192.168.50.52.179: Flags [.], cksum 0xe5dd (incorrect -> 0xc35e), ack 71, win 63, options [nop,nop,TS val 3629793032 ecr 2666051350], length 0
08:11:50.773312 IP (tos 0x0, ttl 255, id 38617, offset 0, flags [DF], proto TCP (6), length 71)
    192.168.50.52.179 > 192.168.50.50.54651: Flags [P.], cksum 0xe5f0 (incorrect -> 0xbf30), seq 71:90, ack 71, win 64, options [nop,nop,TS val 2666051350 ecr 3629793031], length 19: BGP
        Keepalive Message (4), length: 19
08:11:50.773359 IP (tos 0x0, ttl 64, id 26894, offset 0, flags [DF], proto TCP (6), length 52)
    192.168.50.50.54651 > 192.168.50.52.179: Flags [.], cksum 0xe5dd (incorrect -> 0xc34b), ack 90, win 63, options [nop,nop,TS val 3629793032 ecr 2666051350], length 0
08:11:50.773866 IP (tos 0x0, ttl 64, id 26895, offset 0, flags [DF], proto TCP (6), length 71)
    192.168.50.50.54651 > 192.168.50.52.179: Flags [P.], cksum 0xe5f0 (incorrect -> 0xbf1d), seq 71:90, ack 90, win 63, options [nop,nop,TS val 3629793032 ecr 2666051350], length 19: BGP
        Keepalive Message (4), length: 19
08:11:50.774194 IP (tos 0x0, ttl 255, id 38618, offset 0, flags [DF], proto TCP (6), length 52)
    192.168.50.52.179 > 192.168.50.50.54651: Flags [.], cksum 0xe5dd (incorrect -> 0xc336), ack 90, win 64, options [nop,nop,TS val 2666051351 ecr 3629793032], length 0
08:11:50.774963 IP (tos 0x0, ttl 64, id 26896, offset 0, flags [DF], proto TCP (6), length 100)
    192.168.50.50.54651 > 192.168.50.52.179: Flags [P.], cksum 0xe60d (incorrect -> 0x8263), seq 90:138, ack 90, win 63, options [nop,nop,TS val 3629793034 ecr 2666051351], length 48: BGP
        Update Message (2), length: 48
          Origin (1), length: 1, Flags [T]: IGP
          AS Path (2), length: 0, Flags [T]: empty
          Next Hop (3), length: 4, Flags [T]: 192.168.50.50
          Local Preference (5), length: 4, Flags [T]: 100
          Updated routes:
            10.233.64.0/24
08:11:50.775311 IP (tos 0x0, ttl 255, id 38619, offset 0, flags [DF], proto TCP (6), length 52)
    192.168.50.52.179 > 192.168.50.50.54651: Flags [.], cksum 0xe5dd (incorrect -> 0xc303), ack 138, win 64, options [nop,nop,TS val 2666051352 ecr 3629793034], length 0
08:11:50.775656 IP (tos 0x0, ttl 64, id 26897, offset 0, flags [DF], proto TCP (6), length 75)
    192.168.50.50.54651 > 192.168.50.52.179: Flags [P.], cksum 0xe5f4 (incorrect -> 0xc0ce), seq 138:161, ack 90, win 63, options [nop,nop,TS val 3629793034 ecr 2666051352], length 23: BGP
        Update Message (2), length: 23
          End-of-Rib Marker (empty NLRI)
08:11:50.775929 IP (tos 0x0, ttl 255, id 38620, offset 0, flags [DF], proto TCP (6), length 52)
    192.168.50.52.179 > 192.168.50.50.54651: Flags [.], cksum 0xe5dd (incorrect -> 0xc2eb), ack 161, win 64, options [nop,nop,TS val 2666051353 ecr 3629793034], length 0
08:11:51.777028 IP (tos 0x0, ttl 255, id 38621, offset 0, flags [DF], proto TCP (6), length 100)
    192.168.50.52.179 > 192.168.50.50.54651: Flags [P.], cksum 0xe60d (incorrect -> 0x7c2c), seq 90:138, ack 161, win 64, options [nop,nop,TS val 2666052354 ecr 3629793034], length 48: BGP
        Update Message (2), length: 48
          Origin (1), length: 1, Flags [T]: IGP
          AS Path (2), length: 0, Flags [T]: empty
          Next Hop (3), length: 4, Flags [T]: 192.168.50.52
          Local Preference (5), length: 4, Flags [T]: 100
          Updated routes:
            10.233.68.0/24
08:11:51.777029 IP (tos 0x0, ttl 255, id 38622, offset 0, flags [DF], proto TCP (6), length 75)
    192.168.50.52.179 > 192.168.50.50.54651: Flags [P.], cksum 0xe5f4 (incorrect -> 0xbc9c), seq 138:161, ack 161, win 64, options [nop,nop,TS val 2666052354 ecr 3629793034], length 23: BGP
        Update Message (2), length: 23
          End-of-Rib Marker (empty NLRI)
```

可以看到node1告诉了node3自己的pod子网段是`10.233.64.0/24`, node3告诉node1自己的pod子网段是`10.233.68.0/24`

查看路由表确认

```
root@node1:~# route -n
Kernel IP routing table
Destination     Gateway         Genmask         Flags Metric Ref    Use Iface
0.0.0.0         192.168.50.1    0.0.0.0         UG    100    0        0 enp1s0
10.233.66.0     192.168.50.51   255.255.255.0   UG    0      0        0 enp1s0
10.233.68.0     192.168.50.52   255.255.255.0   UG    0      0        0 enp1s0
192.168.50.0    0.0.0.0         255.255.255.0   U     0      0        0 enp1s0
192.168.50.1    0.0.0.0         255.255.255.255 UH    100    0        0 enp1s0
```

## 通信实现

创建一个pod

```bash
kubectl run busybox --image=busybox -- /bin/sh -c "while true; do sleep 3600; done"
```

```bash
ip addr
...
7: vethe5393dc7@if2: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue master kube-bridge state UP group default 
    link/ether aa:f2:09:2c:26:2d brd ff:ff:ff:ff:ff:ff link-netns cni-c9f73dcd-802a-f008-f0e0-f052494c3d43
    inet6 fe80::a8f2:9ff:fe2c:262d/64 scope link 
       valid_lft forever preferred_lft forever
```

可以看到veth网卡在宿主机的编号是7, 在容器里的编号是2, 我们进入容器看一下, 可以看到网卡编号与外部对应

```bash
kubectl exec -it busybox -- /bin/sh
/ # ip a
1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue qlen 1000
    link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
    inet 127.0.0.1/8 scope host lo
       valid_lft forever preferred_lft forever
    inet6 ::1/128 scope host 
       valid_lft forever preferred_lft forever
2: eth0@if7: <BROADCAST,MULTICAST,UP,LOWER_UP,M-DOWN> mtu 1500 qdisc noqueue 
    link/ether e6:c6:dc:64:bf:d9 brd ff:ff:ff:ff:ff:ff
    inet 10.233.68.4/24 brd 10.233.68.255 scope global eth0
       valid_lft forever preferred_lft forever
    inet6 fe80::e4c6:dcff:fe64:bfd9/64 scope link 
       valid_lft forever preferred_lft forever
```

查看一下网桥信息, 可以看到kube-bridge上挂的接口`vethe5393dc7`与网卡名也能对应上

```bash
root@node3:~# brctl show
bridge name     bridge id               STP enabled     interfaces
kube-bridge             8000.4ae815f9f943       no              vethe5393dc7
```

查看网络空间

> docker 环境执行结果
>
> root@node-1:~# ls -l /var/run/netns
> total 0
>
> 是因为docker创建在了这个目录 ls -l /var/run/docker/netns, 如果希望能用ip netns查看可以创建个链接过来
>
> for i in $(ls /var/run/docker/netns); do ln -s /var/run/docker/netns/$i /var/run/netns/$i; done

```bash
root@node3:~# ip netns
cni-c9f73dcd-802a-f008-f0e0-f052494c3d43 (id: 0)
# 连接查看
root@node3:~# ip netns exec cni-c9f73dcd-802a-f008-f0e0-f052494c3d43 ip addr
1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue state UNKNOWN group default qlen 1000
    link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
    inet 127.0.0.1/8 scope host lo
       valid_lft forever preferred_lft forever
    inet6 ::1/128 scope host 
       valid_lft forever preferred_lft forever
2: eth0@if7: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UP group default 
    link/ether e6:c6:dc:64:bf:d9 brd ff:ff:ff:ff:ff:ff link-netnsid 0
    inet 10.233.68.4/24 brd 10.233.68.255 scope global eth0
       valid_lft forever preferred_lft forever
    inet6 fe80::e4c6:dcff:fe64:bfd9/64 scope link 
       valid_lft forever preferred_lft forever
```

