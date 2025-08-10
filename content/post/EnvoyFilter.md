---
title: "EnvoyFilter"
date: 2022-08-17T15:28:29+08:00
tags: ["k8s"]
subs: ["envoy"]
---

文档 https://istio.io/latest/docs/reference/config/networking/envoy-filter/#EnvoyFilter

使用EnvoyFilter修改某些字段的值，添加特定的过滤器，甚至添加全新的侦听器，群集等

![image-20220817153323280](http://inksnw.asuscomm.com:3001/blog/EnvoyFilter_956ff7a38a970bd0900d61e0d7711001.png)

1、网络过滤器（Network Filters）

   处理连接的核心。（读/写/读写过滤）

2、HTTP过滤器 (HTTP Filters)

​    由特殊的网络过滤器HttpConnectionManager管理 。 处理HTTP1/HTTP2/gRPC请求

​	文档: https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/http_filters

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

## 修改日志级别

进入网关的容器

```bash
curl -X POST http://localhost:15000/logging?level=info
```

## 添加filter

```yaml
apiVersion: networking.istio.io/v1alpha3
kind: EnvoyFilter
metadata:
  name: myfilter
  namespace: istio-system
spec:
  workloadSelector:
    labels:
      istio: ingressgateway
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
          name: my.lua
          typed_config:
            "@type": "type.googleapis.com/envoy.extensions.filters.http.lua.v3.Lua"
            inlineCode: |
              function envoy_on_response(response_handle)
                 response_handle:headers():add("Myname", "eeee")
              end
              function envoy_on_request(request)
                    local getid=request:headers():get("Userid")
                     request:logInfo("userid is "..getid)
                end
---
apiVersion: networking.istio.io/v1alpha3
kind: EnvoyFilter
metadata:
  name: myfilter-adduserid
  namespace: istio-system
spec:
  workloadSelector:
    labels:
      istio: ingressgateway
  configPatches:
    - applyTo: HTTP_FILTER
      match:
        context: GATEWAY
        listener:
          filterChain:
            filter:
              name: "envoy.filters.network.http_connection_manager"
              subFilter:
                name: "my.lua"

      patch:
        operation: INSERT_BEFORE
        value:
          name: adduserid.lua
          typed_config:
            "@type": "type.googleapis.com/envoy.extensions.filters.http.lua.v3.Lua"
            inlineCode: |
              function envoy_on_request(request)
                 request:headers():add("Userid", "123")
              end
---
apiVersion: networking.istio.io/v1alpha3
kind: EnvoyFilter
metadata:
  name: myfilter-prefix
  namespace: istio-system
spec:
  workloadSelector:
    labels:
      istio: ingressgateway
  configPatches:
    - applyTo: HTTP_FILTER
      match:
        context: GATEWAY
        listener:
          filterChain:
            filter:
              name: "envoy.filters.network.http_connection_manager"
              subFilter:
                name: "my.lua"

      patch:
        operation: INSERT_BEFORE
        value:
          name: myprefix.lua
          typed_config:
            "@type": "type.googleapis.com/envoy.extensions.filters.http.lua.v3.Lua"
            inlineCode: |
              function envoy_on_response(response_handle)
                  local myname=response_handle:headers():get("Myname")
                 response_handle:headers():add("Mynewname", "inksnw_"..myname)
              end
---
apiVersion: networking.istio.io/v1alpha3
kind: EnvoyFilter
metadata:
  name: myfilter-checkappid
  namespace: istio-system
spec:
  workloadSelector:
    labels:
      istio: ingressgateway
  configPatches:
    - applyTo: HTTP_FILTER
      match:
        context: GATEWAY
        listener:
          filterChain:
            filter:
              name: "envoy.filters.network.http_connection_manager"
              subFilter:
                name: "envoy.filters.http.cors"
      patch:
        operation: INSERT_AFTER
        value:
          name: checkappid.lua
          typed_config:
            "@type": "type.googleapis.com/envoy.extensions.filters.http.lua.v3.Lua"
            inlineCode: |
              function envoy_on_request(request)
                 local getid=request:headers():get("appid")
                 if(getid==nil) then
                    request:respond(
                          {[":status"] = "400"},
                          "error appid")
                 end
              end
```

### 访问测试

返回的header中添加了相应的头

```bash
HTTP/1.1 200 OK
server: istio-envoy
date: Wed, 17 Aug 2022 08:25:44 GMT
content-type: text/html
content-length: 615
last-modified: Tue, 19 Jul 2022 14:05:27 GMT
etag: "62d6ba27-267"
accept-ranges: bytes
x-envoy-upstream-service-time: 0
myname: inksnw
mynewname: addfake_inksnw
```

### 查看日志

```bash
kubectl logs istio-ingressgateway-6dbb44ff8d-4bpsp -n istio-system
```

