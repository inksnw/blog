---
title: "升级带controller的crd"
date: 2023-12-11T17:08:47+08:00
tags: ["k8s"]
---

要升级一个crd,由crd1到crd2, 变更非常大,已经是不同的`group`, `version` 了, 所以不能用同一个crd的不同版本解决, 手动完成了两个cr的数据转换后, 打算把旧的crd1删除,  但是此时环境上还运行这crd1的controller, 因为一些原因controller不能停,

已知删除crd时会同时清理掉cr, 所以如果直接删除crd1可能导致controller运行把关联的资源一起删了(比如cr创建了一个数据库),  那么如何解决, 有没有办法在升级的时候关闭controller的运行

## 复现

创建crd

```yaml
kubectl apply -f - <<EOF
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
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
          properties:
            spec:
              type: object
              properties:
                cronSpec:
                  type: string
            status:
              type: object
              properties:
                state:
                  type: string
      subresources:
        status: {}
  scope: Namespaced
  names:
    plural: crontabs
    singular: crontab
    kind: CronTab
    shortNames:
    - ct1
EOF
```
创建cr

```yaml
kubectl apply -f - <<EOF
apiVersion: "stable.example.com/v1"
kind: CronTab
metadata:
  name: cron1
spec:
  cronSpec: "* * * * */5"
status:
  state: creating
EOF
```

写一个controller

```go
package main

import (
	"fmt"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/clientcmd"
)

func checkErr(err error) {
	if err != nil {
		panic(err)
	}

}

type Event struct {
}

func (e Event) OnAdd(obj interface{}, isInInitialList bool) {
	fmt.Println("---- OnAdd")
}

func (e Event) OnUpdate(oldObj, newObj interface{}) {
	fmt.Println("---- OnUpdate")
}

func (e Event) OnDelete(obj interface{}) {
	fmt.Println("---- 一些我们并不想让它触发的事情")
	fmt.Println("---- OnDelete")
}

func main() {
	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	checkErr(err)
	dynamicClient, err := dynamic.NewForConfig(config)
	checkErr(err)
	shardInformer := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, 0)
	nodeGvr := schema.GroupVersionResource{
		Group:    "stable.example.com",
		Version:  "v1",
		Resource: "crontabs",
	}
	shardInformer.ForResource(nodeGvr).Informer().AddEventHandler(&Event{})

	stopCh := make(chan struct{})
	shardInformer.Start(stopCh)
	shardInformer.WaitForCacheSync(stopCh)
	select {}
}
```

删除测试

```bash
kubectl delete  crd crontabs.stable.example.com
---- OnAdd
---- 一些我们并不想让它触发的事情
---- OnDelete
```

## 解决方案

[官方文档](https://kubernetes.io/zh-cn/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definition-versioning/#overview) 已经提供了方案, 即把在 `spec.versions` 列表中将旧版本的 `served` 设置为 `false`

```
kubectl edit crd crontabs.stable.example.com
```

测试

再次执行查询,或者删除都会报错

```bash
root@node1:~# kubectl get crontabs.stable.example.com 
error: the server doesn't have a resource type "crontabs"
```

controller并未停止, 但日志已报错

```bash
E1211 17:35:46.333746    2797 reflector.go:147] pkg/mod/k8s.io/client-go@v0.28.4/tools/cache/reflector.go:229: Failed to watch stable.example.com/v1, Resource=crontabs: failed to list stable.example.com/v1, Resource=crontabs: the server could not find the requested resource
```

此时删除crd, 查看 `controller`的日志, 并未触发我们不想执行的逻辑

```bash
kubectl delete crd crontabs.stable.example.com
```

因此, 只需要配置`served`字段即可暂停controller的运行,  但删除crd又涉及到cr的`finalizers` 问题,  需要手动清除, 原理可以参考 [k8s删除crd会发生什么](http://inksnw.asuscomm.com:3001/post/k8s%E5%88%A0%E9%99%A4crd%E4%BC%9A%E5%8F%91%E7%94%9F%E4%BB%80%E4%B9%88/)

## 源码分析

<img src="https://inksnw.asuscomm.com:3001/blog/升级带controller的crd_9d09e21d90b2a27de40389b5034a2499.png" alt="image-20231211185944756" style="zoom:50%;" />

当`served` 为false时, 就会跳过写入`addAPIServiceToSync`

```go
func (c *autoRegisterController) addAPIServiceToSync(in *v1.APIService, syncType string) {
	c.apiServicesToSyncLock.Lock()
	defer c.apiServicesToSyncLock.Unlock()

	apiService := in.DeepCopy()
	if apiService.Labels == nil {
		apiService.Labels = map[string]string{}
	}
	apiService.Labels[AutoRegisterManagedLabel] = syncType

	c.apiServicesToSync[apiService.Name] = apiService
	c.queue.Add(apiService.Name)
}
```

`addAPIServiceToSync` 会把数据写入到一个队列中, 这个队列会被取出注册api服务

<img src="https://inksnw.asuscomm.com:3001/blog/升级带controller的crd_75d369b81273b20a7e88b9e0c24170f4.png" alt="image-20231211190244071" style="zoom:50%;" />

如同这个字段的注释, api 服务关了, controller自然也就不运行了

>  served is a flag enabling/disabling this version from being served via REST APIs
