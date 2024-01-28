---
title: "本地搭建helm源"
date: 2024-01-28T15:33:30+08:00
tags: ["k8s"]
---
helm源本质就是一个`url/index.yaml` 可以获取到helm包的下载地址等信息, 可以使用github page, harbor等作为helm 源

`chartmuseum`  工具提供一个简单的自建方式, 调试时比较方便

## 安装

```bash
curl https://raw.githubusercontent.com/helm/chartmuseum/main/scripts/get-chartmuseum | bash
helm plugin install https://github.com/chartmuseum/helm-push.git
```

## 启动一个本地源

```bash
chartmuseum --debug --port=8080  --storage="local"  --storage-local-rootdir="./chartstorage"  --listen-host="192.168.50.251"
```
## 添加使用

```bash
helm repo add myrepo http://192.168.50.251:8080
helm cm-push test-0.1.0.tgz myrepo
helm repo update
# 查看
helm search repo myrepo
NAME       	CHART VERSION	APP VERSION	DESCRIPTION                
myrepo/test	0.1.0        	1.16.0     	A Helm chart for Kubernetes
```

