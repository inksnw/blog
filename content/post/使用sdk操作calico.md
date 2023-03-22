---
title: "使用sdk操作calico"
date: 2023-03-22T10:59:38+08:00
tags: ["k8s"]
---

官方提供了`calicoctl`来操作calico, 但是使用[sdk](https://github.com/projectcalico/api)时一直提示资源未找到

研究了一下, 发现sdk使用的api版本与`calicoctl`并不相同, 需要安装一个api-server才行

```bash
kubectl create -f https://raw.githubusercontent.com/projectcalico/calico/v3.25.0/manifests/apiserver.yaml
openssl req -x509 -nodes -newkey rsa:4096 -keyout apiserver.key -out apiserver.crt -days 365 -subj "/" -addext "subjectAltName = DNS:calico-api.calico-apiserver.svc"

kubectl create secret -n calico-apiserver generic calico-apiserver-certs --from-file=apiserver.key --from-file=apiserver.crt

kubectl patch apiservice v3.projectcalico.org -p \
    "{\"spec\": {\"caBundle\": \"$(kubectl get secret -n calico-apiserver calico-apiserver-certs -o go-template='{{ index .data "apiserver.crt" }}')\"}}"
```

如果是使用的operator安装的calico, 默认就带了这个服务, [参考](https://docs.tigera.io/calico/latest/operations/install-apiserver)

sdk使用示例

```go
package main

import (
	"context"
	"fmt"
	"github.com/projectcalico/api/pkg/client/clientset_generated/clientset"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	config := K8sRestConfig()
	cs, err := clientset.NewForConfig(config)
	if err != nil {
		panic(err)
	}
	list, err := cs.ProjectcalicoV3().IPPools().List(context.Background(), v1.ListOptions{})
	if err != nil {
		panic(err)
	}
	for _, gnp := range list.Items {
		fmt.Printf("%#v\n", gnp)
	}
}

func K8sRestConfig() *rest.Config {
	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	if err != nil {
		panic(err)
	}
	return config
}
```

