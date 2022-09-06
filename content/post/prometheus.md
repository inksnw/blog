---
title: "Prometheus"
date: 2022-08-24T15:11:03+08:00
tags: ["k8s"]
---

## 安装prometheus operator

```bash
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update
kubectl create ns monitor
helm install myprometheus prometheus-community/kube-prometheus-stack -n monitor
```
报错解决

> plugin type="calico" failed (add): error getting ClusterInformation: connection is unauthorized: Unauthorized

删除 /etc/cni/net.d/下的文件,重启后会重建,暂还不清楚原因

会同时安装以下组件

- [prometheus-community/kube-state-metrics](https://github.com/prometheus-community/helm-charts/tree/main/charts/kube-state-metrics)
- [prometheus-community/prometheus-node-exporter](https://github.com/prometheus-community/helm-charts/tree/main/charts/prometheus-node-exporter)
- [grafana/grafana](https://github.com/grafana/helm-charts/tree/main/charts/grafana)

## 安装prometheus-adapter (可选)

```bash
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update
helm install myadapter prometheus-community/prometheus-adapter -n monitor
```

## 安装metrics-server (可选)

> 报错 **`no metrics known for pod`** `x509: cannot validate certificate`。

给部署添加如下命令行即可：

```bash
args:
  - --kubelet-preferred-address-types=InternalIP
  - --kubelet-insecure-tls
```

```bash
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml
```

