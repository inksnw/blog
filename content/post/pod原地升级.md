---
title: "Pod原地升级"
date: 2023-09-18T15:17:12+08:00
tags: ["k8s"]
---

## 什么是原地升级？

原地升级（In-place update）是一种 Kubernetes 中的 Pod 升级方式，这种升级方式可以更新 Pod 中某一个或多个容器的镜像版本，而不影响 Pod 中其余容器的运行，同时保持 Pod 的网络和存储状态不变。

Kubernetes 原生工作负载，不论是 Deployment、StatefulSet 还是 Pod 本身，如果你想升级 Pod 中的镜像，那么 Kubernetes 就会重新销毁该 Pod 并重新调度并创建一个 Pod，对于 StatefulSet 虽然可以保持原有 Pod 的名字，但是实际 **UID** 及 **Pod IP** 都将发生改变。

网上流传的方法,包括 `OpenKruise` 都是更新一下image内容, 就能实现 container 重建/启, 而不影响**UID** 及 **Pod IP**

那这个方法是最解么, 原理是什么

## 原理分析

### sandbox重建

我们知道, Pod IP 的网络名称空间等都是依附于sandbox容器的, 只要sandbox重建, 这些信息都会重建

核心逻辑都在`kubernetes-1.26.5/pkg/kubelet/kuberuntime/kuberuntime_manager.go` 的 **678** 行 `func (m *kubeGenericRuntimeManager) SyncPod(...)` 中, 在一开始会检测`func PodSandboxChanged(pod *v1.Pod, podStatus *kubecontainer.PodStatus) (bool, uint32, string)` 

<img src="http://inksnw.asuscomm.com:3001/blog/pod原地升级_c957f90c907a280f9b9a2666154c5796.png" alt="image-20230918153847126" style="zoom:50%;" />

可以看到有几点判断条件,

- 启动的sandbox数量大于 1
- sandbox不是ready
- 网络信息不同

改动了容器的image并不在这个范围内, 因此并不会重建sandbox, 但是这还没不能说明会重启业务container, 我们继续往下看

### 重启container

在`kubernetes-1.26.5/pkg/kubelet/kuberuntime/kuberuntime_manager.go` 的 **900** 行, 可以看到`podContainerChanges.ContainersToStart`中的容器会被启动, 

![image-20230918154534223](http://inksnw.asuscomm.com:3001/blog/pod原地升级_5fbc7fdb7fd567c756473841aac9584e.png)

那这个数组的数据是如何加入的, 在625 行看到, 当 `containerChanged(&container, containerStatus)` 为真时, 即会被加入

<img src="http://inksnw.asuscomm.com:3001/blog/pod原地升级_571dfacd2ee32a7d3f0f03d2983b3111.png" alt="image-20230918155019412" style="zoom:50%;" />

继续看`containerChanged`的逻辑, 这时就比较简单了

```go
func containerChanged(container *v1.Container, containerStatus *kubecontainer.Status) (uint64, uint64, bool) {
	expectedHash := kubecontainer.HashContainer(container)
	return expectedHash, containerStatus.Hash, containerStatus.Hash != expectedHash
}

func HashContainer(container *v1.Container) uint64 {
	hash := fnv.New32a()
	// Omit nil or empty field when calculating hash value
	// Please see https://github.com/kubernetes/kubernetes/issues/53644
	containerJSON, _ := json.Marshal(container)
	hashutil.DeepHashObject(hash, containerJSON)
	return uint64(hash.Sum32())
}
```

当`v1.Container` 的hash值变了, 就会判断为真, 查看`v1.Container`的数据结构, 发现其中有很多字段的, 但由于apiserver只允许更改`image`字段, 因此 `OpenKruise` 等工具就通过修改image字段实现 container 更新/重启而不重建sandbox, 感觉不是太优雅, 但是怎么说呢, 也算是种feature吧 ~

> 改其它字段会提示: # * spec: Forbidden: pod updates may not change fields other than `spec.containers[*].image`, `spec.initContainers[*].image`, `spec.activeDeadlineSeconds`, `spec.tolerations` (only additions to existing tolerations) or `spec.terminationGracePeriodSeconds` (allow it to be set to 1 if it was previously negative)

```go
type Container struct {
	Name                     string                   `json:"name" protobuf:"bytes,1,opt,name=name"`
	Image                    string                   `json:"image,omitempty" protobuf:"bytes,2,opt,name=image"`
	Command                  []string                 `json:"command,omitempty" protobuf:"bytes,3,rep,name=command"`
	Args                     []string                 `json:"args,omitempty" protobuf:"bytes,4,rep,name=args"`
	WorkingDir               string                   `json:"workingDir,omitempty" protobuf:"bytes,5,opt,name=workingDir"`
	...
}
```

