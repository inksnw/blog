---
title: "Fluent Bit日志收集"
date: 2022-09-27T11:41:51+08:00
tags: ["k8s"]
draft: true
---

组件

- fluent-bit 负责解析及数据过滤
- fluentd 负责接收fluent-bit解析后的数据,发送到mq等

安装

文档: https://docs.fluentbit.io/manual/installation/getting-started-with-fluent-bit

```bash
helm repo add fluent https://fluent.github.io/helm-charts
helm upgrade --install fluent-bit fluent/fluent-bit
```

