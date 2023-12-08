---
title: "Cilium本地路由"
date: 2023-12-07T23:11:38+08:00
tags: ["ebpf"]
---

## 安装

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

重启所有已运行但未以主机联网模式运行的 pod，以确保 Cilium 开始管理它们. 

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

```bash
# 在节点1上执行
iperf3 -s
# 在节点2上执行, 以Mbps为单位
iperf3 -c 192.168.50.50 -f M
- - - - - - - - - - - - - - - - - - - - - - - - -
[ ID] Interval           Transfer     Bitrate         Retr
[  5]   0.00-10.00  sec  38.4 GBytes  3936 MBytes/sec    0             sender
[  5]   0.00-10.04  sec  38.4 GBytes  3920 MBytes/sec                  receiver
```

大约 `3920` MBytes/sec   

### 跨vm主机pod测速

```bash
kubectl run tmp-shell1 --rm -i --tty --image nicolaka/netshoot --overrides='{"spec":{"nodeName":"node1"}}'
kubectl run tmp-shell2 --rm -i --tty --image nicolaka/netshoot --overrides='{"spec":{"nodeName":"node2"}}'
root@node1:~#  kubectl get pod -o wide
NAME         READY   STATUS    RESTARTS   AGE     IP           NODE    NOMINATED NODE   READINESS GATES
tmp-shell1   1/1     Running   0          2m22s   10.0.1.89    node1   <none>           <none>
tmp-shell2   1/1     Running   0          41s     10.0.2.249   node2   <none>           <none>
# 在node1上的pod执行
iperf3 -s
# 在node2上的pod执行
iperf3 -c 10.0.1.89 -f M

- - - - - - - - - - - - - - - - - - - - - - - - -
[ ID] Interval           Transfer     Bitrate         Retr
[  5]   0.00-10.00  sec  8.22 GBytes   841 MBytes/sec  632             sender
[  5]   0.00-10.00  sec  8.21 GBytes   841 MBytes/sec                  receiver
```

从 3920 到 800, 损失相当多啊

## 启用本地路由

```bash
helm upgrade cilium cilium/cilium \
   --namespace kube-system \
   --reuse-values \
   --set tunnel=disabled \
   --set autoDirectNodeRoutes=true \
   --set ipv4NativeRoutingCIDR=10.0.0.0/22
```

配置说明如下:

- `tunnel=disabled` 启用本地路由模式
- `autoDirectNodeRoutes=true` 每个节点都知道所有其他节点的所有 pod IP，并在 **Linux 内核路由表**中插入路由来表示。如果所有节点**共享一个 L2 网络**，则可以启用选项 `auto-direct-node-routes: true` 来解决这个问题。
- `ipv4-native-routing-cidr: x.x.x.x/y` 设置可执行本地路由的 CIDR。

重启触发配置

```
kubectl rollout restart daemonset cilium -n kube-system
```

### 测速

```bash
kubectl run tmp-shell1 --rm -i --tty --image nicolaka/netshoot --overrides='{"spec":{"nodeName":"node1"}}'
kubectl run tmp-shell2 --rm -i --tty --image nicolaka/netshoot --overrides='{"spec":{"nodeName":"node2"}}'
root@node1:~# kubectl get pod -o wide
NAME         READY   STATUS    RESTARTS   AGE   IP          NODE    NOMINATED NODE   READINESS GATES
tmp-shell1   1/1     Running   0          22s     10.0.2.141   node1   <none>           <none>
tmp-shell2   1/1     Running   0          13s     10.0.0.105   node2   <none>           <none>
iperf3 -s
iperf3 -c 10.0.2.141 -f M
- - - - - - - - - - - - - - - - - - - - - - - - -
[ ID] Interval           Transfer     Bitrate         Retr
[  5]   0.00-10.00  sec  38.0 GBytes  3895 MBytes/sec  395             sender
[  5]   0.00-10.00  sec  38.0 GBytes  3895 MBytes/sec                  receiver
```

可以看到在本地路由模式下, 基本没有损失

参考文档: [Cilium系列-14-Cilium NetworkPolicy 简介](https://mp.weixin.qq.com/s/nV0rT14D5WG8h71KjsxXMQ)
