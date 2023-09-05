---
title: "用聚合api实现抽象资源"
date: 2023-09-05T17:10:45+08:00
tags: ["k8s"]
---



## 背景

计算机行业, 没有什么不能加一层缓存实现,  但是也带来了一个常见问题, 如何保持数据一致.

比如你在阿里云和腾讯云各创建了三台主机, 现在要做一个管理系统管理这些主机, 要如何做 .

引入一个数据库, 创建一个主机就在数据库里记录一条

> 但是如果有人绕过这个平台, 直接在阿里云创建了一个主机, 那数据就不同步了

两种解决方案

- 在管理平台查询主机列表时, 不记录真实的主机, 而是透传查询云商的主机列表api, 把结果合并, 但是这会带来分页和筛选的问题, 取前3条, 是阿里2条,腾讯1条, 还是反过来?
- 管理平台只存储一些需要筛选的元信息, 并实现一个事件机制,当云商的api被调动就向管理平台通知增加元信息记录, 查询详情的时候透传

## k8s场景

k8s中也同样会有类似的问题, 比如实现一个应用商店, `redis` 和 `mysql` 是不同的crd, 要聚合在一起显示, 和上文描述的场景一样, 也得在上层放一个对象, 我们假设用一个crd实现, 这里暂叫他 `manifest`

- 创建的时候创建 `manifest` 对象, 里面写上简化的实际资源的spec信息, 在controller的实现中, 解包`manifest` 提交实际的cr, `manifest` 中保留少量的元信息用于搜索, 查看 . 
- 通过informer 监听实例(mysql)信息, 把运行状态等同步到 `manifest`

搞定, 下班

> 但是, 如果用户绕过ui, 直接编辑`mysql` 的配置怎么办, 数据不就不同步了么

那简单, 我们可以通过label等标记真实资源的信息, 让前端编辑时, 去查一下真实资源信息再编辑, 但这个会增大前端的复杂度 .

或者 `manifest` 对象中不再只写简化的实际资源spec, 写全量的, 同时再通过informer把实际资源的状态变更全量同步到 `manifest` 不就可以了么, 是的, 但这会带来延迟和性能开销, 同时操作时可能会有数据冲突 ( 似乎也不是多大的事 -_- )

## 聚合api

所以有没有其它的办法呢, 我们简化这个场景, 就是提交的是A, 实际操作的是B, 有另一个入口操作B时, A最好还能同步

这里就引入k8s的AA `Aggregation Api`  官方文档 [可以参考](https://kubernetes.io/zh-cn/docs/concepts/extend-kubernetes/api-extension/apiserver-aggregation/) 

crd资源我们只能定义cr 的增删改查的事件发生后做什么, 但并不能修改增删改查的逻辑, 而AA可以, 那么如果我们把创建时改成提交另一个资源, 查询/编辑时透传真实资源, 上面的问题不就解决了么

### ShadowResource

为了解决上述问题, 我们可以引入一个名为`ShadowResource` 虚拟资源概念

```yaml
apiVersion: apis.abc.com/v1
kind: ShadowResource
metadata:
  name: task1
  namespace: default
spec:
  flowList:
    - apiVersion: v1
      kind: Pod
      metadata:
        name: nginx1
        namespace: default
      spec:
        containers:
          - image: nginx
            name: nginx
    - apiVersion: v1
      kind: Pod
      metadata:
        name: nginx2
        namespace: default
        annotations:
          foo1: bar2
      spec:
        containers:
          - image: nginx
            name: nginx
```

为ShadowResource实现以下功能：

1. **批量提交**: 创建时会循环flowList进行提交, 可以实现一次性创建多个资源, 如创建MySQL实例时提交多个CR。
2. **资源信息存储**: 记录相关资源的GVR,名称和命名空间等信息, 可存储在Etcd、CRD或关系型数据库中。
3. **实时查询**: 通过聚合API的可定制性, 可以实时查询相关资源, 并将其信息写回flowList, 确保用户编辑的是实时资源, 避免与命令行操作冲突。
4. **删除**: 在删除时, 可以循环删除flowList中定义的资源, 确保资源的清理。

### 实现

实现了个demo [https://github.com/inksnw/shadowresource](https://github.com/inksnw/shadowresource)  代码的实现原理有空再写一下

参考文档 [编写Kubernetes风格的APIServer](https://blog.gmem.cc/kubernetes-style-apiserver)

## 优缺点

优点

- **api统一管理**：可以实现`kubectl`和 `restapi`的统一。

- **实时查询**：通过聚合API实时查询资源信息, 保持用户编排的是原生资源, 避免了数据不一致和冲突问题。

缺点

- **复杂性**：引入AA会增加系统的复杂性, 同步用于搜索和展示的元数据时, 值如何从真实资源中取得需要配置

- **抽象程度**：并没有降低用户接触一个新的 crd 的时候的学习成本

