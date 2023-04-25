---
title: "Knative使用"
date: 2023-03-20T17:49:34+08:00
draft: true
---

### 安装istio

参考 [Istio简单使用](http://inksnw.asuscomm.com:3001/post/istio%E7%AE%80%E5%8D%95%E4%BD%BF%E7%94%A8/)

### 安装knative

安装eventing

[参考文档](https://knative.dev/docs/install/yaml-install/eventing/install-eventing-with-yaml/#verifying-image-signatures)

```
kubectl apply -f https://github.com/knative/eventing/releases/download/knative-v1.9.7/eventing-crds.yaml
kubectl apply -f https://github.com/knative/eventing/releases/download/knative-v1.9.7/eventing-core.yaml
```

安装Serving

```bash
kubectl apply -f https://github.com/knative/serving/releases/download/knative-v1.9.2/serving-crds.yaml
kubectl apply -f https://github.com/knative/serving/releases/download/knative-v1.9.2/serving-core.yaml
```

查看安装的pod

```bash
root@node1:~# kubectl get pod -n  knative-eventing
NAME                                   READY   STATUS    RESTARTS   AGE
eventing-controller-764fc5b44f-6plg5   1/1     Running   0          3m22s
eventing-webhook-5b4777f75f-6swt2      1/1     Running   0          3m22s
root@node1:~# kubectl get pod -n knative-serving
NAME                                     READY   STATUS    RESTARTS   AGE
activator-777654d44b-hd9vm               1/1     Running   0          27s
autoscaler-6fc786cb96-z28zz              1/1     Running   0          26s
controller-596d9b6cc8-ctf6c              1/1     Running   0          26s
domain-mapping-984bc99d9-796nb           1/1     Running   0          26s
domainmapping-webhook-799ffdd45f-92l79   1/1     Running   0          26s
webhook-58b6b89c55-hhcgv                 1/1     Running   0          26s
```

### 创建一个服务

```yaml
apiVersion: serving.knative.dev/v1
kind: Service
metadata:
  name: ksvc1
spec:
  template:
    spec:
      containers:
        - image: nginx:1.18-alpine
          ports:
            - containerPort: 80
```

查看创建的资源

```
kubectl get pod
NAME                                      READY   STATUS    RESTARTS   AGE
ksvc1-00001-deployment-7cbfcf76bb-sc4fs   2/2     Running   0          39s
```

查看ksvc

```
root@node1:~# kubectl get ksvc
NAME    URL                                      LATESTCREATED   LATESTREADY   READY     REASON
ksvc1   http://ksvc1.default.svc.cluster.local   ksvc1-00001     ksvc1-00001   Unknown   IngressNotConfigured
```

显示没有`READY`, 下面配置一下lb

### 配置openelb

参考
