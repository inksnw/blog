---
title: "Envoy转换grpc为http"
date: 2022-08-17T17:29:33+08:00
tags: ["k8s"]
subs: ["envoy"]
---

文档 https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/grpc_json_transcoder_filter

## 环境准备

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

## transcoder配置

生成descriptor 文件

```bash
 protoc --go_out=. --go-grpc_out=. *.proto  --grpc-gateway_out . --descriptor_set_out=proto.pb
```

创建一个filter
proto_descriptor使用文件

proto_descriptor_bin可以使用由 [grpc-transcoder](https://github.com/AliyunContainerService/grpc-transcoder)生成的base64(未调试通过)


```yaml
apiVersion: networking.istio.io/v1alpha3
kind: EnvoyFilter
metadata:
  name: grpcfilter
  namespace: istio-system
spec:
  workloadSelector:
    labels:
      istio: grpc-ingressgateway
  configPatches:
    - applyTo: HTTP_FILTER
      match:
        context: GATEWAY
        listener:
          filterChain:
            filter:
              name: "envoy.filters.network.http_connection_manager"
              subFilter:
                name: "envoy.filters.http.router"
      patch:
        operation: INSERT_BEFORE
        value:
          name: envoy.grpc_json_transcoder
          typed_config:
            "@type": "type.googleapis.com/envoy.extensions.filters.http.grpc_json_transcoder.v3.GrpcJsonTranscoder"
            proto_descriptor:"/etc/proto.pb"
            services:
              - ProdService
            print_options:
              add_whitespace: true
              always_print_primitive_fields: true
              always_print_enums_as_ints: false
              preserve_proto_field_names: false
```

proto_descriptor 的文件放到哪里,还未找到方法
