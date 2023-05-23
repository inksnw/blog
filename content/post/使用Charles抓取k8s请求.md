---
title: "使用Charles抓取k8s请求"
date: 2023-05-23T16:32:50+08:00
tags: ["k8s"]
---

## 配置代理

安装`Charles`,查看设置`Proxy->Proxy Settings`,Charles默认监听于8888端口

<img src="http://inksnw.asuscomm.com:3001/blog/使用Charles抓取k8s请求_b8c69aac5fddbb8c2d0ecc23c47d32bf.png" alt="image-20230523172630419" style="zoom:50%;" />

## 配置客户端证书

从kubeconfig中提取客户端的证书和私钥

```bash
# 提取出客户端证书
grep client-certificate-data ~/.kube/config |awk '{ print $2 }' |base64 --decode > client-cert.pem
# 提取出客户端私钥
grep client-key-data ~/.kube/config |awk '{ print $2 }' |base64 --decode > client-key.pem
# 提取出服务端CA证书
grep certificate-authority-data ~/.kube/config |   awk '{ print $2 }' |   base64 --decode > cluster-ca-cert.pem
```

配置Charles，`Proxy->SSL Proxying Settings`让他拦截端口为`6443`的流量：

<img src="http://inksnw.asuscomm.com:3001/blog/使用Charles抓取k8s请求_cc7636d20e3cb6e5130b86d2bc28f4c2.png" alt="image-20230523173343158" style="zoom:50%;" />

然后配置客户端私钥：

<img src="http://inksnw.asuscomm.com:3001/blog/使用Charles抓取k8s请求_950c76b038342426129a372040e59f01.png" alt="image-20230523171636777" style="zoom:50%;" />



## 配置服务端证书

复制k8s集群主机`/etc/kubernetes/pki/`目录下的`apiserver.crt`与`apiserver.key`

配置

<img src="http://inksnw.asuscomm.com:3001/blog/使用Charles抓取k8s请求_742ae7fa188feb655d5b21b7489e05f9.png" alt="image-20230523171952403" style="zoom:50%;" />

## 配置kubectl

```bash
# 设置https_proxy环境变量，让kubectl使用Charles代理
$ export https_proxy=http://127.0.0.1:8888
# insecure-skip-tls-verify表示不校验服务端证书
$ kubectl get pod -A
NAMESPACE             NAME                                                   READY   STATUS    RESTARTS      AGE
kube-system           calico-node-8mwmk                                      1/1     Running   1 (8h ago)    19h
kube-system           calico-node-clqwh                                      1/1     Running   1 (8h ago)    19h
kube-system           calico-node-rf4qg                                      1/1     Running   1 (8h ago)    19h
...
```

至此,就实现了抓取k8s的请求的功能, 同样的使用`client-go`时,消息也可以被抓取

![image-20230523172335789](http://inksnw.asuscomm.com:3001/blog/使用Charles抓取k8s请求_d59557bce6f984ecba8943a3103977eb.png)
