---
title: "K8s一些工具函数"
date: 2023-06-15T22:32:54+08:00
tags: ["k8s"]
---

### 通用的server-side apply

```go
package main

import (
	"context"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {

	client := InitDynamicClient()
	pod := serverSideApply("v2")
	gvr := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "pods",
	}
	podJson, _ := json.Marshal(pod)
	opt := v1.PatchOptions{FieldManager: "somebody"}
	_, err := client.Resource(gvr).Namespace("default").
		Patch(context.TODO(), pod.Name, types.ApplyPatchType, podJson, opt)
	if err != nil {
		panic(err)
		return
	}
}

func serverSideApply(version string) corev1.Pod {
	pod := corev1.Pod{}
	pod.APIVersion = "v1"
	pod.Kind = "Pod"
	pod.Namespace = "default"
	pod.Name = "nginx"
	pod.Labels = map[string]string{"version": version}
	pod.Spec = corev1.PodSpec{
		Containers: []corev1.Container{{Name: "nginx", Image: "nginx"}},
	}
	return pod
}
func InitDynamicClient() dynamic.Interface {
	config, _ := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	ci, _ := dynamic.NewForConfig(config)
	return ci
}

```

### 读取多行yaml

```go
package main

import (
	"bufio"
	"fmt"
	"io"
	"k8s.io/apimachinery/pkg/util/yaml"
	"os"
)

func main() {
	f, err := os.Open("./nginx.yaml")
	if err != nil {
		panic(err)
	}
	bf := bufio.NewReader(f)
	r := yaml.NewYAMLReader(bf)
	for {
		data, err := r.Read()
		if err != nil && err == io.EOF {
			fmt.Println("读完了")
			break
		}
		fmt.Println(string(data))
	}
}
```

