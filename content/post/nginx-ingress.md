---
title: "nginx-ingress"
date: 2022-08-12T10:27:37+08:00
tags: ["k8s"]
---

## 安装nginx-ingress

参考文档: https://kubernetes.github.io/ingress-nginx/deploy/

```bash
➜ kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.3.0/deploy/static/provider/cloud/deploy.yaml
```

修改服务模式为NodePort,**注意**,要同时删除掉`externalTrafficPolicy: Local`,会重置为模式`externalTrafficPolicy: Cluster`

```bash
➜ kubectl edit svc ingress-nginx-controller -n  ingress-nginx
```

## 什么是external-traffic-policy

在k8s的Service对象（申明一条访问通道）中，有一个“externalTrafficPolicy”字段可以设置。有2个值可以设置：Cluster或者Local。

- Cluster表示：流量可以转发到其他节点上的Pod。

- Local表示：流量只发给本机的Pod。

<img src="https://img2020.cnblogs.com/blog/1464583/202007/1464583-20200707174740496-463162050.png" alt="img" style="zoom: 67%;" />



## 这2种模式有什么区别

### Cluster

注：这个是默认模式，Kube-proxy不管容器实例在哪，公平转发。

Kube-proxy转发时会替换掉报文的源IP。即：容器收的报文，源IP地址，已经被替换为上一个转发节点的了。

原因是Kube-proxy在做转发的时候，会做一次SNAT (source network address translation)，所以源IP变成了节点1的IP地址。

ps：snat确保回去的报文可以原路返回，不然回去的路径不一样，客户会认为非法报文的。（我发给张三的，怎么李四给我回应？丢弃！）

这种模式好处是负载均衡会比较好，因为无论容器实例怎么分布在多个节点上，它都会转发过去。当然，由于多了一次转发，性能会损失一丢丢。

### Local

这种情况下，只转发给本机的容器，绝不跨节点转发。

Kube-proxy转发时会保留源IP。即：容器收到的报文，看到源IP地址还是用户的。

缺点是负载均衡可能不是很好，因为一旦容器实例分布在多个节点上，它只转发给本机，不跨节点转发流量。当然，少了一次转发，性能会相对好一丢丢。

注：这种模式下的Service类型只能为外部流量，即：LoadBalancer 或者 NodePort 两种，否则会报错。

同时，由于本机不会跨节点转发报文，所以要想所有节点上的容器有负载均衡，就需要上一级的Loadbalancer来做了。

<img src="https://img2020.cnblogs.com/blog/1464583/202007/1464583-20200707174845450-1319538618.png" alt="img" style="zoom: 67%;" />

不过流量还是会不太均衡，如上图，Loadbalancer看到的是2个后端（把节点的IP），每个Node上面几个Pod对Loadbalancer来说是不知道的。

想要解决负载不均衡的问题：可以给Pod容器设置反亲和，让这些容器平均的分布在各个节点上（不要聚在一起）。

```yaml
affinity:
  podAntiAffinity:
    preferredDuringSchedulingIgnoredDuringExecution:
    - weight: 100
      podAffinityTerm:
        labelSelector:
          matchExpressions:
           - key: k8s-app
             operator: In
             values:
             - my-app
        topologyKey: kubernetes.io/hostname
```

### 两种模式该怎么选

要想性能（时延）好，当然应该选 Local 模式喽，毕竟流量转发少一次SNAT嘛。

不过注意，选了这个就得考虑好怎么处理好负载均衡问题（ps：通常我们使用Pod间反亲和来达成）。

如果你是从外部LB接收流量的，那么使用：Local模式 + Pod反亲和，一般是足够的

## 快速创建一个`deploy`与`svc`

```bash
kubectl create deployment demo --image=httpd --port=80
kubectl expose deployment demo
```

创建一个`ingress`

```bash
kubectl create ingress demo-localhost --class=nginx   --rule="demo.localdev.me/*=demo:80"
# 查看ingress的nodePort端口
kubectl get svc ingress-nginx-controller  -n ingress-nginx -o json|jq .spec.ports[0].nodePort
```

访问

```bash
curl -H 'Host:demo.localdev.me' http://192.168.50.40:30443
# 结果
<html><body><h1>It works!</h1></body></html>
```

## 查看nginx配置

```bash
 kubectl exec -it ingress-nginx-controller-b4fcbcc8f-gl8bq  -n ingress-nginx -- /bin/bash
 cat /etc/nginx/nginx.conf
```

发现其实本质就是配置了nginx的域名转发

<img src="http://inksnw.asuscomm.com:3001/blog/nginx-ingress_bdc7364de5da8c2f4e185ea3e55ef455.png" alt="image-20220922213751633" style="zoom:50%;" />
