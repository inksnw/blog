---
title: "Cilium(一)安装"
date: 2023-07-26T10:28:48+08:00
tags: ["ebpf","cni"]
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

这里使用`kubekey` 这个工具快速安装k8s, 默认安装的是calico, 由于希望手动安装 `cilium`, 看了下源码临时把plugin设置了一个值,跳过cni的安装

```bash
➜ echo "yes" | kk create cluster -f config-simple.yaml --with-local-storage --skip-pull-images
```

安装  [官方文档](https://docs.cilium.io/en/stable/installation/k8s-install-helm/)

```bash
helm repo add cilium https://helm.cilium.io/
helm install cilium cilium/cilium --version 1.14.5  --namespace kube-system
```

查看配置状态

```basic
➜ kubectl -n kube-system exec ds/cilium -- cilium status
Defaulted container "cilium-agent" out of: cilium-agent, config (init), mount-cgroup (init), apply-sysctl-overwrites (init), mount-bpf-fs (init), clean-cilium-state (init), install-cni-binaries (init)
KVStore:                 Ok   Disabled
Kubernetes:              Ok   1.26 (v1.26.5) [linux/amd64]
Kubernetes APIs:         ["EndpointSliceOrEndpoint", "cilium/v2::CiliumClusterwideNetworkPolicy", "cilium/v2::CiliumEndpoint", "cilium/v2::CiliumNetworkPolicy", "cilium/v2::CiliumNode", "cilium/v2alpha1::CiliumCIDRGroup", "core/v1::Namespace", "core/v1::Pods", "core/v1::Service", "networking.k8s.io/v1::NetworkPolicy"]
KubeProxyReplacement:    False   [enp1s0  (Direct Routing)]
Host firewall:           Disabled
CNI Chaining:            none
Cilium:                  Ok   1.14.5 (v1.14.5-85db28be)
NodeMonitor:             Listening for events on 4 CPUs with 64x4096 of shared memory
Cilium health daemon:    Ok   
IPAM:                    IPv4: 4/254 allocated from 10.0.0.0/24, 
IPv4 BIG TCP:            Disabled
IPv6 BIG TCP:            Disabled
BandwidthManager:        Disabled
Host Routing:            Legacy
Masquerading:            IPTables [IPv4: Enabled, IPv6: Disabled]
Controller Status:       30/30 healthy
Proxy Status:            OK, ip 10.0.0.20, 0 redirects active on ports 10000-20000, Envoy: embedded
Global Identity Range:   min 256, max 65535
Hubble:                  Ok               Current/Max Flows: 140/4095 (3.42%), Flows/s: 3.60   Metrics: Disabled
Encryption:              Disabled         
Cluster health:          0/1 reachable    (2024-01-15T13:27:46Z)
  Name                   IP               Node        Endpoints
  m1 (localhost)         192.168.50.218   reachable   unreachable
```

## 重启未纳管pod

```bash
kubectl get pods --all-namespaces -o custom-columns=NAMESPACE:.metadata.namespace,NAME:.metadata.name,HOSTNETWORK:.spec.hostNetwork --no-headers=true | grep '<none>' | awk '{print "-n "$1" "$2}' | xargs -L 1 -r kubectl delete pod
```

## 开启Cilium Hubble

```bash
helm upgrade cilium cilium/cilium --version 1.14.5 \
   --namespace kube-system \
   --reuse-values \
   --set hubble.relay.enabled=true \
   --set hubble.ui.enabled=true
# 改为nodeport
kubectl edit svc hubble-ui -n kube-system
```

<img src="https://inksnw.asuscomm.com:3001/blog/cilium初探_95fc20d1b4f6747a098d6361cd489828.png" alt="image-20231204232622229" style="zoom:50%;" />
