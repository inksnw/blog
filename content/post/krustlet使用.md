---
title: "Krustlet使用"
date: 2022-10-27T00:41:02+08:00
tags: ["k8s"]
---

### 前置条件

准备一个k8s集群,版本1.24,本机操作在mac上,已经安装好`kubelet`并配置好`~/.kube/config`

```bash
curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
curl -sSL https://get.daocloud.io/docker | sh
```



### 下载二进制程序

到目前为止`2022.10.27` 最新的release1.0还未支持k8s1.24,要想在1.24上使用需要在这个网址下载

```
https://docs.krustlet.dev/intro/install/#from-canary-builds
```

###  创建启动配置文件

文档: https://docs.krustlet.dev/howto/bootstrapping/

按文档中的说明,创建一个`bootstrap.sh`

目录结构

```
.
├── LICENSE
├── README.md
├── bootstrap.sh
└── krustlet-wasi
```

执行`bootstrap.sh`

```bash
./bootstrap.sh 
secret/bootstrap-token-2ypz95 created
Switched to context "kubernetes-admin@cluster.local".
Context "kubernetes-admin@cluster.local" renamed to "tls-bootstrap-token-user@kubernetes".
User "tls-bootstrap-token-user" set.
Context "tls-bootstrap-token-user@kubernetes" modified.
Context "tls-bootstrap-token-user@kubernetes" modified.
```

查看生成的文件

```bash
cat ~/.krustlet/config/bootstrap.conf
```

### 启动krustlet

```bash
# 192.168.50.231 为运行krustlet的主机ip,本例中为我的mac的ip
➜ KUBECONFIG=~/.kube/config ./krustlet-wasi --node-ip 192.168.50.231 --node-name=krustlet --bootstrap-file=~/.krustlet/config/bootstrap.conf 
BOOTSTRAP: TLS certificate requires manual approval. Run kubectl certificate approve base-tls
BOOTSTRAP: received TLS certificate approval: continuing
```

### [签名证书请求](https://kubernetes.io/zh/docs/reference/access-authn-authz/certificate-signing-requests/)

执行`approve`后,可以看到已经添加了一个新节点

```bash
➜ kubectl certificate approve base-tls
➜ kubectl get node
NAME       STATUS   ROLES                  AGE     VERSION
krustlet   Ready    <none>                 7s      1.0.0-alpha.1
node1      Ready    control-plane,worker   7h35m   v1.24.3
node2      Ready    worker                 7h34m   v1.24.3
node3      Ready    worker                 7h34m   v1.24.3
```

