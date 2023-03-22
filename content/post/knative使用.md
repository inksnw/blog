---
title: "Knative使用"
date: 2023-03-20T17:49:34+08:00
draft: true
---

### 安装istio

参考 [Istio简单使用](http://inksnw.asuscomm.com:3001/post/istio%E7%AE%80%E5%8D%95%E4%BD%BF%E7%94%A8/)

### 安装knative

[参考文档](https://knative.dev/docs/install/yaml-install/eventing/install-eventing-with-yaml/#verifying-image-signatures)

```
kubectl apply -f https://github.com/knative/eventing/releases/download/knative-v1.9.7/eventing-crds.yaml
kubectl apply -f https://github.com/knative/eventing/releases/download/knative-v1.9.7/eventing-core.yaml
```

查看安装的pod

```bash
root@node1:~# kubectl get pod -n  knative-eventing
NAME                                   READY   STATUS    RESTARTS   AGE
eventing-controller-764fc5b44f-6plg5   1/1     Running   0          3m22s
eventing-webhook-5b4777f75f-6swt2      1/1     Running   0          3m22s
```

### 安装metalb

参考[使用metalb实现内网负载均衡](http://inksnw.asuscomm.com:3001/post/%E8%AE%BF%E9%97%AEpod%E7%9A%84%E6%96%B9%E5%BC%8F/#%E4%BD%BF%E7%94%A8metalb%E5%AE%9E%E7%8E%B0%E5%86%85%E7%BD%91%E8%B4%9F%E8%BD%BD%E5%9D%87%E8%A1%A1)
