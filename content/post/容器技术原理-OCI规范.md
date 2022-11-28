---
title: "OCI规范-Image spec"
date: 2022-11-28T17:17:41+08:00
tags: ["k8s"]
---

## OCI 规范说起

OCI（Open Container Initiative）规范是事实上的容器标准,OCI 规范分为 `Image spec` 和[ Runtime spec ](http://inksnw.asuscomm.com:3001/post/runc/)两部分，它们分别覆盖了容器生命周期的不同阶段：

## 镜像规范

镜像规范定义了如何创建一个符合 OCI 规范的镜像，它规定了镜像的构建系统需要输出的内容和格式，输出的容器镜像可以被解包成一个 `runtime bundle` ，`runtime bundle` 是由特定文件和目录结构组成的一个文件夹，从中可以根据运行时标准运行容器。

### 镜像里面都有什么

规范要求镜像内容必须包括以下 3 部分：

- **Image Manifest**：提供了镜像的配置和文件系统层定位信息，可以看作是镜像的目录，文件格式为 `json` 。
- **Image Layer Filesystem Changeset**：序列化之后的文件系统和文件系统变更，它们可按顺序一层层应用为一个容器的 rootfs，因此通常也被称为一个 `layer`（与下文提到的镜像层同义），文件格式可以是 `tar` ，`gzip` 等存档或压缩格式。
- **Image Configuration**：包含了镜像在运行时所使用的执行参数以及有序的 rootfs 变更信息，文件类型为 `json`。

*rootfs (root file system)即 `/` 根挂载点所挂载的文件系统，是一个操作系统所包含的文件、配置和目录，但并不包括操作系统内核，同一台机器上的所有容器都共享宿主机操作系统的内核。*
