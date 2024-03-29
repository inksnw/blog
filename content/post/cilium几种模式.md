---
title: "Cilium(三)几种模式"
date: 2024-01-15T21:43:59+08:00
draft: true
---

## kubeproxy

### direct-routing

```bash
# 3.install cni[Cilium]
helm repo add cilium https://helm.cilium.io 
helm repo update 

helm install cilium cilium/cilium  --version 1.14.5 --namespace kube-system --set debug.enabled=true --set debug.verbose=datapath --set monitorAggregation=none --set ipam.mode=cluster-pool  --set tunnel=disabled --set autoDirectNodeRoutes=true --set ipv4NativeRoutingCIDR="10.0.0.0/8"
```

### vxlan

```bash
# VxLAN Options(default mode)
helm install cilium cilium/cilium  --version 1.14.5 --namespace kube-system --set debug.enabled=true --set debug.verbose=datapath --set monitorAggregation=none --set ipam.mode=cluster-pool 
```

## kubeproxy-replacement

### direct-routing

```bash
helm install cilium cilium/cilium  --version 1.14.5 --namespace kube-system --set debug.enabled=true --set debug.verbose=datapath --set monitorAggregation=none --set ipam.mode=cluster-pool  --set kubeProxyReplacement=true --set tunnel=disabled --set autoDirectNodeRoutes=true --set ipv4NativeRoutingCIDR="10.0.0.0/8"
```

### vxlan

```bash
helm install cilium cilium/cilium  --version 1.14.5 --namespace kube-system --set debug.enabled=true --set debug.verbose=datapath --set monitorAggregation=none --set ipam.mode=cluster-pool  --set kubeProxyReplacement=true
```

## kubeproxy-replacement-ebpf

### direct-routing

```bash
helm install cilium cilium/cilium  --version 1.14.5 --namespace kube-system --set debug.enabled=true --set debug.verbose=datapath --set monitorAggregation=none --set ipam.mode=cluster-pool  --set kubeProxyReplacement=strict --set tunnel=disabled --set autoDirectNodeRoutes=true --set ipv4NativeRoutingCIDR="10.0.0.0/8" --set bpf.masquerade=true 
```

### vxlan

```bash
helm install cilium cilium/cilium  --version 1.14.5 --namespace kube-system --set debug.enabled=true --set debug.verbose=datapath --set monitorAggregation=none --set ipam.mode=cluster-pool  --set kubeProxyReplacement=strict --set bpf.masquerade=true 
```

