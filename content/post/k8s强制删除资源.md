---
title: "K8s强制删除资源"
date: 2025-01-17T19:20:57+08:00
tags: ["k8s"]
---

最近换了工作, 博客好久没更新了, 最近也确实没有什么题材写, 水一个强制删除资源的吧

在某些情况下，资源可能由于某些原因无法正常删除。这时，我们需要使用一些强制删除的方法。

## 强制删除 Pod

在 Kubernetes 中，Pod 的删除通常是一个优雅的过程。Kubernetes 会发送 SIGTERM 信号给 Pod 中的容器，并等待一段时间（默认 30 秒）让容器完成清理工作。如果 Pod 卡在终止状态，可以使用以下命令强制删除：

```bash
kubectl delete pod <pod-name> -n <namespace> --force --grace-period=0
```

- `--force`：强制删除 Pod，跳过正常的终止流程。
- `--grace-period=0`：将优雅删除时间设置为 0 秒，立即终止 Pod。

## 强制删除资源的 Finalizers

Kubernetes 中的 Finalizers 就是个锁用于其它程序做一些清理工作, 当执行删除时, 只会标记上删除时间戳, 只有Finalizers 变为空后才会真的删除, 可以手动清除 Finalizers 来强制删除资源。

```bash
kubectl patch pod <pod> -p '{"metadata":{"finalizers":null}}'
```

## 强制删除 Namespace

Namespace 的`finalizers`在他的`spec`中, 可能是由于历史原因, 或者特意设计, 像上文那样清理metadata中的 `finalizers`是无效的,  需要这样操作

```bash
kubectl proxy --address='0.0.0.0' --port=8001
```

```bash
curl -k -H "Content-Type: application/json" -X PUT --data-binary @- http://127.0.0.1:8001/api/v1/namespaces/<namespace>/finalize <<EOF
{
  "apiVersion": "v1",
  "kind": "Namespace",
  "metadata": {
    "name": "<namespace>"
  },
  "spec": {
    "finalizers": []
  }
}
EOF
```

## 使用 etcd 直接删除数据

如果以上方法都无法解决问题，可以直接通过 etcd 删除数据。

```bash
alias etcdctl='etcdctl --key=/etc/ssl/etcd/ssl/admin-vm87-key.pem --cert=/etc/ssl/etcd/ssl/admin-vm87.pem --cacert=/etc/ssl/etcd/ssl/ca.pem --endpoints 192.168.50.28:2379'
#查看节点
ETCDCTL_API=3 etcdctl -w table member list
# 查找key
etcdctl get / --prefix --keys-only | grep ff
# 删除
etcdctl del /registry/namespaces/ff
```

直接操作 etcd 是高风险操作，可能导致集群状态不一致。

