---
title: "istio和jwt"
date: 2022-08-15T10:27:37+08:00
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
    istio: ingressgateway # 通过label选择gateway 服务
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

正常访问

```bash
curl http://nginx.inksnw.com:32016/
```

## JWT配置

jwk格式说明

| alg  | 具体的算法                  |
| ---- | --------------------------- |
| key  | 秘钥类型                    |
| use  | 可选,sig或enc(签名还是加密) |
| kid  | key的唯一标识               |
| e    | 秘钥的模                    |
| n    | 秘钥指数                    |

生成私钥

```bash
# 生成私钥
openssl genrsa -out mypri.pem 2048
# 生成公钥
openssl rsa -in mypri.pem -pubout -out mypub.pem
```

生成jwk json

```go
package main

import (
	"encoding/json"
	"fmt"
	"github.com/lestrrat-go/jwx/jwk"
	"io/ioutil"
	"os"
)

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func pubKey() []byte {
	f, err := os.Open("../mypub.pem")
	check(err)
	b, err := ioutil.ReadAll(f)
	check(err)
	return b
}
func main() {
	key, err := jwk.ParseKey(pubKey(), jwk.WithPEM(true))
	check(err)
	if pubKey, ok := key.(jwk.RSAPublicKey); ok {
		b, err := json.Marshal(pubKey)
		check(err)
		fmt.Println(string(b))
	}
}
```

生成的json

```json
{
"e": "AQAB",
"kty": "RSA",
"n": "zkWWoZeJbgGLJjzxNg6IBALn1UFAslvAlPSIbgZsvUd1eYF5AXQzBie_ML5JEsIF42kH_6TJt0fX7wQG2bxN7si030racma6bEE1XRpPLURfYk-W4Y7tIDcS67S-Qi4tEcBjyYoEV8TLCLbwgAOU8HDbUEngUrXMRWp0FecBcM61RgR0BpQbhUVJZudPaU3WJRKuqDYicij4z7rT65eBHxcWDh7Ykd5XV0hyZW91Y6CVpl-2On8CNUdogoN7ebw2eXET0VBARHP27j-xXe_czeri_lQnAFTpQQpOuD9au_ZwHx2bgJN9avu7eLeijTqfSTZQ6A1alFs8Rq9_tBzbSQ"
}
```

jwt yaml

参考文档

https://istio.io/latest/docs/reference/config/security/request_authentication/#RequestAuthentication

https://istio.io/latest/docs/reference/config/security/authorization-policy/

jwt 配置的名称空间需要和要控制的ingress在同一名称空间

```yaml
apiVersion: security.istio.io/v1beta1
kind: RequestAuthentication
metadata:
  name: jwt-test
  namespace: istio-system
spec:
  selector:
    matchLabels:
      istio: ingressgateway
  jwtRules:
    - issuer: "user@inksnw.com"
      # istio处理后将token发给后端服务
      forwardOriginalToken: true
      # istio处理后将token中的信息以base64方式发给后端服务
      outputPayloadToHeader: "Userinfo"
      # 可以动态的获取jwk信息
      # jwksUri:
      jwks: |
        {
          "e": "AQAB",
          "kty": "RSA",
          "n": "zkWWoZeJbgGLJjzxNg6IBALn1UFAslvAlPSIbgZsvUd1eYF5AXQzBie_ML5JEsIF42kH_6TJt0fX7wQG2bxN7si030racma6bEE1XRpPLURfYk-W4Y7tIDcS67S-Qi4tEcBjyYoEV8TLCLbwgAOU8HDbUEngUrXMRWp0FecBcM61RgR0BpQbhUVJZudPaU3WJRKuqDYicij4z7rT65eBHxcWDh7Ykd5XV0hyZW91Y6CVpl-2On8CNUdogoN7ebw2eXET0VBARHP27j-xXe_czeri_lQnAFTpQQpOuD9au_ZwHx2bgJN9avu7eLeijTqfSTZQ6A1alFs8Rq9_tBzbSQ"
        }
---
apiVersion: security.istio.io/v1beta1
kind: AuthorizationPolicy
metadata:
  name: prod-authpolicy
  namespace: istio-system
spec:
  selector:
    matchLabels:
      istio: ingressgateway
  rules:
    - from:
        - source:
            requestPrincipals: [ "*" ]
```

再次访问

```bash
curl http://nginx.inksnw.com:32016/
```

在https://jwt.io/ 生成 需要的jwt信息

![Snipaste_2022-08-15_13-56-59](http://inksnw.asuscomm.com:3001/blog/Snipaste_2022-08-15_13-56-59.jpg)

再次访问

```bash
curl -i -X GET \
   -H "Authorization:Bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyLCJpc3MiOiJ1c2VyQGlua3Nudy5jb20ifQ.i3i0a9YjagdQ8Ao6gm63LcL1SuZTk0pd8PPfOniyOdIBo0kJDGzR_5zdDS8XbjEuopwUx48JLJJI0GRRb8hzQvl-v6J0O_C0g0iHXkVq2yqUozGLFCkStjLx3V0-W8s8HYVJCPZgFS-t9YAcmREeUh2vxs3mQ4CP0iwMsKYcvj-rjyViQ_MiF3UXh9N2rHJ_a8aqRQnKkFu-K51XlAl0hYUk6UmocIwXad5YPSQ574qaK6YKVsCVEemGVIRnA3Gr58NDDGG-VPgrVsslEMwtCQLlj4K6iQtptvwQPmdVkC4gZFlr-vum9KwiC-fBKwGeTOyQvbVNn-WkYhzS8RpH9g" \
 'http://nginx.inksnw.com:32016/'
```

## 跨域处理

https://istio.io/latest/docs/reference/config/networking/virtual-service/#CorsPolicy

在ws配置中添加如下信息

```yaml
    corsPolicy:
      allowOrigins:
        - exact: "*"
      allowMethods:
        - GET
        - POST
        - PATCH
        - PUT
        - DELETE
        - OPTIONS
      allowCredentials: true
      allowHeaders:
        - authorization
      maxAge: "24h"
```

## 在jwt处理之前就把cors信息返回

如果token出错,cors也不会处理,因此使用以下方式提前处理cors

```yaml
apiVersion: networking.istio.io/v1alpha3
kind: EnvoyFilter
metadata:
  name: reorder-cors-before-jwt
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
        operation: REMOVE
    - applyTo: HTTP_FILTER
      match:
        context: GATEWAY
        listener:
          filterChain:
            filter:
              name: "envoy.filters.network.http_connection_manager"
              subFilter:
                name: "envoy.filters.http.jwt_authn"
      patch:
        operation: INSERT_BEFORE
        value:
          name: "envoy.filters.http.cors"
          typed_config:
            "@type": "type.googleapis.com/envoy.extensions.filters.http.cors.v3.Cors"
```

