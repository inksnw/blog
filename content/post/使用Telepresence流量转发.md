---
title: "使用Telepresence流量转发"
date: 2022-11-03T19:45:56+08:00
tags: ["k8s"]
---

## 简介

yaml工程师在开发时想从本机直连k8s集群的服务,或者将集群内的请求流量劫持到本机进行处理, 都很麻烦,[Telepresence](https://www.telepresence.io/) 这个工具可以帮助做到这点.

Telepresence 会在远程集群中运行的现有应用程序容器旁边安装流量代理 sidecar。 当它捕获进入 Pod 的所有流量请求时，不是将其转发到远程集群中的应用程序， 而是路由所有流量（当创建[全局拦截器](https://www.getambassador.io/docs/telepresence/latest/concepts/intercepts/#global-intercept)时） 或流量的一个子集（当创建[自定义拦截器](https://www.getambassador.io/docs/telepresence/latest/concepts/intercepts/#personal-intercept)时） 到本地开发环境。

## 安装

参考[安装文档](https://www.telepresence.io/docs/latest/install/)

本机安装

```bash
brew install datawire/blackbird/telepresence
```

k8s集群安装

```bash
helm repo add datawire  https://app.getambassador.io
helm repo update
kubectl create namespace ambassador
helm install traffic-manager --namespace ambassador datawire/telepresence
```

查看安装的资源

```bash
root@node1:~# kubectl get all -n ambassador
NAME                                   READY   STATUS    RESTARTS   AGE
pod/traffic-manager-5cd74c95f6-t9kbg   1/1     Running   0          18s

NAME                      TYPE        CLUSTER-IP      EXTERNAL-IP   PORT(S)              AGE
service/agent-injector    ClusterIP   10.233.25.124   <none>        443/TCP              19s
service/traffic-manager   ClusterIP   None            <none>        8081/TCP,15766/TCP   19s

NAME                              READY   UP-TO-DATE   AVAILABLE   AGE
deployment.apps/traffic-manager   1/1     1            1           19s

NAME                                         DESIRED   CURRENT   READY   AGE
replicaset.apps/traffic-manager-5cd74c95f6   1         1         1       19s
```

## 连接集群

安装完成后，需要使用下面的命令来连接到 k8s 集群。

```bash
telepresence connect
# 查看状态
telepresence status
```

## 访问集群内的服务

```bash
# 创建一个简单服务
kubectl create deployment demo --image=nginx --port=80
kubectl expose deployment demo
# 从本地电脑访问
curl http://demo.default:80
```

## 劫持集群内的服务流量到本机

对于某些线上服务的疑难杂症，可以直接把发给他的流量劫持到本机，这样就能在本机直接边改代码边在集群中进行调试

```bash
# telepresence intercept $SERVICE_NAME --port $LOCAL_PORT:REMOTE_PORT
$ telepresence intercept demo --port 8000:80 -n default

Using Deployment demo
intercepted
    Intercept name         : demo-default
    State                  : ACTIVE
    Workload kind          : Deployment
    Destination            : 127.0.0.1:8000
    Service Port Identifier: 80
    Volume Mount Error     : sshfs is not installed on your local machine
    Intercepting           : all TCP requests
```

在本地开一个服务

```
python3 -m http.server
```

在集群上访问原服务

```bash
$ kubectl get svc
NAME         TYPE        CLUSTER-IP      EXTERNAL-IP   PORT(S)   AGE
demo         ClusterIP   10.233.63.220   <none>        80/TCP    7m32s
$ curl 10.233.63.220:80
```

会发现集群上的服务已经由 nginx 变成了本机电脑的 python 文件服务

列出服务的劫持状态：

```bash
$ telepresence list -n default
demo: intercepted
    Intercept name         : demo-default
    State                  : ACTIVE
    Workload kind          : Deployment
    Destination            : 127.0.0.1:8000
    Service Port Identifier: 80
    Intercepting           : all TCP requests
```

查看pod信息

```bash
# 原信息
NAME                  READY   STATUS    RESTARTS   AGE
demo-859dc84b-9q9nn   1/1     Running   0          2m25s
# 劫持以后, pod新建一个,在新的Pod中会多一个 traffic-agent 的容器,对流量进行拦截
NAME                    READY   STATUS    RESTARTS   AGE
demo-69949dbc65-p9qvk   2/2     Running   0          35s
```

卸载服务的劫持状态

```bash
$ telepresence uninstall --agent demo -n default
$ telepresence list -n default
demo: ready to intercept (traffic-agent not yet installed)
```

## 关闭集群连接

关闭到集群的连接，将本地的网络恢复原状：

```bash
$ telepresence quit
```

