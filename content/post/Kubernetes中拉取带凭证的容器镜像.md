---
title: "Kubernetes中拉取带凭证的容器镜像"
date: 2023-08-30T20:29:51+08:00
tags: ["k8s"]
---

## 简单使用

源码位于`kubernetes-1.26.5/pkg/kubelet/kubelet.go` 1827行, syncPod的时候需要拉取镜像/验证镜像, 这时候就需要验证auth

在syncPod的步骤中会首先验证镜像是否存在, 发送grpc的拉取镜像请求, 这时会使用带上的认证信息, 直接配置容器运行时如`docker login` 是没有用的, k8s并不会使用这个登录信息

<img src="http://inksnw.asuscomm.com:3001/blog/Kubernetes中拉取带凭证的容器镜像_9b829e648e5360c9a165c4b5f6f58034.png" alt="image-20230830210358808" style="zoom:50%;" />

简单使用, 手动注入

```bash
docker login 
kubectl create secret generic regcred  --from-file=.dockerconfigjson=/root/.docker/config.json --type=kubernetes.io/dockerconfigjson
```

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: private-reg
spec:
  containers:
  - name: private-reg-container
    image: nginx
  imagePullSecrets:
  - name: regcred
```

## 指定名称空间

每个名称空间会默认关联一个名为default的serviceAccount, 而ServiceAccount 的属性可以被注入到Pod 中, 因此我们可以利用这个方法配置一个名称空间的拉取凭证

```bash
kubectl create ns test
kubectl edit serviceAccounts default -n test
# 增加属性`imagePullSecrets`
kubectl patch serviceaccount default -p '{"imagePullSecrets": [{"name": "regcred"}]}' -n test
```

在此空间下创建一个pod

```bash
kubectl run nginx --image=nginx -n test
kubectl get pod -n test -o jsonpath="{.items[*].spec.imagePullSecrets}"
[{"name":"regcred"}]
```

## 全局设置

k8s并未提供全局设置的办法

这里提供两种方法

### 对所有的ServiceAccount打patch

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	config, _ := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	client, _ := kubernetes.NewForConfig(config)

	log.Info("Service-Account Patcher started")
	name := "regcred"
	var patched = 0

	// get all service accounts in all namespaces
	serviceAccounts, err := client.CoreV1().ServiceAccounts("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}

	for _, sa := range serviceAccounts.Items {
		if includeImagePullSecret(&sa, name) {
			continue
		}
		patch, err := getPatchString(&sa, name)
		if err != nil {
			panic(err.Error())
		}
		_, err = client.CoreV1().ServiceAccounts(sa.Namespace).
			Patch(context.TODO(), sa.Name, types.StrategicMergePatchType, patch, metav1.PatchOptions{})
		if err != nil {
			panic(err.Error())
		}
		patched++
	}
	fmt.Println(patched)
}

func includeImagePullSecret(sa *corev1.ServiceAccount, secretName string) bool {
	for _, imagePullSecret := range sa.ImagePullSecrets {
		if imagePullSecret.Name == secretName {
			return true
		}
	}
	return false
}
func getPatchString(sa *corev1.ServiceAccount, secretName string) ([]byte, error) {
	type patch struct {
		ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
	}
	saPatch := patch{
		ImagePullSecrets: append([]corev1.LocalObjectReference(nil), sa.ImagePullSecrets...),
	}
	saPatch.ImagePullSecrets = append(saPatch.ImagePullSecrets, corev1.LocalObjectReference{Name: secretName})
	return json.Marshal(saPatch)
}
```

### webhook

> 拦截创建pod请求 , 并自动注入imagePullSecrets

### 给k8s提pr

> ^_
