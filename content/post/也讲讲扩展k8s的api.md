---
title: "也讲讲扩展k8s的api"
date: 2024-10-29T16:25:46+08:00
tags: ["k8s"]
---

Kubernetes 可以通过多种方式扩展其 API。本文将深入探讨三种主要的扩展方式：自定义资源定义（Custom Resource Definitions, CRD）、聚合 API（API Aggregation）以及独立的 API Server。这里通过案例分析每种方式的优缺点，帮助你选择最适合你需求的扩展方式。

## 使用 CRD 扩展 Kubernetes API

原理分析

我们先实现一个最简单的动态api

```go
package main

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

var router *gin.Engine

func init() {
	router = gin.Default()
}

func addRouteHandler(c *gin.Context) {
	path := c.Param("path")
	router.GET("/"+path, func(c *gin.Context) {
		c.String(http.StatusOK, "This is a new GET route: "+path)
	})
	c.JSON(http.StatusOK, gin.H{"message": "Route added successfully"})
}

func main() {
	router.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "Welcome to the Gin router!")
	})
	router.GET("/add-route/:path", addRouteHandler)
	router.Run(":8080")
}
```

操作验证

```bash
http://127.0.0.1:8080/add-route/abc
http://127.0.0.1:8080/abc
```

是的, 这就是 crd 的核心原理, 即动态的给一个apiserver增加路由, 这里看一下k8s的源码

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

查看已经注册的 APIService

```bash
kubectl get APIService v1.stable.example.com -o yaml
apiVersion: apiregistration.k8s.io/v1
kind: APIService
metadata:
  creationTimestamp: "2024-11-05T02:23:09Z"
  labels:
    kube-aggregator.kubernetes.io/automanaged: "true"
  name: v1.stable.example.com
  resourceVersion: "3151"
  uid: 88fb4e95-44e7-4e11-8c32-c426f22b2b86
spec:
  group: stable.example.com
  groupPriorityMinimum: 1000
  version: v1
  versionPriority: 100
status:
  conditions:
  - lastTransitionTime: "2024-11-05T02:23:09Z"
    message: Local APIServices are always available
    reason: Local
    status: "True"
    type: Available
```

那还有crd 的定义呢, imformer呢, 这些只是k8s又在此附加的功能

- 生成crd 的openapi文件用于 `kubectl apply` 的时候校验, 判断字段的合并规则等等
- 提供一个`watch` api, 当一个控制程序监听到资源的变动时, 触发逻辑

k8s的watch功能本质就是利用http的`chunked`功能

源码位于k8s.io/apiserver/pkg/endpoints/handlers/watch.go 200行, 调用位置在k8s.io/apiserver/pkg/endpoints/handlers/get.go 270行

<img src="https://inksnw.asuscomm.com:3001/blog/也讲讲扩展k8s的api_bf3be1aca63e5fdd11aa57af5eaf1d48.png" alt="image-20241105135913135" style="zoom:50%;" />

这里也代码简单实现一下

```go
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

func main() {
	ch := make(chan []byte)
	go func() {
		for {
			data := map[string]string{
				"date": time.Now().Format(time.DateTime),
			}
			now, _ := json.Marshal(data)
			ch <- now
			time.Sleep(time.Second * 3)
		}
	}()
	http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		flusher, _ := writer.(http.Flusher)
		writer.Header().Set("Content-Type", "application/json")
		writer.Header().Set("Transfer-Encoding", "chunked")
		writer.WriteHeader(http.StatusOK)
		flusher.Flush()
		for {
			select {
			case data := <-ch:
				writer.Write(data)
				flusher.Flush()
			}
		}
	})
	fmt.Println("server start")
	http.ListenAndServe(":8080", nil)
}
```

<img src="https://inksnw.asuscomm.com:3001/blog/也讲讲扩展k8s的api_f8d0496aea5cb9e794d3e6d3f09fb9d7.png" alt="模拟k8s的watch_f8d0496aea5cb9e794d3e6d3f09fb9d7" style="zoom:50%;" />

有什么不足点呢?

控制程序能在资源创建/更新/删除时候触发逻辑, 但是没法处理查询时, 比如你想实现查询一个cr的时候附加一些动态的环境变量, 那就不行了 

假设在做一个数据管理平台, 要管理多种数据库实现, 通过 crd 实现有什么问题

- 有新的crd学习成本, 数据库实现可能是第三方的, 他们不愿意改成你设计的crd, 那用一个字段存储具体crd的字符串,套娃实现?

```yaml
spec:
 realCRD: "xxxx"
```

- 但是如果用户绕过直接改底层的crd会有数据同步问题, 就需要做数据同步, realCRD可能是很多个不同资源, 全做监听同步, 成本似乎有点高

- 如果需求只是 `restapi`有自己的服务端, 可以透传查询真实的资源, 如果不是呢, 使用者直接对接的k8s的api呢

局限的原因, crd只能在controller层实现创建/删除/修改逻辑, 而查询是标准的k8s api 无法自定义



## 使用聚合 API 扩展 Kubernetes API

聚合 API（API Aggregation）是 Kubernetes 提供的一种更高级的 API 扩展方式。通过聚合 API，用户可以编写一个独立的 API Server，并将其注册到 Kubernetes 的 API Server 中。Kubernetes 会将对该 API Server 的请求转发到该 API Server 进行处理。

核心原理, 两个字:  `转发`, 写一段最简单的api代码

```go
package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/json"
)

func ApiResourceList() metav1.APIResourceList {
	apiList := metav1.APIResourceList{
		GroupVersion: "apis.abc.com/v1",
		APIResources: []metav1.APIResource{
			{
				Name:       "mypod",
				Kind:       "MyPod",
				ShortNames: []string{"mp"},
				Namespaced: true,
				Verbs:      []string{"get", "list"},
			},
		},
	}
	apiList.APIVersion = "v1"
	apiList.Kind = "APIResourceList"
	return apiList
}

func List() any {
	rv := `
{"apiVersion":"apis.abc.com/v1","items":[{"metadata":{"namespace":"default","name":"mypod1"}}],"kind":"MyPodList"}
`
	r := map[string]any{}
	json.Unmarshal([]byte(rv), &r)
	return r
}

func main() {
	group := "apis.abc.com"
	version := "v1"
	root := fmt.Sprintf("/apis/%s/%s", group, version)

	r := gin.Default()
	r.GET(root, func(c *gin.Context) {
		c.JSON(200, ApiResourceList())
	})

	listUrl := fmt.Sprintf("/apis/apis.abc.com/v1/namespaces/default/mypod")
	r.GET(listUrl, func(c *gin.Context) {
		c.JSON(200, List())
	})
	err := r.RunTLS(":8443", "./cmd/certs/aa.crt", "./cmd/certs/aa.key")
	if err != nil {
		panic(err)
	}
}
```

注册aa api

```yaml
apiVersion: apiregistration.k8s.io/v1
kind: APIService
metadata:
  name: v1.apis.abc.com
spec:
  service:
    name: myapi
    namespace: default
    port: 8443
  group: apis.abc.com
  version: v1
  insecureSkipTLSVerify: true
  groupPriorityMinimum: 100
  versionPriority: 100
```

可以看到, 聚合api其实和crd的定义本质上是一样的, 只是crd帮忙把这个资源APIService注册了, 聚合api多了service字段, 从而把请求转发到另一个svc

<img src="/Users/inksnw/Library/Application Support/typora-user-images/image-20241105141648652.png" alt="image-20241105141648652" style="zoom:50%;" />

执行 `kubectl get deploy -v=6` 可以看到请求的地址是 `https://lb.kubesphere.local:6443/apis/apps/v1/namespaces/default/deployments?limit=1`

因此,我们只要实现类似的路由,即可使用kubectl 来查询我们的自定义资源

```bash
➜ kubectl get mypod
NAME     AGE
mypod1   <unknown>
```

这就解决了上文提到的, crd的get请求无法自定义的问题, 就可以透传查询,  避免数据不同步, 还兼容了 `restapi` 和 `sdk`

只要为之实现不同的路由及方法, 就可以自己控制增删改查时的行为, 你想到了什么, 是不是可以实现一个apiserver覆盖pod的路由, 他查询pod时, 我就从两个集群查询pod,然后合并起来, 当有创建请求, 我就分别向两个集群发送创建,  这是不是就实现了多集群管理与多集群分发 ?! 是的, karmada, clusterpedia的核心原理就是这个, 但他们还有更多的逻辑, 我们继续看下文

## 使用独立的 API Server 而不依赖k8s

有没有可能使用k8s的sdk而不依赖k8s, 比如写一个博客后台管理, 希望实现 kubectl create post 就创建一个文章, 还能用informer监听文章资源变动, 但不需要安装k8s, 数据可以存储到mysql 里?

上文已经实现了

- 注册资源路由
- 独立存储(字符串)

只是还需要k8s, 这里我们只要再实现脱离k8s的apiserver, 就搞定了

实现api注册

```go
package options

import (
	"github.com/inksnw/kubestack/pkg/apis/v1alpha1"
	"github.com/inksnw/kubestack/pkg/apiserver/store"
	"github.com/inksnw/kubestack/pkg/sch"
	"github.com/phuslu/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/endpoints/openapi"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericoptions "k8s.io/apiserver/pkg/server/options"
	utilversion "k8s.io/apiserver/pkg/util/version"
)

// GetRecommendedOptions initializes and returns the recommended options for the API server.
func GetRecommendedOptions() *genericoptions.RecommendedOptions {
	rc := genericoptions.NewRecommendedOptions("", sch.Codecs.LegacyCodec(v1alpha1.SchemeGroupVersion))
	rc.SecureServing.BindPort = 443
	rc.CoreAPI = nil
	rc.Admission = nil
	rc.Authorization = nil
	rc.Authentication = nil
	rc.Features.EnablePriorityAndFairness = false
	return rc
}

// GenerateServer creates and configures the GenericAPIServer.
func GenerateServer() *genericapiserver.GenericAPIServer {
	addCoreTypesToScheme()
	if err := v1alpha1.AddToScheme(sch.Scheme); err != nil {
		log.Fatal().Msgf(err.Error())
	}
	apiGroupInfo := createAPIGroupInfo()
	serverConfig := createServerConfig()
	server, err := serverConfig.New("kubestack-apiserver", genericapiserver.NewEmptyDelegate())
	if err != nil {
		log.Fatal().Msgf("Failed to create server: %v", err)
	}
	if err := server.InstallAPIGroup(apiGroupInfo); err != nil {
		log.Fatal().Msgf("Failed to install API group: %v", err)
	}
	return server
}

func addCoreTypesToScheme() {
	metav1.AddToGroupVersion(sch.Scheme, schema.GroupVersion{Version: "v1"})
	sch.Scheme.AddUnversionedTypes(
		schema.GroupVersion{Group: "", Version: "v1"},
		&metav1.Status{},
		&metav1.APIVersions{},
		&metav1.APIGroupList{},
		&metav1.APIGroup{},
		&metav1.APIResourceList{},
	)
}

func createAPIGroupInfo() *genericapiserver.APIGroupInfo {
	agi := genericapiserver.NewDefaultAPIGroupInfo(
		v1alpha1.SchemeGroupVersion.Group,
		sch.Scheme,
		metav1.ParameterCodec, sch.Codecs)
	// 这里注册的是一个虚拟机的资源, 你可以实现当这个资源创建就去调用 /usr/sbin/libvirtd 的socket实现创建虚拟机
	addResource(&agi, &v1alpha1.VirtualMachine{})
	addResource(&agi, &v1alpha1.PhysicalNode{})
	addResource(&agi, &v1alpha1.StoragePool{})
	addResource(&agi, &v1alpha1.NetPool{})

	return &agi
}
func createServerConfig() *genericapiserver.CompletedConfig {
	config := genericapiserver.NewRecommendedConfig(sch.Codecs)
	opt := GetRecommendedOptions()

	if err := opt.ApplyTo(config); err != nil {
		log.Fatal().Msgf("Failed to apply options to config: %v", err)
	}

	config.OpenAPIConfig = genericapiserver.DefaultOpenAPIConfig(
		v1alpha1.GetOpenAPIDefinitions,
		openapi.NewDefinitionNamer(sch.Scheme))
	config.OpenAPIV3Config = genericapiserver.DefaultOpenAPIV3Config(
		v1alpha1.GetOpenAPIDefinitions,
		openapi.NewDefinitionNamer(sch.Scheme))
	config.OpenAPIConfig.Info.Title = "KubeStack"
	config.OpenAPIConfig.Info.Version = "0.1"

	config.EffectiveVersion = utilversion.NewEffectiveVersion("")

	c := config.Complete()
	return &c
}

// addResource adds a resource to the API group info.
func addResource(agi *genericapiserver.APIGroupInfo, resourceObj v1alpha1.ResourceObj) {
	resource := resourceObj.GetGroupResource().Resource
	version := resourceObj.GetGroupVersion().Version
	_, registryStore := store.NewGenericStoreRegistry(sch.Scheme, false, resourceObj)
	if agi.VersionedResourcesStorageMap[version] == nil {
		agi.VersionedResourcesStorageMap[version] = make(map[string]rest.Storage)
	}
	agi.VersionedResourcesStorageMap[version][resource] = registryStore
}

```

实现存储, 这里使用了etcd作为存储, 你完全可以自己实现存储, 存到关系型数据, 比如karmada, clusterpedia的实现

```go
package store

import (
	"context"
	"fmt"
	"github.com/inksnw/kubestack/pkg/apis/v1alpha1"
	"github.com/inksnw/kubestack/pkg/sch"
	"github.com/phuslu/log"
	clientv3 "go.etcd.io/etcd/client/v3"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/storage"
	cacherstorage "k8s.io/apiserver/pkg/storage/cacher"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/apiserver/pkg/storage/storagebackend"
	"k8s.io/apiserver/pkg/storage/storagebackend/factory"
	"path"
)

type RESTStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator
	namespaceScoped          bool
	allowCreateOnUpdate      bool
	allowUnconditionalUpdate bool
}

func (t *RESTStrategy) NamespaceScoped() bool          { return t.namespaceScoped }
func (t *RESTStrategy) AllowCreateOnUpdate() bool      { return t.allowCreateOnUpdate }
func (t *RESTStrategy) AllowUnconditionalUpdate() bool { return t.allowUnconditionalUpdate }

func (t *RESTStrategy) PrepareForCreate(ctx context.Context, obj runtime.Object) {
	metaObj, err := meta.Accessor(obj)
	if err != nil {
		log.Fatal()
		return
	}
	labels := metaObj.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	labels["prepare_create"] = "true"
	metaObj.SetLabels(labels)
}

func (t *RESTStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {}
func (t *RESTStrategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
	return nil
}
func (t *RESTStrategy) WarningsOnCreate(ctx context.Context, obj runtime.Object) []string {
	return nil
}
func (t *RESTStrategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	return nil
}
func (t *RESTStrategy) WarningsOnUpdate(ctx context.Context, obj, old runtime.Object) []string {
	return nil
}
func (t *RESTStrategy) Canonicalize(obj runtime.Object) {}

// EtcdTestServer encapsulates the datastructures needed to start local instance for testing
type EtcdTestServer struct {
	V3Client *clientv3.Client
}

func (e *EtcdTestServer) Terminate() {
	// no-op, server termination moved to test cleanup
}

func NewUnsecuredEtcd3ClientServer() (*EtcdTestServer, *storagebackend.Config) {
	server := &EtcdTestServer{}
	cli, err := clientv3.New(clientv3.Config{
		Endpoints: []string{"localhost:2379"},
	})
	if err != nil {
		panic(err)
	}
	server.V3Client = cli
	config := &storagebackend.Config{
		Type:   "etcd3",
		Prefix: "/registry",
		Transport: storagebackend.TransportConfig{
			ServerList: server.V3Client.Endpoints(),
		},
	}
	return server, config
}

func getAttrs(obj runtime.Object) (labels.Set, fields.Set, error) {
	m, err := meta.Accessor(obj)
	if err != nil {
		panic(err)
	}
	return labels.Set{"name": m.GetName()}, nil, nil
}

func NewGenericStoreRegistry(scheme *runtime.Scheme, hasCacheEnabled bool, resourceObj v1alpha1.ResourceObj) (factory.DestroyFunc, *registry.Store) {
	gr := resourceObj.GetGroupResource()
	Prefix := fmt.Sprintf("/%s", gr.Resource)
	server, sc := NewUnsecuredEtcd3ClientServer()
	strategy := &RESTStrategy{
		scheme,
		names.SimpleNameGenerator,
		resourceObj.NamespaceScoped(),
		false,
		true,
	}

	KeyFunc := func(obj runtime.Object) (string, error) {
		if resourceObj.NamespaceScoped() {
			return storage.NamespaceKeyFunc(Prefix, obj)
		}
		return storage.NoNamespaceKeyFunc(Prefix, obj)
	}

	sc.Codec = sch.Codecs.LegacyCodec(v1alpha1.SchemeGroupVersion)
	storageInterface, dFunc, err := factory.Create(*sc.ForResource(gr), resourceObj.NewFun, resourceObj.NewListFun, Prefix)
	if err != nil {
		panic(err)
	}
	destroyFunc := func() {
		dFunc()
		server.Terminate()
	}
	if hasCacheEnabled {
		config := cacherstorage.Config{
			Storage:        storageInterface,
			Versioner:      storage.APIObjectVersioner{},
			GroupResource:  gr,
			ResourcePrefix: Prefix,
			KeyFunc:        KeyFunc,
			GetAttrsFunc:   getAttrs,
			NewFunc:        resourceObj.NewFun,
			NewListFunc:    resourceObj.NewListFun,
			Codec:          sc.Codec,
		}
		cacher, err := cacherstorage.NewCacherFromConfig(config)
		if err != nil {
			panic(err)
		}

		d := destroyFunc
		storageInterface = cacher
		destroyFunc = func() {
			cacher.Stop()
			d()
		}
	}

	return destroyFunc, &registry.Store{
		NewFunc:                   resourceObj.NewFun,
		NewListFunc:               resourceObj.NewListFun,
		DefaultQualifiedResource:  gr,
		SingularQualifiedResource: gr,
		CreateStrategy:            strategy,
		UpdateStrategy:            strategy,
		DeleteStrategy:            strategy,
		TableConvertor:            resourceObj,
		KeyRootFunc: func(ctx context.Context) string {
			return Prefix
		},
		KeyFunc: func(ctx context.Context, id string) (string, error) {
			namespace, ok := genericapirequest.NamespaceFrom(ctx)
			if !ok {
				return "", fmt.Errorf("namespace is required")
			}
			if resourceObj.NamespaceScoped() {
				return path.Join(Prefix, namespace, id), nil
			}
			return path.Join(Prefix, id), nil
		},
		ObjectNameFunc: func(obj runtime.Object) (string, error) {
			m, err := meta.Accessor(obj)
			if err != nil {
				panic(err)
			}
			return m.GetName(), nil
		},
		PredicateFunc: func(label labels.Selector, field fields.Selector) storage.SelectionPredicate {
			return storage.SelectionPredicate{
				Label: label,
				Field: field,
				GetAttrs: func(obj runtime.Object) (lbs labels.Set, f fields.Set, err error) {
					m, err := meta.Accessor(obj)
					if err != nil {
						panic(err)
					}
					if !resourceObj.NamespaceScoped() {
						f = fields.Set{
							"metadata.name": m.GetName(),
						}
					}
					f = fields.Set{
						"metadata.name":      m.GetName(),
						"metadata.namespace": m.GetNamespace(),
					}
					return m.GetLabels(), f, nil
				},
			}
		},

		Storage: registry.DryRunnableStorage{Storage: storageInterface},
	}
}
```

看一下还缺失的功能

- [x]  自定义资源注册
- [x]  自定义资源存储
- [ ]  实现事件驱动的逻辑触发

目前只是当资源创建时就存储到etcd中, 由于这是一个标准的 k8s风格的 apiserver 你当然可以使用 `client-go` 的代码去创建 `operator` 来实现一个不依赖k8s 又有reconcile的控制器, 所以这个也搞定了
- [x]  实现事件驱动的逻辑触发

如果你只提供查询功能, 把存储换成mysql, 再用informer把数据同步到mysql中那这就是 clusterpedia

如果你把增删改查的逻辑自己实现, 而不使用etcd存储内置的, 就能实现比如当一个资源提交了, 通过一个调度器, 依据集群性能, 向A集群创建1个副本, B集群创建两2个副本, 这就是karmada

## 总结

到这里就结束了, 三种扩展k8s的方式, 说透原理后, 他并不复杂, CRD适用于简单的资源管理，聚合API适用于复杂的API扩展，独立的API Server适用于完全自定义的API实现, 你可以依据需求选择合适的方式
