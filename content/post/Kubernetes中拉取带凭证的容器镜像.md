---
title: "Kubernetes中拉取带凭证的容器镜像"
date: 2023-08-30T20:29:51+08:00
tags: ["k8s"]
---

k8s 拉取带凭据的镜像源时, 需要配置拉取凭据, 有几种办法

## 配置到kubelet启动

```
nerdctl login
cp ~/.docker/config.json /var/lib/kubelet/
systemctl restart kubelet
```
kubelet启动的时候会读取 `searchPaths` 中的配置文件
<img src="http://inksnw.asuscomm.com:3001/blog/Kubernetes中拉取带凭证的容器镜像_35eee10da3d183008a977cff24752a25.png" alt="image-20230922140925408" style="zoom:50%;" />

实际获取凭据的时候会把 `imagePullSecrets` 的信息和kubelet启动文件拿到的信息合并

<img src="http://inksnw.asuscomm.com:3001/blog/Kubernetes中拉取带凭证的容器镜像_00a60edd332da84e96262bb2f6f00a76.png" alt="image-20230922141216159" style="zoom:50%;" />



## 配置到pod中

源码位于`kubernetes-1.26.5/pkg/kubelet/kubelet.go` 1827行, 在syncPod的步骤中会发送grpc的拉取镜像请求, 这时会使用带上的认证信息, 这个信息来自于pod的 `imagePullSecrets`

<img src="http://inksnw.asuscomm.com:3001/blog/Kubernetes中拉取带凭证的容器镜像_9b829e648e5360c9a165c4b5f6f58034.png" alt="image-20230830210358808" style="zoom:50%;" />

简单使用, 手动注入

```bash
nerdctl login 
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

这样, 这个pod的拉取就会使用regcred 里配置的用户名密码

## 配置到serviceaccount

但是总不能,每个pod都手动添加一下这个参数吧, 如下配置, 即可以一个名称空间指定`imagePullSecrets`

每个名称空间会默认关联一个名为default的serviceAccount, 而ServiceAccount 的属性会被注入到这个名称空间下的Pod 中, 因此我们可以利用这个方法配置一个名称空间的拉取凭证

```bash
kubectl create ns test
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

这里提供几种办法, 第一种算是上面方法的进阶

### 打patch

对所有的ServiceAccount打patch

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

给对所有的ServiceAccount打patch还存在一个问题, 万一patch还没来得及打上, pod的配置已经生成了, 那此时还是不行,持续执行上面的patch操作可以么,也不太好, 因为这个时间差不可控,  所以也可以实现一个webhook

> 拦截创建pod请求 , 并自动注入imagePullSecrets, 实现比较简单, 这里就不演示了

### 给k8s提pr

理论上把上文的`pullSecrets := kl.getPullSecretsForPod(pod)` 这里加些类似的逻辑就可以, 但估计不太容易接受, 暂时先不考虑了吧

```go
var pullSecrets xxx
if kl.getPullSecretsFromGlobal()>0{
pullSecrets = kl.getPullSecretsFromGlobal()
}else{
  pullSecrets = kl.getPullSecretsForPod(pod)
}
```

