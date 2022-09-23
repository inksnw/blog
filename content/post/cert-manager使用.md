---
title: "Cert Manager使用"
date: 2022-09-23T14:11:28+08:00
tags: ["k8s"]
---

## cert-manager 工作原理

cert-manager 部署到 Kubernetes 集群后，它会 watch 它所支持的 CRD 资源，我们通过创建 CRD 资源来指示 cert-manager 为我们签发证书并自动续期:

<img src="http://inksnw.asuscomm.com:3001/blog/cert-manager使用_f2fe5a87ab3ea068935705e6a1fd7435.png" alt="image-20220923141435361" style="zoom: 50%;" />

几个关键的资源:

- Issuer/ClusterIssuer: 用于指示 cert-manager 用什么方式签发证书，本文主要讲解签发免费证书的 ACME 方式。ClusterIssuer 与 Issuer 的唯一区别就是 Issuer 只能用来签发自己所在 namespace 下的证书，ClusterIssuer 可以签发任意 namespace 下的证书。
- Certificate: 用于告诉 cert-manager 我们想要什么域名的证书以及签发证书所需要的一些配置，包括对 Issuer/ClusterIssuer 的引用。

### 安装 cert-manager

```bash
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.9.1/cert-manager.yaml
```

创建Issuer

```yaml
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: my-issue
  namespace: default
spec:
  selfSigned: {}
```

创建证书

```yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: ngx-tls
  namespace: default
spec:
  secretName: gin-tls
  duration: 5h
  renewBefore: 2h
  dnsNames:
  - inksnw.test.com
  issuerRef:
    name: my-issue
```

会生成一个名为`gin-tls`的secret

```
kubectl get secret
```

### 验证

创建一个简单nginx-ingress与相应服务

```bash
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.3.1/deploy/static/provider/cloud/deploy.yaml
#修改服务模式为NodePort,注意,要同时删除掉externalTrafficPolicy: Local,会重置为模式externalTrafficPolicy: Cluster
kubectl edit svc ingress-nginx-controller -n  ingress-nginx
kubectl create deployment demo --image=nginx --port=80
kubectl expose deployment demo
kubectl create ingress demo-localhost --class=nginx   --rule="inksnw.test.com/*=demo:80"
# 查看nginx-ingress svc 的nodePort端口
kubectl get svc ingress-nginx-controller  -n ingress-nginx -o json|jq .spec.ports[0].nodePort
# 访问测试
curl -H "Host:inksnw.test.com" http://192.168.50.40:32599
```

添加证书

```yaml
spec:
  ingressClassName: nginx
  tls:
  - hosts:
    - inksnw.test.com
    secretName: gin-tls
```



