---
title: "Crd更新status"
date: 2023-09-13T15:42:13+08:00

---

crd的status字段，如果要更新，需要使用`UpdateStatus`方法

## 创建crd与cr

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

```yaml
kubectl apply -f - <<EOF
apiVersion: "stable.example.com/v1"
kind: CronTab
metadata:
  name: my-new-cron-object
spec:
  cronSpec: "* * * * */5"
status:
  state: creating
EOF
```
通过 kubectl apply 或者 edit 是无法更新status的

## 代码测试

```go
package main

import (
	"context"
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/metadata"
	"k8s.io/client-go/tools/clientcmd"
	"net/http"
)

func (w wrap) RoundTrip(request *http.Request) (*http.Response, error) {
	//fmt.Printf("方法: %s, 请求url: %s\n", request.Method, request.URL.String())
	return w.rt.RoundTrip(request)
}

type wrap struct {
	rt http.RoundTripper
}

var (
	namespace = "default"            // Modify this as needed
	crName    = "my-new-cron-object" // Modify this as needed
	gvr       = schema.GroupVersionResource{
		Group:    "stable.example.com",
		Version:  "v1",
		Resource: "crontabs",
	}
	dynamicClient *dynamic.DynamicClient
)

func main() {
	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	if err != nil {
		panic(err)
	}
	config.WrapTransport = func(rt http.RoundTripper) http.RoundTripper {
		return wrap{rt}
	}
	// 使用protobuf传输数据
	config = metadata.ConfigFor(config)
	dynamicClient, _ = dynamic.NewForConfig(config)

	utd, cron, status := getInfo()
	fmt.Printf("初始值 %s %s\n", cron, status)
	unstructured.SetNestedField(utd.Object, "1", "status", "state")
	unstructured.SetNestedField(utd.Object, "1", "spec", "cronSpec")
	_, err = dynamicClient.Resource(gvr).Namespace(namespace).UpdateStatus(context.TODO(), utd, metav1.UpdateOptions{})
	if err != nil {
		panic(err)
	}
	utd, cron, status = getInfo()
	fmt.Printf("UpdateStatus       预期: %s, 实际spec: %s, 实际status: %s\n", "1", cron, status)

	unstructured.SetNestedField(utd.Object, "2", "status", "state")
	unstructured.SetNestedField(utd.Object, "2", "spec", "cronSpec")
	json, _ := utd.MarshalJSON()
	_, err = dynamicClient.Resource(gvr).Namespace(namespace).
		Patch(context.TODO(), crName, types.MergePatchType, json, metav1.PatchOptions{})
	if err != nil {
		panic(err)
	}
	utd, cron, status = getInfo()
	fmt.Printf("Patch              预期: %s, 实际spec: %s, 实际status: %s\n", "2", cron, status)

	unstructured.SetNestedField(utd.Object, "3", "status", "state")
	unstructured.SetNestedField(utd.Object, "3", "spec", "cronSpec")
	json, _ = utd.MarshalJSON()
	_, err = dynamicClient.Resource(gvr).Namespace(namespace).
		Patch(context.TODO(), crName, types.MergePatchType, json, metav1.PatchOptions{}, "status")
	if err != nil {
		panic(err)
	}
	utd, cron, status = getInfo()
	fmt.Printf("Patch+subresources 预期: %s, 实际spec: %s, 实际status: %s\n", "3", cron, status)
}

func getInfo() (utd *unstructured.Unstructured, cron, status string) {
	utd, err := dynamicClient.Resource(gvr).Namespace(namespace).Get(context.TODO(), crName, metav1.GetOptions{})
	if err != nil {
		panic(err)
	}
	cron, _, _ = unstructured.NestedString(utd.Object, "spec", "cronSpec")
	status, _, _ = unstructured.NestedString(utd.Object, "status", "state")
	return utd, cron, status
}

```

查看日志可以看到,当使用`UpdateStatus` 或者 patch加上`subresources`参数时, 请求的路由多了`/status`

```basic
方法: GET, 请求url: https://192.168.50.50:6443/apis/stable.example.com/v1/namespaces/default/crontabs/my-new-cron-object
方法: PATCH, 请求url: https://192.168.50.50:6443/apis/stable.example.com/v1/namespaces/default/crontabs/my-new-cron-object/status

  
初始值 2 3
UpdateStatus       预期: 1, 实际spec: 2, 实际status: 1
Patch              预期: 2, 实际spec: 2, 实际status: 1
Patch+subresources 预期: 3, 实际spec: 2, 实际status: 3
```

另外可以看出`UpdateStatus`, `Patch`, 或者是 `Patch+subresources`, 都是各自更新各自的, 不会一起更新, 所以, 如果你两个都要更新, 得调用两次

## 源码分析

在`kubernetes-1.26.5/staging/src/k8s.io/apiextensions-apiserver/pkg/apiserver/customresource_handler.go` 342行定义了如何处理`/status`请求

![image-20230914163904740](http://inksnw.asuscomm.com:3001/blog/crd更新status_61f5f9a9c7d91d6f2180d2609a171748.png)

`r.serveStatus` 和 `r.serveResource` 更新时都会调用 `handlers.UpdateResource(storage, requestScope, r.admission)` ,但传入的 storage不同, 也印证了上文说的, 更新子资源与更新主资源各自是各自的逻辑

- ```
  storage := crdInfo.storages[requestInfo.APIVersion].Status
  ```

- ```
  storage := crdInfo.storages[requestInfo.APIVersion].CustomResource
  ```

## 命令行操作

如果想使用kubectl 更新子资源, 需要加上 `--subresource` 参数, 版本要求 `Kubernetes v1.24`, 参考 [官方文档](https://kubernetes.io/zh-cn/docs/tasks/manage-kubernetes-objects/update-api-object-kubectl-patch/#scale-kubectl-patch) 

> 推测核心逻辑代码一直在, 只是kubectl 在1.24才加上了这个子命令, 所以如果你的集群比较旧, 可以单独下载个新的kubectl 执行这个命令

```bash
kubectl patch OperatorApplicationVersion kafka --subresource='status' --type='merge' -p '{"status":{"state":"suspended"}}'
```



