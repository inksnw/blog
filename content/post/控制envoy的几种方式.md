---
title: "控制envoy的几种方式"
date: 2023-02-17T15:40:43+08:00
tags: ["k8s","envoy"]
---

## 安装

```bash
brew install envoy
```

## 静态文件

下载[示例配置](https://www.envoyproxy.io/docs/envoy/latest/start/quick-start/configuration-static#static-resources)文件 `envoy-demo.yaml` 并启动

```bash
envoy -c envoy-demo.yaml
```

修改示例配置中的转发地址为 `www.baidu.com`

访问本地,可以看到请求被成功转发

```bash
curl localhost:10000
```
配置文件解析


- `listener` : Envoy 的监听地址，就是真正干活的。Envoy 会暴露一个或多个 Listener 来监听客户端的请求。
- `filter` : 过滤器。在 Envoy 中指的是一些“可插拔”和可组合的逻辑处理层，是 Envoy 核心逻辑处理单元。
- `route_config` : 路由规则配置。即将请求路由到后端的哪个集群。
- `cluster` : 服务提供方集群。Envoy 通过服务发现定位集群成员并获取服务，具体路由到哪个集群成员由负载均衡策略决定

## GRPC控制

官方[参考实现](https://github.com/envoyproxy/go-control-plane)
