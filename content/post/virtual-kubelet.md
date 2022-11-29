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

克隆仓库

```
git clone https://github.com/virtual-kubelet/cri.git
cd cri
make build
```
查看providers

```
➜ ./virtual-kubelet providers
cri
```

复制一个k8s集群中`/etc/kubernetes/pki`目录下的`apiserver-kubelet-client.crt`和`apiserver-kubelet-client.key`到本机,执行如下命令启动

```bash
export VKUBELET_POD_IP=192.168.50.40:6443
export APISERVER_CERT_LOCATION="/root/apiserver-kubelet-client.crt"
export APISERVER_KEY_LOCATION="/root/apiserver-kubelet-client.key"
export KUBELET_PORT="10250"
cd bin
./virtual-kubelet --provider cri --kubeconfig /root/.kube/config
```

在k8s主机上查看节点,可以看到已经多了一个名为 `virtual-kubelet `的节点

```bash
➜ kubectl get node
NAME              STATUS   ROLES                         AGE   VERSION
node1             Ready    control-plane,master,worker   3h    v1.23.10
node2             Ready    worker                        3h    v1.23.10
node3             Ready    worker                        3h    v1.23.10
virtual-kubelet   Ready    agent                         10s   v1.15.2-vk-cri-5dec3cb
```

