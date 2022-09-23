---
title: "Cert Manager使用"
date: 2022-09-23T14:11:28+08:00
tags: ["k8s"]
---

## cert-manager 工作原理

cert-manager 部署到 Kubernetes 集群后，它会 watch 它所支持的 CRD 资源，我们通过创建 CRD 资源来指示 cert-manager 为我们签发证书并自动续期:

<img src="http://inksnw.asuscomm.com:3001/blog/cert-manager使用_f2fe5a87ab3ea068935705e6a1fd7435.png" alt="image-20220923141435361" style="zoom: 50%;" />
