---
title: "K8s更新crd会发生什么"
date: 2023-07-28T14:43:36+08:00
tags: ["k8s"]
---

K8s更新crd会发生什么(1.26)

## 模拟测试

先清空

```
kubectl delete crd examples.example.com
```

创建一个CRD，例如我们创建一个名为 `example.com` 的 CRD，资源类型为 `Example`：

```yaml
kubectl apply -f - <<EOF
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: examples.example.com
spec:
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

创建一个CR：

```yaml
kubectl apply -f - <<EOF
apiVersion: "example.com/v1"
kind: Example
metadata:
  name: example1
spec:
  field1: "value1"
  field2: "value2"
EOF
```

查看

```
kubectl get example example1 -o yaml
apiVersion: example.com/v1
kind: Example
metadata:
  creationTimestamp: "2023-07-28T07:48:41Z"
  generation: 1
  name: example1
  namespace: default
  resourceVersion: "14190"
  uid: dcb179ee-6ee7-4835-8ed2-68a0113f2579
spec:
  field1: value1
  field2: value2
```

更新CRD，我们

- 删除 `field1` 
- 将`field2` 改成`number`
- 添加一个字段`field3`

```yaml
kubectl apply -f - <<EOF
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: examples.example.com
spec:
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
              field2:
                type: number
              field3:
                type: string
  scope: Namespaced
  names:
    plural: examples
    singular: example
    kind: Example
EOF
```

再次查看

```bash
kubectl get example example1 -o yaml
apiVersion: example.com/v1
kind: Example
metadata:
  creationTimestamp: "2023-07-28T07:48:41Z"
  generation: 1
  name: example1
  namespace: default
  resourceVersion: "14190"
  uid: dcb179ee-6ee7-4835-8ed2-68a0113f2579
spec:
  field2: value2
```

只创建一个新字段

```
kubectl apply -f - <<EOF
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: examples.example.com
spec:
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
              field3:
                type: string
  scope: Namespaced
  names:
    plural: examples
    singular: example
    kind: Example
EOF
```

再次查看

```yaml
apiVersion: example.com/v1
kind: Example
metadata:
  creationTimestamp: "2023-07-28T08:03:20Z"
  generation: 1
  name: example1
  namespace: default
  resourceVersion: "15972"
  uid: 6a531e31-e821-4df8-9bd9-4ae2dd0d3a9a
spec: {}
```

## 初步结论

> [注意](https://helm.sh/zh/docs/chart_best_practices/custom_resource_definitions/)
>
> 目前不支持使用Helm升级或删除CRD。由于数据意外丢失的风险，这是经过多次社区讨论后作出的明确决定
>
> 所以helm upgrade --install 并不会帮你升级crd

- 增加字段不会影响旧的cr
- 删除字段会导致cr的字段删除
- 更新字段类型不会对旧的cr有影响
- 新crd如果字段key和以前全不一样, 会导致旧cr属性字段清空

排列组合太多了, 并未详细测试, 有空看下源码这块怎么处理的

## 建议实现

如果项目不大, 对字段定义没有硬性要求的情况下,可以尝试crd里只放个版本字段, 其它的字段都是动态的, 代码里自己根据版本适配

```yaml
kubectl apply -f - <<EOF
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: examples.example.com
spec:
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
            x-kubernetes-preserve-unknown-fields: true
            type: object
            required: ["version"]
            properties:
              version:
                type: string
  scope: Namespaced
  names:
    plural: examples
    singular: example
    kind: Example
EOF
```

创建cr

```yaml
kubectl apply -f - <<EOF
apiVersion: "example.com/v1"
kind: Example
metadata:
  name: example1
spec:
  version: "v1"
  foo: "bar"
EOF
```

