---
title: "k8s一些工具函数"
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
	"sigs.k8s.io/controller-runtime/pkg/client/config"
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
	conf := config.GetConfigOrDie()
	ci, _ := dynamic.NewForConfig(conf)
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
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

func main() {

	conf := config.GetConfigOrDie()
	clientset := kubernetes.NewForConfigOrDie(conf)
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
	"bytes"
	"context"
	"fmt"
	"io"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/metadata"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

type wrap struct {
	rt http.RoundTripper
}

func (w wrap) RoundTrip(request *http.Request) (*http.Response, error) {
	fmt.Println(request.URL.String())
	rsp, err := w.rt.RoundTrip(request)
	if err != nil {
		return nil, err
	}
	b, _ := io.ReadAll(rsp.Body)
	fmt.Println("响应长度是: ", len(b))
	rsp.Body = io.NopCloser(bytes.NewBuffer(b))

	return rsp, err
	//return w.rt.RoundTrip(request)
}

func main() {

	conf := config.GetConfigOrDie()

	conf.WrapTransport = func(rt http.RoundTripper) http.RoundTripper {
		return wrap{rt}
	}
	//使用 protobuf
	conf = metadata.ConfigFor(conf)
	client := kubernetes.NewForConfigOrDie(conf)
	
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

开发时可用此办法临时解决

```bash
kubectl create clusterrolebinding test:anonymous --clusterrole=cluster-admin --user=system:anonymous
```

### k8s类型转换

#### runtime.object 转成 unstructured

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

#### unstructured 转成 runtime.object

知道目标类型

```go
func ConvertToRuntimeObject(unstructuredObj *unstructured.Unstructured, obj runtime.Object) error {
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.UnstructuredContent(), obj)
	return err
}
```

不知道目标类型

```go
func ConvertToRuntimeObject(unstructuredObj *unstructured.Unstructured) (runtime.Object, error) {
	runtimeObj := &runtime.Unknown{}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.UnstructuredContent(), runtimeObj)
	return runtimeObj, err
}
```

any 转runtime.object

```go
func (n nodeHandle) OnDelete(obj interface{}) {
	node := obj.(*corev1.Node)
}
```

#### jsonBytes 转 runtime.Object

```go
func Decode(data []byte) (obj runtime.Object, err error) {
	decoder := unstructured.UnstructuredJSONScheme
	obj, _, err = decoder.Decode(data, nil, nil)
	return obj, err
}
```

#### jsonBytes 转 v1.Pod{}

```go
package main

import (
	"fmt"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

func main() {
	jsonStr := `{"apiVersion":"v1","kind":"Pod","metadata":{"labels":{"app":"mytest"},"name":"mytest","namespace":"default"},"spec":{"containers":[{"image":"nginx","imagePullPolicy":"IfNotPresent","name":"mytest"}],"restartPolicy":"Always"}}`
	// 设置序列化器和反序列化器
	codec := serializer.NewCodecFactory(runtime.NewScheme()).UniversalDeserializer()
	// 将JSON字符串解码到Pod对象中
	ins := &v1.Pod{}
	_, _, err := codec.Decode([]byte(jsonStr), nil, ins)
	if err != nil {
		fmt.Printf("Error decoding Pod: %v\n", err)
		return
	}
	fmt.Printf("Decoded Pod: %v\n", ins.Name)
}
```



#### gvk 和 gvr互转

```go
package main

import (
	"fmt"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

var (
	Mapper meta.RESTMapper
)

func InitMapper() {
	//获取  所有api groupresource
	conf := config.GetConfigOrDie()
	client := kubernetes.NewForConfigOrDie(conf)
	gr, err := restmapper.GetAPIGroupResources(client.Discovery())
	if err != nil {
		os.Exit(1)
	}
	Mapper = restmapper.NewDiscoveryRESTMapper(gr)
}

func GvkToGvr(gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
	mapping, err := Mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if meta.IsNoMatchError(err) || err != nil {
		return schema.GroupVersionResource{}, err
	}
	return mapping.Resource, nil
}

func GvrToGvk(gvr schema.GroupVersionResource) (schema.GroupVersionKind, error) {
	gvk, err := Mapper.KindFor(gvr)
	return gvk, err
}

func main() {
	InitMapper()
	gvk := schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Pod",
	}
	gvr, err := GvkToGvr(gvk)
	if err != nil {
		panic(err)
	}
	fmt.Println(gvr.Resource)
	gvk1, _ := GvrToGvk(gvr)
	fmt.Println(gvk1.Kind)
}
```

### 调试容器

一个网络工具很全的容器

```bash
kubectl run tmp-shell --rm -i --tty --image nicolaka/netshoot
```

### 带缓存的client

```go
package main

import (
	"context"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	restconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
)

var Scheme = runtime.NewScheme()

func main() {
	_ = clientgoscheme.AddToScheme(Scheme)
	_ = apiextensionsv1.AddToScheme(Scheme)
  
	config:= restconfig.GetConfigOrDie()
	c, err := cluster.New(config, func(options *cluster.Options) {
		options.Scheme = Scheme
	})
	if err != nil {
		panic(err)
	}

	//为特定资源增加筛选索引
	indexerFunc := func(obj client.Object) []string {
		e := obj.(*corev1.Event)
		return []string{e.InvolvedObject.Name}
	}
	if err = c.GetCache().IndexField(context.Background(), &corev1.Event{}, "involvedObject.name", indexerFunc); err != nil {
		klog.Fatalf("unable to create index field: %v", err)
	}
	//启动缓存同步
	go c.GetCache().Start(context.Background())
	if c.GetCache().WaitForCacheSync(context.Background()) {
		fmt.Println("Cache sync success")
	}

	namespace := corev1.NamespaceList{}
	err = c.GetClient().List(context.Background(), &namespace)
	if err != nil {
		panic(err)
	}
	for _, i := range namespace.Items {
		fmt.Println(i.Name)
	}

}
```

### 使用k8s仓库

解决版本 `0.0.0` 问题

```bash
MODS=($(
curl -sS https://raw.githubusercontent.com/kubernetes/kubernetes/v1.26.5/go.mod |
sed -n 's|.*k8s.io/\(.*\) => ./staging/src/k8s.io/.*|k8s.io/\1|p'
))
for MOD in "${MODS[@]}"; do
go mod edit "-replace=${MOD}=${MOD}@v0.26.5"
done
go get "k8s.io/kubernetes@v1.26.5"
```

### 一些命令

```bash
kubectl run nginx --image=nginx --dry-run=client -o yaml
kubectl run nginx --image=nginx --overrides='{"apiVersion":"v1","spec":{"nodeName":"m1"}}'
kubectl create deployment nginx --image=nginx
kubectl create deployment nginx --image=nginx --dry-run=client  -oyaml 
kubectl expose pod nginx --port=80
kubectl expose deployment/nginx --port=80 --target-port=8080
kubectl expose deployment/nginx --port=80 --type=LoadBalancer
kubectl port-forward svc/web 8080:80
kubectl port-forward pod nginx 8888:80
kubectl set image deployment/web web=nginx:1.20
kubectl rollout history deployment/web
kubectl rollout undo deployment/web --to-revision=2
kubectl rollout restart deployment/web
kubectl scale --replicas=3 deployment/web
kubectl label pods web platform=www
kubectl config set-context --current --namespace production
```

