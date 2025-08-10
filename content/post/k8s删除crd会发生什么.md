---
title: "K8s删除crd会发生什么"
date: 2023-06-29T13:58:37+08:00
tags: ["k8s"]
---

### 源起

源于一次环境问题排查,在创建cr时, 提示

```
 (MethodNotAllowed): error when creating "cr1.yaml": create not allowed while custom resource definition is terminating
```

先说结论, 这是由于crd处在预删除状态, 因此无法创建cr, 操作删除并重新安装crd后可修复 

那么删除crd的流程是什么样的, k8s会自删除相关cr么

### 模拟全流程, 创建一个crd

```yaml
kind: CustomResourceDefinition
apiVersion: apiextensions.k8s.io/v1
metadata:
  name: crontabs.stable.example.com
spec:
  group: stable.example.com
  versions:
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          #不限定内容, 可设置任意值
          x-kubernetes-preserve-unknown-fields: true

  scope: Namespaced
  names:
    plural: crontabs
    singular: crontab
    kind: CronTab
```

### 创建一个cr

```bash
apiVersion: "stable.example.com/v1"
kind: CronTab
metadata:
  name: my-new-cron-object
spec:
  cronSpec: "* * * * */5"
```

### 查看etcd数据

使用工具`https://github.com/inksnw/etcdviewer`查看etcd中的信息

可以看到crd存储于`/registry/apiextensions.k8s.io/customresourcedefinitions/crontabs.stable.example.com`

<img src="http://inksnw.asuscomm.com:3001/blog/k8s删除crd会发生什么_2ce0d3a45a47e6ce641edf35623906e8.png" alt="image-20230629141221544" style="zoom:67%;" />

cr存储于`/registry/stable.example.com/crontabs/default/my-new-cron-object`

<img src="http://inksnw.asuscomm.com:3001/blog/k8s删除crd会发生什么_1d72f5c14a77fb775722bd1ebaf74c0d.png" alt="image-20230629141321816" style="zoom:67%;" />

### 删除crd

```
 kubectl delete -f crd.yaml 
```

再次查看etcd中的数据, 会看到`crd`与`cr`都已消失,看来k8s会自动删除相关的cr

执行`kubectl get crontabs` 会提示 Unable to list "stable.example.com/v1, Resource=crontabs", 推测此时apiserver里还有crd的缓存信息, 稍后再执行就提示error: the server doesn't have a resource type "crontabs"

### 验证

重新创建crd与cr, 再手动给cr打上`finalizers`标记

```bash
kubectl apply -f crd.yaml 
kubectl apply -f cr.yaml 
kubectl patch crontab my-new-cron-object -p '{"metadata":{"finalizers":["abc"]}}' --type=merge
```

再次删除crd, 删除操作会卡住, 查看发现crd处于预删除状态

```bash
kubectl delete -f crd.yaml
# 查看crd信息
kubectl get crd crontabs.stable.example.com -o jsonpath='{.metadata.deletionTimestamp}'
2023-06-29T06:19:21Z
# k8s会未crd打上 customresourcecleanup.apiextensions.k8s.io 的finalizers标记
kubectl get crd crontabs.stable.example.com -o jsonpath='{.metadata.finalizers}'
["customresourcecleanup.apiextensions.k8s.io"]
```

由于相关cr未能成功删除, crd处于预删除状态, 此时创建一个新的cr会报错

```bash
kubectl apply -f cr1.yaml 
Error from server (MethodNotAllowed): error when creating "cr1.yaml": create not allowed while custom resource definition is terminating
```

移除cr的`finalizers`标记

```bash
kubectl patch crontab my-new-cron-object -p '{"metadata":{"finalizers":null}}' --type=merge
```

查看crd, 发现已被删除

```bash
kubectl get crd|grep crontab
```

### 破坏操作

如果我手动把crd上的`finalizers`标记移除实现删除, 那之前的cr会怎么样,

```bash
# 创建crd
kubectl apply -f crd.yaml
# 创建cr
kubectl apply -f cr.yaml
# 标记cr
kubectl patch crontab my-new-cron-object -p '{"metadata":{"finalizers":["abc"]}}' --type=merge
# 删除crd
kubectl delete -f crd.yaml
# 移除crd的finalizers
kubectl patch crd crontabs.stable.example.com -p '{"metadata":{"finalizers":null}}' --type=merge
# crd已被删除
kubectl get crd|grep crontab
```

这里再查看etcd中的数据, crd已经消失, cr还存储于etcd中, 这个情况k8s会如何处理这个数据, 不知道会不会永久成为垃圾数据

<img src="http://inksnw.asuscomm.com:3001/blog/k8s删除crd会发生什么_dbf2638650507e005821955a15a7a1cf.png" alt="image-20230629143450608" style="zoom:67%;" />

再次创建回crd, 这个cr 居然还可以再次查到, 好吧, 看来crd与cr之间并无特定惟一标识对应

```bash
kubectl apply -f crd.yaml
kubectl get crontabs
NAME                 AGE
my-new-cron-object   6m4s
```

