---
title: "Virtual Kubelet"
date: 2022-11-29T11:39:20+08:00
tags: ["k8s"]
---
安装 `containerd`

```
apt install containerd
```

安装 `crictl`

```bash
VERSION="v1.24.1"
wget https://github.com/kubernetes-sigs/cri-tools/releases/download/$VERSION/crictl-$VERSION-linux-amd64.tar.gz
sudo tar zxvf crictl-$VERSION-linux-amd64.tar.gz -C /usr/local/bin
rm -f crictl-$VERSION-linux-amd64.tar.gz
```
设置`crictl`的配置文件

```bash
cat > /etc/crictl.yaml <<EOF
runtime-endpoint: unix:///run/containerd/containerd.sock
image-endpoint: unix:///run/containerd/containerd.sock
timeout: 10
EOF
```

拉取镜像测试

```
➜ crictl pull docker.io/alpine:3.12
➜ crictl pull registry.cn-hangzhou.aliyuncs.com/google_containers/pause:3.2
➜ crictl images
IMAGE                                                       TAG                 IMAGE ID            SIZE
docker.io/library/alpine                                    3.12                24c8ece58a1aa       2.81MB
registry.cn-hangzhou.aliyuncs.com/google_containers/pause   3.2                 80d28bedfe5de       300kB
```

