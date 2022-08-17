---
title: "Istio与grpc"
date: 2022-08-16T15:38:47+08:00
tags: ["k8s"]
subs: ["istio"]
---

准备一套grpc示例代码,[参考]( http://inksnw.asuscomm.com:3001/post/grpc示例/)

## istio配置

查看gateway配置文件

```bash
istioctl profile dump --config-path components demo
```

### 创建grpc专用gateway

```yaml
apiVersion: install.istio.io/v1alpha1
kind: IstioOperator
spec:
  components:
    egressGateways:
      - enabled: true
        name: istio-egressgateway
    ingressGateways:
      - enabled: true
        name: istio-ingressgateway
      - enabled: true
        label:
          app: grpc-ingressgateway
          istio: grpc-ingressgateway
        name: grpc-ingressgateway
```

重新安装

```
istioctl install -f grpc.yaml
```

修改两个 ingressgateway 为NodePort模式

```bash
kubectl edit svc istio-ingressgateway -n istio-system
kubectl edit svc grpc-ingressgateway -n istio-system
```

修改hosts添加域名解析

```bash
192.168.50.40 grpc.inksnw.com
```

### Gateway

```yaml
apiVersion: networking.istio.io/v1beta1
kind: Gateway
metadata:
  name: grpc-gateway
  namespace: myistio
spec:
  selector:
    istio: grpc-ingressgateway 
  servers:
    - port:
        number: 80
        name: grpc
        protocol: GRPC
      hosts:
        - "*"
```

### VirtualService

```yaml
apiVersion: networking.istio.io/v1beta1
kind: VirtualService
metadata:
  name: grpcvs
  namespace: myistio
spec:
  hosts:
    - "*"
  gateways:
    - grpc-gateway
  http:
    -  route:
       - destination:
          host: gprodsvc
          port:
            number: 80
```

### 业务yaml

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gprodapi
  namespace: myistio
spec:
  selector:
    matchLabels:
      app: gprod
  replicas: 1
  template:
    metadata:
      labels:
        app: gprod
        version: v1
    spec:
      nodeName: node1
      containers:
        - name: gprodapi
          image: alpine:3.12
          imagePullPolicy: IfNotPresent
          command: ["/app/gprods"]
          volumeMounts:
            - name: appdata
              mountPath: /app
          ports:
            - containerPort: 8080
      volumes:
        - name: appdata
          hostPath:
            path: /root/build
---
apiVersion: v1
kind: Service
metadata:
  name: gprodsvc
  namespace: myistio
  labels:
    app: gprod
spec:
  type: ClusterIP
  ports:
    - port: 80
      targetPort: 8080
      protocol: TCP
      name: grpc
  selector:
    app: gprod

```

## 访问测试

### 代码访问

```go
package main

import (
	"context"
	"fmt"
	"google.golang.org/grpc"
	"grpc/pbfiles"
	"log"
)

func main() {
	//client, err := grpc.DialContext(context.Background(), ":8080", grpc.WithInsecure())
	client, err := grpc.DialContext(context.Background(), "grpc.inksnw.com:31882", grpc.WithInsecure())
	if err != nil {
		log.Fatal(err)
	}
	rsp := &pbfiles.ProdResponse{}
	err = client.Invoke(context.Background(),
		"/ProdService/GetProd",
		&pbfiles.ProdRequest{ProdId: 123}, rsp)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(rsp.Result)

}
```

### 工具访问

推荐一个很好用的grpc图形工具

https://github.com/bloomrpc/bloomrpc

![Snipaste_2022-08-16_16-33-11](http://inksnw.asuscomm.com:3001/blog/Snipaste_2022-08-16_16-33-11.jpg)

## 单向证书

### 生成自签证书

```bash
certstrap init --common-name "ExampleCA" --expires "20 years" # 生成根证书
certstrap request-cert -cn server  -domain "*.inksnw.com"  # 服务端证书
certstrap request-cert -cn client  														# 客户端证书
certstrap sign client --CA ExampleCA
certstrap sign server --CA ExampleCA
```

### 修改gateway文件

```yaml
apiVersion: networking.istio.io/v1beta1
kind: Gateway
metadata:
  name: grpc-gateway
  namespace: myistio
spec:
  selector:
    istio: grpc-ingressgateway
  servers:
    - port:
        number: 80
        name: grpc
        protocol: HTTPS
      tls:
        mode: SIMPLE
        credentialName: grpc-ingressgateway-certs
      hosts:
        - "*"
```

### 创建secret

```bash
kubectl create -n istio-system secret tls grpc-ingressgateway-certs --key=server.key  --cert=server.crt
```

### 访问测试

```go
package main

import (
	"context"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"grpc/pbfiles"
	"log"
	"os"
	"path"
)

func main() {
	str, _ := os.Getwd()
	creds, err := credentials.NewClientTLSFromFile(path.Join(str, "server.crt"),
		"grpc.inksnw.com")
	if err != nil {
		log.Fatal(err)
	}
	client, err := grpc.DialContext(context.Background(),
		"grpc.inksnw.com:31882",
		grpc.WithTransportCredentials(creds))
	rsp := &pbfiles.ProdResponse{}
	err = client.Invoke(context.Background(),
		"/ProdService/GetProd",
		&pbfiles.ProdRequest{ProdId: 123}, rsp)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(rsp.Result)

}
```

工具访问

![Snipaste_2022-08-16_22-53-35](http://inksnw.asuscomm.com:3001/blog/istio与grpc_20220817012633.png)

> 因为Gateway配置的是80端口,因此使用grpc ingress的对应 NodePort 端口
>
> Gateway 的协议配置https,会自动识别grpc

## 双向证书

### 导入cacert

```bash
kubectl create -n istio-system secret generic grpc-ingressgateway-certs --from-file=key=server.key --from-file=cert=server.crt --from-file=cacert=ExampleCA.crt
```

### 修改gateway为双向认证

```yaml
apiVersion: networking.istio.io/v1beta1
kind: Gateway
metadata:
  name: grpc-gateway
  namespace: myistio
spec:
  selector:
    istio: grpc-ingressgateway
  servers:
    - port:
        number: 80
        name: grpc
        protocol: HTTPS
      tls:
        mode: MUTUAL
        credentialName: grpc-ingressgateway-certs
      hosts:
        - "*"
```

### 访问测试

```go
package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"grpc/pbfiles"
	"log"
	"os"
)

func main() {
	cert, err := tls.LoadX509KeyPair("../out/client.crt", "../out/client.key")
	if err != nil {
		log.Fatal(err)
	}
	certPool := x509.NewCertPool()
	ca, err := os.ReadFile("../out/ExampleCA.crt")
	if err != nil {
		log.Fatal(err)
	}
	certPool.AppendCertsFromPEM(ca)
	creds := credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{cert},
		ServerName:   "grpc.inksnw.com",
		RootCAs:      certPool,
	})
	client, err := grpc.DialContext(context.Background(),
		"grpc.inksnw.com:31882",
		grpc.WithTransportCredentials(creds))

	if err != nil {
		log.Fatal(err)
	}
	rsp := &pbfiles.ProdResponse{}
	err = client.Invoke(context.Background(),
		"/ProdService/GetProd",
		&pbfiles.ProdRequest{ProdId: 123}, rsp)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(rsp.Result)

}
```

