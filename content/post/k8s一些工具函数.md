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

### 重试机制

```go
package main

import (
	"context"
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/retry"
)

func main() {
	kubeconfig := "/Users/inksnw/.kube/config"
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		panic(err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}
	
	//go updatePodLabels(clientset, "nginx", "default", "key1", "value1")
	//go updatePodLabels(clientset, "nginx", "default", "key1", "value2")

	go updatePodLabelsWithRetry(clientset, "nginx", "default", "key1", "value1")
	go updatePodLabelsWithRetry(clientset, "nginx", "default", "key1", "value2")
	select {}
}
func updatePodLabels(clientset *kubernetes.Clientset, podName string, namespace string, key string, value string) error {
	pod, err := clientset.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	pod.ObjectMeta.Labels[key] = value
	opt := metav1.UpdateOptions{}
	_, updateErr := clientset.CoreV1().Pods(namespace).Update(context.TODO(), pod, opt)
	if updateErr != nil {
		fmt.Println("更新失败", updateErr)
	}
	return updateErr
}

func updatePodLabelsWithRetry(clientset *kubernetes.Clientset, podName string, namespace string, key string, value string) {
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		pod, err := clientset.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		pod.ObjectMeta.Labels[key] = value
		opt := metav1.UpdateOptions{}
		_, updateErr := clientset.CoreV1().Pods(namespace).Update(context.TODO(), pod, opt)
		return updateErr
	})
	if retryErr != nil {
		fmt.Printf("Update failed: %v\n", retryErr)
	}
}

```

### WrapTransport

```go
package main

import (
	"context"
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"net/http"
)

type wrap struct {
	rt http.RoundTripper
}

func (w wrap) RoundTrip(request *http.Request) (*http.Response, error) {
	fmt.Println(request.URL.String())
	return w.rt.RoundTrip(request)
}

func main() {
	kubeconfig := "/Users/inksnw/.kube/config"
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		panic(err)
	}
	config.WrapTransport = func(rt http.RoundTripper) http.RoundTripper {
		return wrap{rt}
	}
	client, err := kubernetes.NewForConfig(config)
	pod, err := client.CoreV1().Pods("default").Get(context.TODO(), "nginx", metav1.GetOptions{})
	if err != nil {
		panic(err)
	}
	fmt.Println(pod.Name)
}
```

### 多集群clientset获取

config示例

```yaml
apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: xxx
    server: https://127.0.0.1:8443/apis/clusterpedia.io/v1beta1/resources/clusters/cluster-example
  name: cluster-example
- cluster:
    certificate-authority-data: xxx
    server: https://192.168.50.50:6443
  name: cluster.local
- cluster:
    certificate-authority-data: xxx
    server: https://127.0.0.1:8443/apis/clusterpedia.io/v1beta1/resources
  name: clusterpedia
contexts:
- context:
    cluster: cluster.local
    user: kubernetes-admin
  name: kubernetes-admin@cluster.local
current-context: kubernetes-admin@cluster.local
kind: Config
preferences: {}
users:
- name: kubernetes-admin
  user:
    client-certificate-data: xxx
    client-key-data: xxx
```

获取指定集群的clientset

```go
package main

import (
	"context"
	"fmt"
	"log"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)


func createClient(clusterName string) (*kubernetes.Clientset, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{
		CurrentContext: "kubernetes-admin@cluster.local",
	}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	rawConfig, err := kubeConfig.RawConfig()
	if err != nil {
		return nil, err
	}
	clientConfig := clientcmd.NewDefaultClientConfig(rawConfig, configOverrides)
	config, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}
	config.Host = rawConfig.Clusters[clusterName].Server
	fmt.Printf("连接的地址是: %s", config.Host)
	//跳过tls验证
	config.TLSClientConfig = rest.TLSClientConfig{
		Insecure: true,
	}
	clientset, err := kubernetes.NewForConfig(config)

	return clientset, err
}

func main() {
	clusterName := "cluster-example"
	clientset, err := createClient(clusterName)
	if err != nil {
		log.Fatal(err)
	}
	podsClient := clientset.CoreV1().Pods("default")
	podList, err := podsClient.List(context.Background(), v1.ListOptions{})
	if err != nil {
		log.Fatal(err)
	}
	for _, pod := range podList.Items {
		fmt.Println("Pod:", pod.Name)
	}
}
```

### 开发授权

> configmaps is forbidden: User “system:anonymous” cannot list resource “configmaps” in API group “” in the namespace “default”

给匿名用户授权即可解决，测试环境可用此快速解决

```bash
kubectl create clusterrolebinding test:anonymous --clusterrole=cluster-admin --user=system:anonymous
```

### runtime.object 转成 unstructured

```go
import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func ConvertToUnstructured(obj runtime.Object) (*unstructured.Unstructured, error) {
	objMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}

	unstructuredObj := &unstructured.Unstructured{Object: objMap}
	return unstructuredObj, nil
}
```

### unstructured 转成 runtime.object

```go
func ConvertToRuntimeObject(unstructuredObj *unstructured.Unstructured, obj runtime.Object) error {
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.UnstructuredContent(), obj)
	return err
}
```

