---
title: "Cilium初探"
date: 2023-07-26T10:28:48+08:00
tags: ["ebpf"]
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

安装 Cilium CLI, [官方文档](https://docs.cilium.io/en/stable/gettingstarted/k8s-install-default/#k8s-install-quick)

```bash
CILIUM_CLI_VERSION=$(curl -s https://raw.githubusercontent.com/cilium/cilium-cli/main/stable.txt)
CLI_ARCH=amd64
if [ "$(uname -m)" = "aarch64" ]; then CLI_ARCH=arm64; fi
curl -L --fail --remote-name-all https://github.com/cilium/cilium-cli/releases/download/${CILIUM_CLI_VERSION}/cilium-linux-${CLI_ARCH}.tar.gz{,.sha256sum}
sha256sum --check cilium-linux-${CLI_ARCH}.tar.gz.sha256sum
sudo tar xzvfC cilium-linux-${CLI_ARCH}.tar.gz /usr/local/bin
rm cilium-linux-${CLI_ARCH}.tar.gz{,.sha256sum}

```

安装

```bash
➜ cilium install
```

查看配置状态

```basic
➜ kubectl -n kube-system exec ds/cilium -- cilium status
Defaulted container "cilium-agent" out of: cilium-agent, config (init), mount-cgroup (init), apply-sysctl-overwrites (init), mount-bpf-fs (init), clean-cilium-state (init), install-cni-binaries (init)
KVStore:                 Ok   Disabled
Kubernetes:              Ok   1.26 (v1.26.5) [linux/amd64]
Kubernetes APIs:         ["cilium/v2::CiliumClusterwideNetworkPolicy", "cilium/v2::CiliumEndpoint", "cilium/v2::CiliumNetworkPolicy", "cilium/v2::CiliumNode", "core/v1::Namespace", "core/v1::Node", "core/v1::Pods", "core/v1::Service", "discovery/v1::EndpointSlice", "networking.k8s.io/v1::NetworkPolicy"]
KubeProxyReplacement:    Strict   [enp1s0 192.168.50.52]
Host firewall:           Disabled
CNI Chaining:            none
CNI Config file:         CNI configuration file management disabled
Cilium:                  Ok   1.13.4 (v1.13.4-4061cdfc)
NodeMonitor:             Listening for events on 4 CPUs with 64x4096 of shared memory
Cilium health daemon:    Ok   
IPAM:                    IPv4: 2/254 allocated from 10.0.0.0/24, 
IPv6 BIG TCP:            Disabled
BandwidthManager:        Disabled
Host Routing:            Legacy
Masquerading:            IPTables [IPv4: Enabled, IPv6: Disabled]
Controller Status:       18/18 healthy
Proxy Status:            OK, ip 10.0.0.87, 0 redirects active on ports 10000-20000
Global Identity Range:   min 256, max 65535
Hubble:                  Ok   Current/Max Flows: 263/4095 (6.42%), Flows/s: 0.86   Metrics: Disabled
Encryption:              Disabled
Cluster health:          3/3 reachable   (2023-08-03T07:23:39Z)
```

查看信息

```bash
➜ cilium status
    /¯¯\
 /¯¯\__/¯¯\    Cilium:             OK
 \__/¯¯\__/    Operator:           OK
 /¯¯\__/¯¯\    Envoy DaemonSet:    disabled (using embedded mode)
 \__/¯¯\__/    Hubble Relay:       disabled
    \__/       ClusterMesh:        disabled

DaemonSet              cilium             Desired: 3, Ready: 3/3, Available: 3/3
Deployment             cilium-operator    Desired: 1, Ready: 1/1, Available: 1/1
Containers:            cilium-operator    Running: 1
                       cilium             Running: 3
Cluster Pods:          3/3 managed by Cilium
Helm chart version:    1.13.4
Image versions         cilium-operator    quay.io/cilium/operator-generic:v1.13.4@sha256:xxx: 1
                       cilium             quay.io/cilium/cilium:v1.13.4@sha256:xxx: 3
```

