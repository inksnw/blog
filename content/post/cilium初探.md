---
title: "Cilium初探"
date: 2023-07-26T10:28:48+08:00
tags: ["Cilium"]
---

## 安装

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

这里使用`kubekey` 这个工具快速安装k8s, 希望手动安装`cilium`, 看了下源码临时把plugin设置了一个值,跳过cni的安装

```bash
helm repo add cilium https://helm.cilium.io/
helm install cilium cilium/cilium --version 1.13.4 --namespace kube-system --set operator.replicas=1 --set ipam.operator.clusterPoolIPv4PodCIDR=10.233.64.0/18 --set hubble.ui.enabled=true  --set hubble.relay.enabled=true
```

查看配置状态

```basic
root@node1:~# kubectl -n kube-system exec ds/cilium -- cilium status
Defaulted container "cilium-agent" out of: cilium-agent, mount-cgroup (init), apply-sysctl-overwrites (init), clean-cilium-state (init)
KVStore:                Ok   Disabled
Kubernetes:             Ok   1.26 (v1.26.5) [linux/amd64]
Kubernetes APIs:        ["cilium/v2::CiliumClusterwideNetworkPolicy", "cilium/v2::CiliumEndpoint", "cilium/v2::CiliumNetworkPolicy", "cilium/v2::CiliumNode", "core/v1::Namespace", "core/v1::Node", "core/v1::Pods", "core/v1::Service", "discovery/v1::EndpointSlice", "networking.k8s.io/v1::NetworkPolicy"]
KubeProxyReplacement:   Disabled   
Host firewall:          Disabled
Cilium:                 Ok   1.11.7 (v1.11.7-506fcf6)
NodeMonitor:            Listening for events on 2 CPUs with 64x4096 of shared memory
Cilium health daemon:   Ok   
IPAM:                   IPv4: 5/254 allocated from 10.233.65.0/24, 
BandwidthManager:       Disabled
Host Routing:           Legacy
Masquerading:           IPTables [IPv4: Enabled, IPv6: Disabled]
Controller Status:      31/31 healthy
Proxy Status:           OK, ip 10.233.65.19, 0 redirects active on ports 10000-20000
Hubble:                 Ok   Current/Max Flows: 4095/4095 (100.00%), Flows/s: 4.13   Metrics: Disabled
Encryption:             Disabled
Cluster health:         3/3 reachable   (2023-07-26T02:29:36Z)
```

### 安装 Cilium CLI

```bash

CILIUM_CLI_VERSION=$(curl -s https://raw.githubusercontent.com/cilium/cilium-cli/master/stable.txt)
CLI_ARCH=amd64
if [ "$(uname -m)" = "aarch64" ]; then CLI_ARCH=arm64; fi
curl -L --fail --remote-name-all https://github.com/cilium/cilium-cli/releases/download/${CILIUM_CLI_VERSION}/cilium-linux-${CLI_ARCH}.tar.gz{,.sha256sum}
sha256sum --check cilium-linux-${CLI_ARCH}.tar.gz.sha256sum
sudo tar xzvfC cilium-linux-${CLI_ARCH}.tar.gz /usr/local/bin
rm cilium-linux-${CLI_ARCH}.tar.gz{,.sha256sum}
```

查看信息

```basic
root@node1:~# cilium version
cilium-cli: v0.15.3 compiled with go1.20.4 on linux/amd64
cilium image (default): v1.13.4
cilium image (stable): v1.13.4
cilium image (running): 1.13.4
root@node1:~# cilium status
    /¯¯\
 /¯¯\__/¯¯\    Cilium:             OK
 \__/¯¯\__/    Operator:           OK
 /¯¯\__/¯¯\    Envoy DaemonSet:    disabled (using embedded mode)
 \__/¯¯\__/    Hubble Relay:       OK
    \__/       ClusterMesh:        disabled

Deployment             cilium-operator    Desired: 1, Ready: 1/1, Available: 1/1
DaemonSet              cilium             Desired: 3, Ready: 3/3, Available: 3/3
Deployment             hubble-relay       Desired: 1, Ready: 1/1, Available: 1/1
Deployment             hubble-ui          Desired: 1, Ready: 1/1, Available: 1/1
Containers:            cilium             Running: 3
                       cilium-operator    Running: 1
                       hubble-relay       Running: 1
                       hubble-ui          Running: 1
Cluster Pods:          5/5 managed by Cilium
Helm chart version:    1.13.4
```

## 

Cilium 会自动以封装(隧道 tunnel)模式运行，因为这种模式**对底层网络基础设施的要求最低**。

在这种模式下，所有集群节点都会使用基于 UDP 的封装协议 VXLAN 或 Geneve 形成网状隧道。

由于增加了封装头，有效载荷可用的 MTU 要低于本地路由, 这导致特定网络连接的最大吞吐率降低。
