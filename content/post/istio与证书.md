---
title: "istio与证书"
date: 2022-08-16T17:00:50+08:00
tags: ["k8s"]
subs: ["istio"]
---

## 环境准备

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: mynginx
  namespace: myistio
  labels:
    app: mynginx
spec:
  replicas: 1
  template:
    metadata:
      name: mynginx
      labels:
        app: mynginx
    spec:
      containers:
        - name: mynginx
          image: nginx
          imagePullPolicy: IfNotPresent
      restartPolicy: Always
  selector:
    matchLabels:
      app: mynginx
---
apiVersion: v1
kind: Service
metadata:
  name: mynginx
  namespace: myistio
spec:
  selector:
    app: mynginx
  ports:
    - port: 80
  type: NodePort
---
apiVersion: networking.istio.io/v1beta1
kind: Gateway
metadata:
  name: mynginx-gateway
  namespace: myistio
spec:
  selector:
    istio: ingressgateway
  servers:
    - port:
        number: 80
        name: http
        protocol: HTTP
      hosts:
        - nginx.inksnw.com
    - port:
        number: 443
        name: https
        protocol: HTTPS
      tls:
        mode: SIMPLE
        serverCertificate: /etc/istio/ingressgateway-certs/tls.crt
        privateKey: /etc/istio/ingressgateway-certs/tls.key
      hosts:
        - nginx.inksnw.com

---
apiVersion: networking.istio.io/v1beta1
kind: VirtualService
metadata:
  name: mynginx
  namespace: myistio
spec:
  hosts:
    - nginx.inksnw.com
  gateways:
    - mynginx-gateway
  http:
    - match:
        - uri:
            prefix: "/"
      route:
        - destination:
            host: mynginx.myistio.svc.cluster.local
            port:
              number: 80
---
```

访问测试

```bash
curl http://nginx.inksnw.com:32016/
```

## 证书配置

使用 certstrap 快速生成证书

```bash
brew install certstrap
certstrap init --common-name "ExampleCA" --expires "20 years"
certstrap request-cert -cn server  -domain "nginx.inksnw.com"
certstrap sign server --CA ExampleCA
certstrap request-cert -cn client
certstrap sign client --CA ExampleCA
```

文档 https://istio.io/latest/zh/docs/tasks/traffic-management/ingress/secure-ingress-mount/#configure-a-TLS-ingress-gateway-with-a-file-mount-based-approach

创建secret

```bash
kubectl create -n istio-system secret tls istio-ingressgateway-certs --key server.key --cert server.crt
```

>该 secret **必须**在 `istio-system` 命名空间下，且名为 `istio-ingressgateway-certs`

查看证书文件

```bash
kubectl exec -it -n istio-system $(kubectl -n istio-system get pods -l istio=ingressgateway -o jsonpath='{.items[0].metadata.name}') -- ls -al /etc/istio/ingressgateway-certs
```

## 访问测试

```bash
curl https://nginx.inksnw.com:31754/
```
