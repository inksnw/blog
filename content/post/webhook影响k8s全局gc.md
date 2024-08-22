---
title: "Webhook影响k8s全局gc"
date: 2023-11-21T10:31:12+08:00
tags: ["k8s"]
---

源于一次环境异常, 发现删除deploy时, pod不会被自动清除

## 复现

提交一个带`Webhook` 的crd

```bash
kubectl apply -f - <<EOF
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: examples.example.com
spec:
  conversion:
    strategy: Webhook
    webhook:
      clientConfig:
        caBundle: ZmFrZQ==
        service:
          name: mysql-webhook
          namespace: default
          path: /convert
          port: 443
      conversionReviewVersions:
      - v1
  group: example.com
  versions:
  - name: v1
    served: true
    storage: true
    schema:
      openAPIV3Schema:
        type: object
        properties:
          spec:
            type: object
            properties:
              field1:
                type: string
              field2:
                type: string
  scope: Namespaced
  names:
    plural: examples
    singular: example
    kind: Example
EOF
```

验证gc

```bash
kubectl create deployment my-deployment --image=nginx
kubectl get pod 
kubectl delete deploy my-deployment
# 发现还是存在
kubectl get pod
```

查看kube-controller的日志会发现

<img src="https://inksnw.asuscomm.com:3001/blog/webhook影响k8s全局gc_7716fbb60d4999a9a203c277c9d34364.png" alt="image-20231121155432905" style="zoom:50%;" />

## 原因追踪

逻辑在`pkg/controller/garbagecollector/garbagecollector.go` 183行

gc会收集所有可删除的资源列表生成 `newResources`

<img src="https://inksnw.asuscomm.com:3001/blog/webhook影响k8s全局gc_d3f59a700dbbb28410a9f431edd093a3.png" alt="image-20231121154832747" style="zoom:50%;" />

为所有可删除的资源创建informer

<img src="https://inksnw.asuscomm.com:3001/blog/webhook影响k8s全局gc_58e3285164444b1947f774b7d938d89f.png" alt="image-20231121155058272" style="zoom:50%;" />

核心原因就在这里

<img src="https://inksnw.asuscomm.com:3001/blog/webhook影响k8s全局gc_999ae8cda743b50757b01fe654de56a6.png" alt="image-20231121155142767" style="zoom:50%;" />

k8s要求所有可删除的资源的informer都同步完成才行, 而informer会调用list方法, 但是由于我们的crd使用的webhook还未安装/运行错误, 因此这个同步一直无法完成, 所以也影响到了其它资源的gc, 感觉这块的实现并不是很合理

官方issue中也有提到这个, 但目前来看, 还未修复 

https://github.com/kubernetes/kubernetes/issues/110720
