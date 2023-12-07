---
title: "Cilium本地路由"
date: 2023-12-07T23:11:38+08:00
draft: true
---

使用 Helm Chart 进行基本安装

```bash
helm repo add cilium https://helm.cilium.io/

helm install cilium cilium/cilium --version 1.14.4 \
   --namespace kube-system \
   --set operator.replicas=1 \
   --set k8sServiceHost=192.168.50.50 \
   --set k8sServicePort=6443 \
   --set hubble.relay.enabled=true \
   --set hubble.ui.enabled=true
```

如果你创建的集群中没有使用 `node.cilium.io/agent-not-ready` 污点的节点，则需要手动重启未托管的 pod。重启所有已运行但未以主机联网模式运行的 pod，以确保 Cilium 开始管理它们。这样做是为了确保所有在部署 Cilium 之前已经运行的 pod 都具有 Cilium 提供的网络连接，并且 NetworkPolicy 也适用于它们：

```bash
kubectl get pods -A -o custom-columns=NAMESPACE:.metadata.namespace,NAME:.metadata.name,HOSTNETWORK:.spec.hostNetwork --no-headers=true | grep '<none>' | awk '{print "-n "$1" "$2}' | xargs -L 1 -r kubectl delete pod
pod "coredns-7f647946c8-922vs" deleted
pod "coredns-7f647946c8-c7mhh" deleted
pod "hubble-relay-7b459f4fd8-4f2w6" deleted
pod "hubble-ui-86d4df48b9-h4xtg" deleted
pod "openebs-localpv-provisioner-7cc4c84b9-rjczh" deleted
```



## 性能测试

### VM 间宽带

> `-f M`: 这是用于指定iperf3报告中的带宽单位的标志。在这里，`M` 表示带宽以兆比特每秒（Mbps）为单位进行显示

```bash
iperf3 -s
iperf3 -c 192.168.50.51 -f M
Connecting to host 192.168.50.51, port 5201
[  5] local 192.168.50.50 port 47240 connected to 192.168.50.51 port 5201
[ ID] Interval           Transfer     Bitrate         Retr  Cwnd
[  5]   0.00-1.00   sec  3.94 GBytes  4037 MBytes/sec    0   3.13 MBytes       
[  5]   1.00-2.00   sec  4.05 GBytes  4143 MBytes/sec    0   3.13 MBytes       
[  5]   2.00-3.00   sec  4.15 GBytes  4247 MBytes/sec    0   3.13 MBytes       
...     
- - - - - - - - - - - - - - - - - - - - - - - - -
[ ID] Interval           Transfer     Bitrate         Retr
[  5]   0.00-10.00  sec  41.3 GBytes  4225 MBytes/sec    0             sender
[  5]   0.00-10.04  sec  41.3 GBytes  4209 MBytes/sec                  receiver
```

大约 4209 MBytes/sec   

### 跨vm主机pod测速

```bash
kubectl run tmp-shell1 --rm -i --tty --image nicolaka/netshoot --overrides='{"spec":{"nodeName":"node1"}}'
kubectl run tmp-shell2 --rm -i --tty --image nicolaka/netshoot --overrides='{"spec":{"nodeName":"node2"}}'
root@node1:~# kubectl get pod -o wide
NAME         READY   STATUS    RESTARTS   AGE   IP          NODE    NOMINATED NODE   READINESS GATES
tmp-shell1   1/1     Running   0          64s   10.0.2.97   node1   <none>           <none>
tmp-shell2   1/1     Running   0          6s    10.0.0.24   node2   <none>           <none>
iperf3 -s
iperf3 -c 10.0.2.97 -f M

Connecting to host 10.0.2.97, port 5201
[  5] local 10.0.0.24 port 53388 connected to 10.0.2.97 port 5201
[ ID] Interval           Transfer     Bitrate         Retr  Cwnd
[  5]   0.00-1.00   sec   833 MBytes   833 MBytes/sec  180   1.52 MBytes       
[  5]   1.00-2.00   sec   864 MBytes   864 MBytes/sec  236   1.61 MBytes       
[  5]   2.00-3.00   sec   851 MBytes   851 MBytes/sec   34   1.64 MBytes       
...   
- - - - - - - - - - - - - - - - - - - - - - - - -
[ ID] Interval           Transfer     Bitrate         Retr
[  5]   0.00-10.00  sec  7.98 GBytes   817 MBytes/sec  951             sender
[  5]   0.00-10.00  sec  7.97 GBytes   816 MBytes/sec                  receiver
```

从4000到800, 损失相当多啊

### Helm Chart 启用本地路由

```

helm upgrade cilium cilium/cilium \
   --namespace kube-system \
   --reuse-values \
   --set tunnel=disabled \
   --set autoDirectNodeRoutes=true \
   --set ipv4NativeRoutingCIDR=10.0.0.0/22
```

配置说明如下:

- `--reuse-values` 复用上一次的 Helm Chart 安装配置

- `tunnel=disabled` 启用本地路由模式
- `autoDirectNodeRoutes=true` 每个节点都知道所有其他节点的所有 pod IP，并在 **Linux 内核路由表**中插入路由来表示。如果所有节点**共享一个 L2 网络**，则可以启用选项 `auto-direct-node-routes: true` 来解决这个问题。
- `ipv4-native-routing-cidr: x.x.x.x/y` 设置可执行本地路由的 CIDR。
## 验证本地路由是否启用

未启用之前, 也就是通过 VXLan 封装时, 会有一个对应的 VXLan 网卡 `cilium_vxlan`

```
56: cilium_vxlan: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UNKNOWN group default qlen 1000
    link/ether d2:f0:55:2a:72:c4 brd ff:ff:ff:ff:ff:ff
    inet6 fe80::d0f0:55ff:fe2a:72c4/64 scope link 
       valid_lft forever preferred_lft forever
```

##### 方式一：

从集群中每个节点获取 pod CIDR 地址。

```
kubectl get nodes -o jsonpath='{.items[*].spec.podCIDR}'
```

##### 方式二：

kube-proxy所使用的 pod网络CIDR。

```
kubectl cluster-info dump | grep -m 1 cluster-cidr
```

测速

```
kubectl run tmp-shell1 --rm -i --tty --image nicolaka/netshoot --overrides='{"spec":{"nodeName":"node1"}}'
kubectl run tmp-shell2 --rm -i --tty --image nicolaka/netshoot --overrides='{"spec":{"nodeName":"node2"}}'
root@node1:~# kubectl get pod -o wide
NAME         READY   STATUS    RESTARTS   AGE   IP          NODE    NOMINATED NODE   READINESS GATES
tmp-shell1   1/1     Running   0          22s     10.0.2.141   node1   <none>           <none>
tmp-shell2   1/1     Running   0          13s     10.0.0.105   node2   <none>           <none>
iperf3 -s
iperf3 -c 10.0.2.141 -f M
```

