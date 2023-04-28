---
title: "Clusterpedia使用"
date: 2023-03-28T14:56:53+08:00
tags: ["k8s"]
---

Clusterpedia在兼容Kubernetes OpenAPI的基础上，可以实现多集群资源同步，并提供更强大的搜索功能，帮助您高效快捷地获取任何您正在寻找的多集群资源。

### 安装

[官方文档](https://github.com/clusterpedia-io/clusterpedia-helm/tree/main/charts/clusterpedia)

```
$ helm repo add clusterpedia https://clusterpedia-io.github.io/clusterpedia-helm/
$ helm repo list
NAME          	URL
clusterpedia  	https://clusterpedia-io.github.io/clusterpedia-helm/
```

```
helm install clusterpedia clusterpedia/clusterpedia \
--namespace clusterpedia-system \
--create-namespace \
--set persistenceMatchNode=None \
--set installCRDs=true
```

### 加入集群

首先需要将接入集群的 kube config base64 编码。

```bash
base64 -w 0 ~/.kube/config
```

将生成的编码内容,填入到下方的`spec.kubeconfig`中

`syncResources` 字段为要同步的资源

```yaml
apiVersion: cluster.clusterpedia.io/v1alpha2
kind: PediaCluster
metadata:
  name: cluster-example
spec:
  apiserver:
  kubeconfig: xxxxxxxxx==
  caData:
  tokenData:
  certData:
  keyData:
  syncResources:
  - group: apps
    resources:
     - deployments
  - group: ""
    resources:
     - pods
     - configmaps
  - group: cert-manager.io
    versions:
      - v1
    resources:
      - certificates
```

同步所有资源

```yaml
spec:
  syncResources:
  - group: "*"
    resources:
    - "*"
```

查看集群情况

```bash
root@node1:~# kubectl get pediacluster
NAME              READY   VERSION   APISERVER
cluster-example   True    v1.25.3   https://192.168.50.50:6443
```

### 配置kubectl

```bash
curl -sfL https://raw.githubusercontent.com/clusterpedia-io/clusterpedia/main/hack/gen-clusterconfigs.sh | bash -
```

查看config文件

```yaml
apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: xxx
    server: https://192.168.50.50:6443/apis/clusterpedia.io/v1beta1/resources/clusters/cluster-example
  name: cluster-example
- cluster:
    certificate-authority-data: xxx
    server: https://192.168.50.50:6443
  name: cluster.local
- cluster:
    certificate-authority-data: xxx
    server: https://192.168.50.50:6443/apis/clusterpedia.io/v1beta1/resources
  name: clusterpedia
contexts:
- context:
    cluster: clusterpedia
    user: kubernetes-admin
  name: kubernetes-admin@cluster.local
current-context: kubernetes-admin@cluster.local
kind: Config
preferences: {}
users:
- name: kubernetes-admin
  user:
    client-certificate-data: xxx
    client-key-data: xxx
```

可以发现clusterpedia将请求转发到了他内部的api上

查询测试

```bash
kubectl --cluster clusterpedia get pod -A
NAMESPACE             CLUSTER           NAME                                                   READY   STATUS    RESTARTS   AGE
clusterpedia-system   cluster-example   clusterpedia-apiserver-d4d9c57fb-8gvbt                 1/1     Running   0          123m
clusterpedia-system   cluster-example   clusterpedia-clustersynchro-manager-7799678fc6-mnxht   1/1     Running   0          123m
...
```

### 实现分析

查看创建的pod

```bash
root@node1:~# kubectl get pod -n clusterpedia-system
NAME                                                   READY   STATUS    RESTARTS   AGE
clusterpedia-apiserver-d4d9c57fb-8gvbt                 1/1     Running   0          124m
clusterpedia-clustersynchro-manager-7799678fc6-mnxht   1/1     Running   0          124m
clusterpedia-controller-manager-56bbcc7478-62qw2       1/1     Running   0          124m
clusterpedia-postgresql-0                              1/1     Running   0          124m
```

数据库的密码在secret中

```bash
kubectl get secret clusterpedia-postgresql -n clusterpedia-system  -o yaml
```

以下是推测的`clusterpedia`技术实现

- `clusterpedia-clustersynchro-manager`通过`informer`把要同步的资源写入数据库中
- 当用户通过`--cluster clusterpedia`参数查询时, kubectl 以为这是另一个集群上下文, 而将请求发送到`clusterpedia-apiserver`, `clusterpedia-apiserver` 则会去数据库中查询数据, 将结果附加上集群字段返回
- `clusterpedia-controller-manager`则负责一些集群的发现工作等

### 使用存储插件

clusterpedia通过`Go Plugin` 的方式支持了存储插件, 我们卸载掉普通版本,准备安装`clusterpedia-core`版本

#### 安装`clusterpedia-core`

```
helm search repo clusterpedia
NAME                            CHART VERSION   APP VERSION     DESCRIPTION                  
clusterpedia/clusterpedia       1.5.0           v0.6.3          A Helm chart for Kubernetes  
clusterpedia/clusterpedia-core  0.1.1           v0.6.0          A Helm chart for Clusterpedia
clusterpedia/clusterpedia-mysql 0.1.1           v0.6.0          A Helm chart for Clusterpedia
```

修改values.yaml

```bash
helm pull clusterpedia/clusterpedia-core
# 在values.yaml 中修改以下内容
```

```yaml
storage:
  name: "arango-storage-layer"
  image:
    registry: docker.io
    repository: zichenkkkk/arango-storage-layer
    tag: v0.0.1
  config:
    type: "arango"
    host: "192.168.50.54"
    port: 8529
    database: clusterpedia
    pullPolicy: IfNotPresent
```

安装

```bash
helm install clusterpedia clusterpedia/clusterpedia-core \
--namespace clusterpedia-system \
--create-namespace \
--set persistenceMatchNode=None \
--set installCRDs=true -f values.yaml
```

修改同步程序参数

在上文中, 我们说到配置`syncResources`可以通过通配符的的方式同步所有资源, 配置后同步程序需要添加功能

```bash
# 添加启动参数 --feature-gates=AllowSyncAllResources=true
kubectl edit deploy clusterpedia-clusterpedia-core-controller-manager -n clusterpedia-system
```

但是执行后启动报错, 提示没有这个功能, 推测`clusterpedia-core`的版本较低,还没有这个功能, 所以此步跳过

开启同步资源

```yaml
apiVersion: cluster.clusterpedia.io/v1alpha2
kind: PediaCluster
metadata:
  name: cluster-example
spec:
  apiserver:
  kubeconfig: xxxxxx
  caData:
  tokenData:
  certData:
  keyData:
  syncResources:
  - group: apps
    resources:
     - deployments
  - group: ""
    resources:
     - pods
     - configmaps
  - group: cert-manager.io
    versions:
      - v1
    resources:
      - certificates
```

执行查询

```bash
kubectl --cluster clusterpedia get pod  --insecure-skip-tls-verify=true  
CLUSTER           NAME                      READY   STATUS    RESTARTS        AGE
cluster-example   nginx6-848c7fc44d-ffnt8   1/1     Running   1 (7h42m ago)   16h
```



