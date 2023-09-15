---
title: "K8s运行流程追踪"
date: 2023-09-15T13:42:20+08:00
tags: ["k8s"]
---

k8s从`1.27`  开始 [OpenTelemetry ](https://github.com/open-telemetry/opentelemetry-collector#-opentelemetry-collector) 进入了beta状态, 在此之前要开启还需要配置featureGates



## k8s自带追踪

### 安装jaeger

下载`all in one `包约100M

https://www.jaegertracing.io/download/

```bash
.
├── example-hotrod
├── jaeger-agent
├── jaeger-all-in-one
├── jaeger-collector
├── jaeger-ingester
└── jaeger-query
./jaeger-all-in-one
```

访问

```
http://127.0.0.1:16686/search
```

### 配置apiserver

编辑 `/etc/kubernetes/manifests/kube-apiserver.yaml`

```yaml
- --feature-gates=APIServerTracing=true
- --tracing-config-file=/etc/kubernetes/pki/TracingConfiguration.yaml
```

apiserver会自动重启, 由于`/etc/kubernetes/pki/`会被挂载进容器, 这里就偷懒放到这了

grpc使用的端口是 `4317`

```yaml
apiVersion: apiserver.config.k8s.io/v1alpha1
kind: TracingConfiguration
# 1% sampling rate
samplingRatePerMillion: 10000
endpoint: "10.6.0.2:4317"
```

### 配置kubelet

编辑 `/var/lib/kubelet/config.yaml`, 添加类似的信息, 每台主机都配置后, 重启kubelet

```yaml
apiVersion: kubelet.config.k8s.io/v1beta1
kind: KubeletConfiguration
featureGates:
  KubeletTracing: true
tracing:
  samplingRatePerMillion: 1000000
  endpoint: "10.6.0.2:4317"
```

### 配置containerd

编辑`/etc/containerd/config.toml` , http使用的端口是`4318` , 每台主机都配置后, 重启containerd

```toml
[plugins."io.containerd.tracing.processor.v1.otlp"]
    endpoint = "http://10.6.0.2:4318" 
  [plugins."io.containerd.internal.v1.tracing"]
    sampling_ratio = 1.0
    service_name = "containerd"
```

### 查看信息

```
http://127.0.0.1:16686/search
```

<img src="http://inksnw.asuscomm.com:3001/blog/k8s运行流程追踪_08e48e93a114b7d2efda4c3eb128ddda.png" alt="image-20230915135617342" style="zoom:67%;" />

此时 jaeger 的界面上已可以看到运行的流程, 对jaeger 的ui不太熟悉, 没有找出一个很好的示例, 由于是开发者自己决定在代码的哪些位置添加追踪点, 可能并不能满足使用者都对追踪的需求, 可以试下字节跳动开源的 Kelemetry

## Kelemetry

核心原理是收集event信息, 然后根据元信息组合成一条链路

修改一下配置文件中的  [values.yaml](https://github.com/kubewharf/kelemetry/blob/main/charts/kelemetry/values.yaml)  `storageClassName` 为你实际的,`replicaCount`  为需要的副本数

### 安装

```
helm install kelemetry oci://ghcr.io/kubewharf/kelemetry-chart --values values.yaml
```

查看pod

```
kubectl get pod
NAME                                   READY   STATUS    RESTARTS      AGE
kelemetry-collector-79f879b44d-z5nw5   1/1     Running   2 (64s ago)   77s
kelemetry-consumer-77586595fb-xdlvp    1/1     Running   0             77s
kelemetry-etcd-0                       1/1     Running   0             77s
kelemetry-frontend-547ff76799-lqd94    2/2     Running   5 (40s ago)   77s
kelemetry-informers-745b845dbc-t22c9   1/1     Running   0             77s
kelemetry-storage-0                    1/1     Running   0             77s
```

### 测试

```bash
kubectl create deployment demo --image=httpd --port=80
```

可以看到以Kelemetry 这种方式收集, 可能更适合追踪

![image-20230915144115722](http://inksnw.asuscomm.com:3001/blog/k8s运行流程追踪_fb13bd9e5ad48d7b6dd2d9ca2d637485.png)
