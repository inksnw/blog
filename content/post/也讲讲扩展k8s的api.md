---
title: "也讲讲扩展k8s的api"
date: 2024-10-29T16:25:46+08:00
tags: ["k8s"]
---

Kubernetes 可以通过多种方式扩展其 API。本文将深入探讨三种主要的扩展方式：自定义资源定义（Custom Resource Definitions, CRD）、聚合 API（API Aggregation）以及独立的 API Server。这里通过案例分析每种方式的优缺点，帮助你选择最适合你需求的扩展方式。

## 使用 CRD 扩展 Kubernetes API

定义一个CRD名为 `DatabaseConfig` 的资源类型，如下所示：

```yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: databaseconfigs.example.com
spec:
  group: example.com
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
                replicas:
                  type: integer
                storage:
                  type: string
  scope: Namespaced
  names:
    plural: databaseconfigs
    singular: databaseconfig
    kind: DatabaseConfig
    shortNames:
    - dbcfg
```

定义好 CRD 后，你可以在 Kubernetes 中创建和管理 `DatabaseConfig` 资源：

```yaml
apiVersion: example.com/v1
kind: DatabaseConfig
metadata:
  name: my-database
spec:
  replicas: 3
  storage: "10Gi"
```

假设在做一个数据管理平台, 要管理多种数据库实现, 那crd有什么问题

1. 有新的crd学习成本, 数据库实现可能是第三方的, 他们不愿意改成你设计的crd
2. 使用一个字段存储具体crd的字符串,套娃具体数据库的crd
3. 但是如果用户绕过直接改底层的crd会有数据同步问题, 就需要做数据同步
4. 如果是一个api调用, 有自己的服务端, 可以透传查询, 但这需要一个独立的后端api服务

>  局限的原因, crd只能在controller层实现创建/删除/修改逻辑, 而查询是标准的k8s api 无法自定义



## 使用聚合 API 扩展 Kubernetes API

聚合 API（API Aggregation）是 Kubernetes 提供的一种更高级的 API 扩展方式。通过聚合 API，用户可以编写一个独立的 API Server，并将其注册到 Kubernetes 的 API Server 中。Kubernetes 会将对该 API Server 的请求转发到该 API Server 进行处理。

写一段最简单的api代码

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

执行 `kubectl get deploy -v=6` 可以发现请求的实际地址是`https://lb.kubesphere.local:6443/apis/apps/v1/namespaces/default/deployments?limit=1`

因此,我们只要实现`/apis/apis.abc.com/v1/namespaces/default/mypod`的路由,即可实现使用kubectl 来查询我们的自定义资源

```bash
➜ kubectl get mypod
NAME     AGE
mypod1   <unknown>
```

这就解决了上文提到的, crd的get请求无法自定义的问题, 就可以透传查询, 避免数据不同步

兼容kubectl工具, 

## 使用独立的 API Server 而不依赖k8s

有没有可能使用k8s的sdk而不依赖k8s, 比如写一个博客后台管理, 希望实现 kubectl create post 就创建一个文章, 还能用informer监听文章资源变动, 但不需要安装k8s, 当然这里多少有点看什么都是钉子的意思了, 但有些场景下可能确实有用

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



实现存储, 这里使用了etcd和k8s内的代码, 你完全可以自己实现存储, 存到关系型数据, 比如karmada, clusterpedia的实现

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



