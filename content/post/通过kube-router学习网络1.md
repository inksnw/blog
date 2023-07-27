---
title: "通过kube Router学习网络1"
date: 2023-07-27T10:57:51+08:00
tags: ["cni"]
draft: true
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


# 定义主机名称列表
HOSTS=("node1" "node2" "node3")

# 定义需要在远程主机执行的命令
COMMAND1="ip link delete kube-dummy-if"
COMMAND2="ip link delete kube-bridge"

# 循环遍历每个主机并通过 SSH 执行命令
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

## 网络连通

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

所以我们有了第一个需求, 监听主机的变动去改变路由表

## 主机变动
