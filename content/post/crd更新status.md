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
	fmt.Printf("方法: %s, 请求url: %s\n", request.Method, request.URL.String())
	return w.rt.RoundTrip(request)
}

type wrap struct {
	rt http.RoundTripper
}

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

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		panic(err)
	}
	namespace := "default"         // Modify this as needed
	crName := "my-new-cron-object" // Modify this as needed
	gvr := schema.GroupVersionResource{
		Group:    "stable.example.com",
		Version:  "v1",
		Resource: "crontabs",
	}
	utd, err := dynamicClient.Resource(gvr).Namespace(namespace).Get(context.TODO(), crName, metav1.GetOptions{})
	if err != nil {
		panic(err)
	}
	unstructured.SetNestedField(utd.Object, "running1", "status", "state")

	//使用 UpdateStatus 可以更新成功
	//_, err = dynamicClient.Resource(gvr).Namespace(namespace).UpdateStatus(context.TODO(), utd, metav1.UpdateOptions{})
	//if err != nil {
	//	panic(err)
	//}
	unstructured.SetNestedField(utd.Object, "patch3", "status", "state")
	unstructured.SetNestedField(utd.Object, "113", "spec", "cronSpec")

	json, err := utd.MarshalJSON()
	if err != nil {
		panic(err)
	}
	//
	//不加subresources 无法更新
	//_, err = dynamicClient.Resource(gvr).Namespace(namespace).
	//	Patch(context.TODO(), crName, types.MergePatchType, json, metav1.PatchOptions{})
	//if err != nil {
	//	panic(err)
	//	return
	//}
	//加subresources 可以更新
	_, err = dynamicClient.Resource(gvr).Namespace(namespace).
		Patch(context.TODO(), crName, types.MergePatchType, json, metav1.PatchOptions{}, "status")
	if err != nil {
		panic(err)
		return
	}

	fmt.Println("CR status updated successfully.")
}
```

查看日志可以看到,当使用`UpdateStatus` 或者 patch加上`subresources`参数时, 请求的路由多了`/status`

```basic
方法: GET, 请求url: https://192.168.50.50:6443/apis/stable.example.com/v1/namespaces/default/crontabs/my-new-cron-object
方法: PATCH, 请求url: https://192.168.50.50:6443/apis/stable.example.com/v1/namespaces/default/crontabs/my-new-cron-object/status
CR status updated successfully.
```
