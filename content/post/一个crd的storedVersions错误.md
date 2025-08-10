---
title: "一个crd的storedVersions错误"
date: 2024-09-26T17:20:05+08:00
tags: ["k8s"]
---

哎, 最近水文章也没意思了, 但还是写一个吧, 解决同事遇到的一个错误

```bash
The CustomResourceDefinition "testcrds.example.com" is invalid: status.storedVersions[0]: Invalid value: "v2": must appear in spec.versions
```

### 复现

创建一个crd

```yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: testcrds.example.com
spec:
  group: example.com
  names:
    kind: TestCRD
    plural: testcrds
  scope: Namespaced
  versions:
  - name: v1
    served: true
    storage: false
    schema:
      openAPIV3Schema:
        type: object
        properties:
          spec:
            type: object
            properties:
              foo:
                type: string
  - name: v2
    served: true
    storage: true
    schema:
      openAPIV3Schema:
        type: object
        properties:
          spec:
            type: object
            properties:
              foo:
                type: string
```

```bash
kubectl apply -f c.yaml
```

再次提交一个更新

```yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: testcrds.example.com
spec:
  group: example.com
  names:
    kind: TestCRD
    plural: testcrds
  scope: Namespaced
  versions:
  - name: v3
    served: true
    storage: true
    schema:
      openAPIV3Schema:
        type: object
        properties:
          spec:
            type: object
            properties:
              foo:
                type: string
```

```bash
root@server:~# kubectl apply -f c.yaml 
The CustomResourceDefinition "testcrds.example.com" is invalid: status.storedVersions[0]: Invalid value: "v2": must appear in spec.versions
```

原因, 第一次crd 指定了存储的版本是v2, 再提交一个存储的版本是v3, 就不对了

那如果把v2,v3都设为`storage: true` 呢

> must have exactly one version marked as storage version

报错, 只能有一个版本储存

### 源码分析

k8s的资源都要实现一个更新策略接口 `rest.RESTUpdateStrategy`, 这个接口要实现一些方法, 用于创建/更新等时的验证

```go
type RESTUpdateStrategy interface {
	...
	ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList
	...
}
```

其中这个 `ValidateUpdate` ,就是触发这个错误的位置, 最终调用的函数是`ValidateCustomResourceDefinition `看一下这个方法实现, 代码位置在 `staging/src/k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/validation/validation.go`

<img src="http://inksnw.asuscomm.com:3001/blog/一个crd的storedVersions错误_6b28e29d2649b88dbc7e33223d4bdfff.png" alt="image-20240926174947953" style="zoom:50%;" />

继续看这个 `ValidateCustomResourceDefinitionStoredVersions`函数

<img src="http://inksnw.asuscomm.com:3001/blog/一个crd的storedVersions错误_29c68d3dbfcc65b976343a8a36a9818b.png" alt="image-20240926175336190" style="zoom:50%;" />

大意就是从资源的`Status.StoredVersions` 字段中取出存储的版本列表, 然后和将要提交的比对, 如果也是storage为true并且在旧的里面没找到, 就报错, 大概是这个意思吧, 逻辑比较简单就不单步调试了

### 解决办法

删除旧的crd, 重新提交新的

水完了, 就这样
