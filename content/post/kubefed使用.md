---
title: "Kubefed使用"
date: 2022-05-25T11:01:26+08:00
tags: ["k8s"]
---

## 安装配置

```bash
helm --namespace kube-federation-system upgrade -i kubefed kubefed-charts/kubefed --version=0.9.1 --create-namespace
```

安装kubefedctl二进制

配置config上下文:略

```bash
kubefedctl join k1 --cluster-context  k1-admin --host-cluster-context  k1-admin --v=2
kubefedctl join k2 --cluster-context  k2-admin --host-cluster-context  k1-admin --v=2
```

```bash
kubectl -n kube-federation-system get kubefedclusters
```

```bash
kubectl create ns fedns
```

## 创建联邦ns

在主集群k1上创建fed对象的ns

```yaml
apiVersion: types.kubefed.io/v1beta1
kind: FederatedNamespace
metadata:
  name: fedns
  namespace: fedns
spec:
  placement:
    clusters:
      - name: k1
      - name: k2
```

在子集群k2上查看ns

kubectl get ns |grep fedns

## 创建联邦deploy

```yaml
apiVersion: types.kubefed.io/v1beta1
kind: FederatedDeployment
metadata:
  name: test-deployment
  namespace: fedns
spec:
  template:
    metadata:
      labels:
        app: nginx
    spec:
      replicas: 3
      selector:
        matchLabels:
          app: nginx
      template:
        metadata:
          labels:
            app: nginx
        spec:
          containers:
            - image: nginx
              name: nginx
  placement:
    clusters:
      - name: k1
      - name: k2
  # 集群差异设置    
  overrides:
  - clusterName: k2
    clusterOverrides:
      - path: "/spec/replicas"
        value: 2
      - path: "/spec/template/spec/containers/0/image"
        value: "nginx:1.17.0-alpine"
      - path: "/metadata/annotations"
        op: "add"
        value:
          foo: bar
      - path: "/metadata/annotations/foo"
        op: "remove"
```

## 代码调用

```go
package main

import (
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"log"
	"sigs.k8s.io/kubefed/pkg/client/generic"
)

func main() {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "types.kubefed.io",
		Version: "v1beta1",
		Kind:    "FederatedDeployment",
	})
	client := generic.NewForConfigOrDie(K8sRestConfig())
	err := client.Get(context.Background(), obj, "fedns", "test-deployment")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(obj)
}
```

## CRD传播

```bash
kubefedctl enable customresourcedefinitions
```

```yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: hahas.inksnw.com
spec:
  group: inksnw.com
  names:
    kind: Haha
    plural: hahas
  scope: Namespaced
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
```

```bash
#主集群

kubefedctl federate crd  hahas.inksnw.com
kubectl get FederatedCustomResourceDefinition
kubefedctl enable hahas.inksnw.com
kubectl get FederatedTypeConfig -n kube-federation-system
kubectl get crd|grep hahas
```

