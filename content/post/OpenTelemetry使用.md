---
title: "OpenTelemetry使用"
date: 2022-09-25T17:47:24+08:00
tags: ["k8s"]
---

## 什么是OpenTelemetry

有了以上的背景知识，我们就可以顶一下OpenTelemetry的终极目标了：实现Metrics、Tracing、Logging的融合及大一统，作为APM的数据采集终极解决方案。

- Tracing：提供了一个请求从接收到处理完成整个生命周期的跟踪路径，一次请求通常过经过N个系统，因此也被称为分布式链路追踪
- Metrics：例如cpu、请求延迟、用户访问数等Counter、Gauge、Histogram指标
- Logging：传统的日志，提供精确的系统记录

这三者的组合可以形成大一统的APM解决方案：

1. 基于Metrics告警发现异常
2. 通过Tracing定位到具体的系统和方法
3. 根据模块的日志最终定位到错误详情和根源
4. 调整Metrics等设置，更精确的告警/发现问题

**该如何融合？**

在以往对APM的理解中，这三者都是完全独立的，但是随着时间的推移，人们逐步发现了三者之间的关联，例如我们可以把Tracing的TraceID打到Logging的日志中，这样可以把分布式链路跟踪和日志关联到一起，彼此数据互通，但是还存在以下问题：

1. 如何把Metrics和其他两者关联起来
2. 如何提供更多维度的关联，例如请求的方法名、URL、用户类型、设备类型、地理位置等
3. 关联关系如何一致，且能够在分布式系统下传播

在OpenTelemetry中试图使用Context为Metrics、Logging、Tracing提供统一的上下文，三者均可以访问到这些信息，同时Context可以随着请求链路的深入，不断往下传播

- Context数据在Task/Request的执行周期中都可以被访问到
- 提供统一的存储层，用于保存Context信息，并保证在各种语言和处理模型下都可以工作（例如单线程模型、线程池模型、CallBack模型、Go Routine模型等）
- 多种维度的关联基于元信息(标签)实现，元信息由业务确定，例如：通过Env来区别是测试还是生产环境等
- 提供分布式的Context传播方式，例如通过W3C的traceparent/tracestate头、GRPC协议等

## 编写简单示例

```
http://inksnw.asuscomm.com:3000/inksnw/myotel
```

## 安装Jaeger

下载`all in one `包

https://www.jaegertracing.io/download/

```bash
.
├── example-hotrod
├── jaeger-agent
├── jaeger-all-in-one
├── jaeger-collector
├── jaeger-ingester
└── jaeger-query
./jaeger-all-in-one
```

访问

```
http://127.0.0.1:16686/search
```

<img src="http://inksnw.asuscomm.com:3001/blog/OpenTelemetry使用_d3a6255352cf09dee377af017c56e4cd.png" alt="image-20220925182452720" style="zoom:50%;" />
