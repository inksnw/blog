---
title: "istio授权策略"
date: 2022-08-15T10:27:37+08:00
tags: ["k8s"]
subs: ["istio"]
---

Istio授权策略可对网格中的工作负载进行访问控制.类似spring security,casbin

## action

 ALLOW 或者DENY、AUDIT、CUSTOM

 同时支持允许和拒绝策略。如同时存在，则拒绝策略优先处理。评估由以下规则确定：

1、如果有任何与请求匹配的DENY策略，请拒绝该请求。

2、如果没有针对该工作负载的ALLOW策略，则允许该请求。

3、如果有任何ALLOW策略与请求匹配，则允许该请求。

##  rule规则

from  指定请求的来源。如果未设置，则允许任何来源。

> principals: 源的serviceaccount 列表.需要开启mTLS(双向认证)
>
> requestPrincipals:请求身份列表（即“ iss / sub”声明）
>
> namespaces : ns列表。需要mTLS
>
> ipBlocks : IP 允许列表（source.ip）。支持单个IP（例如“ 1.2.3.4”）和CIDR（例如“ 1.2.3.0/24”）
>
> remoteIpBlocks: 针对的是remote.ip (如 X-Forwarded-For)
>
> 每一个选项都有个 notxxxx  作为反义词 如 notPrincipals、如notRequestPrincipals

to 指定请求的操作。如果未设置，则允许任何操作。

> hosts:允许的主机列表--- request.host
>
> ports:端口
>
> methods: 方法
>
> paths : 路由
>
> 每一个选项都有个 notxxxx  作为反义词,如notHosts

when  请求的其他条件列表。如果未设置，则允许任何条件。

## 测试

```yaml
apiVersion: security.istio.io/v1beta1
kind: AuthorizationPolicy
metadata:
  name: prod-authpolicy
  namespace: istio-system
spec:
  action: ALLOW
  selector:
    matchLabels:
      istio: ingressgateway
  rules:
    - from:
        - source:
            requestPrincipals: ["*"]
      to:
       - operation:
          methods: ["GET"]
          paths: ["/"]
```

只能访问根路径,添加子路由会无权访问

### 允许访问

```yaml
apiVersion: security.istio.io/v1beta1
kind: AuthorizationPolicy
metadata:
  name: prod-authpolicy
  namespace: istio-system
spec:
  action: ALLOW
  selector:
    matchLabels:
      istio: ingressgateway
  rules:
    - from:
        - source:
            requestPrincipals: ["*"]
      to:
       - operation:
          methods: ["GET"]
          paths: ["/prods/*"]
    - from:
        - source:
            requestPrincipals: ["*"]
      to:
        - operation:
            methods: ["GET"]
            paths: ["/admin"]
      # jwt中的信息 用户为 admin时才能访问admin路由
      when:
        - key: request.auth.claims[role]
          values: ["admin"]
```

### when常用参数

```yaml
key: request.headers[User-Agent]
values: ["Mozilla/*"]

key: source.namespace
values: ["default"]

key: source.principal
values: ["cluster.local/ns/default/sa/productpage"]

key: request.auth.claims[iss]

values: ["*@foo.com"]
```

### 拒绝访问示例

#### 方法一

```yaml
 - to:
        - operation:
            methods: ["POST"]
            paths: ["/admin"]
      when:
        - key: request.auth.claims[role]
          values: ["superadmin"]

```

#### 方法二

```yaml
action: DENY
  selector:
    matchLabels:
      istio: ingressgateway
  rules:
    - to:
        - operation:
            methods: ["POST"]
            paths: ["/admin"]
      when:
        - key: request.auth.claims[role]
          notValues: ["superadmin"]
```





