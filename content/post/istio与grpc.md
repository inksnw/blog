---
title: "Istio与grpc"
date: 2022-08-16T15:38:47+08:00
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
  	# 使用grpc专用的gateway
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
