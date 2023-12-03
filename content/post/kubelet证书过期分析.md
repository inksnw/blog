---
title: "Kubelet证书过期分析"
date: 2023-12-03T10:31:47+08:00
tags: ["k8s"]
---

源自一次环境问题, kubelet重启后, 报错

> "command failed" err="failed to run Kubelet: unable to load bootstrap kubeconfig: stat /etc/kubernetes/bootstrap-kubelet.conf: no such file or directory"

`bootstrap-kubelet.conf` 文件主机上确实没有, 他的作用是什么, 这里简单分析一下 

[官方文档](https://kubernetes.io/zh-cn/docs/reference/access-authn-authz/kubelet-tls-bootstrapping/#bootstrap-initialization) 已经说的很清楚了, 简单来说, 就是首次启动时使用 `临时token(bootstrap-kubelet)` 发起csr请求申请证书, 由于安全性考虑, 这个文件首次使用后一般会被安装工具删除,  后续kubelet 会自动轮转的更新证书  [参考kubelet证书轮换](http://inksnw.asuscomm.com:3001/post/kubelet%E8%AF%81%E4%B9%A6%E8%BD%AE%E6%8D%A2/)  , 但是由于某些原因这个轮转并未能工作, 再次启动时, 临时bootstrap文件没了, 旧证书也失效了, 因此无法启动

## 复现

将一个节点的证书文件, bootstrap-kubelet.conf 都删除, 这个操作等价于证书过期等证书不可用的情况

```
rm -rf /var/lib/kubelet/pki/*
rm -rf /etc/kubernetes/bootstrap-kubelet.conf
```

<img src="http://inksnw.asuscomm.com:3001/blog/kubelet证书过期分析_d3eb52476fd77144ec910bb59ef5af10.png" alt="image-20231203110835717" style="zoom:50%;" />

kubelet启动时, 检测`isClientConfigStillValid` 旧证书失败, 于是去寻找 bootstrap 文件也不存在, 于是报错

所以, 两种修复思路

- 把旧证书换成可用的 
- 生成bootstrap文件, 让kubelet自动发起csr请求, 更新证书

这里提供一下第二种方法

## shell实现

在master节点创建一个`boot.sh`, 运行

```bash
KUBE_APISERVER="https://192.168.50.50:6443"
node_name=node2
# 创建 token
export BOOTSTRAP_TOKEN=$(kubeadm token create \
  --description kubelet-bootstrap-token \
  --groups system:bootstrappers:kubeadm:default-node-token \
  --kubeconfig ~/.kube/config)
# 设置集群参数
kubectl config set-cluster kubernetes \
  --certificate-authority=/etc/kubernetes/pki/ca.crt \
  --embed-certs=true \
  --server=${KUBE_APISERVER} \
  --kubeconfig=bootstrap-kubelet.conf
# 设置客户端认证参数
kubectl config set-credentials tls-bootstrap-token-user \
  --token=${BOOTSTRAP_TOKEN} \
  --kubeconfig=bootstrap-kubelet.conf
# 设置上下文参数
kubectl config set-context tls-bootstrap-token-user@kubernetes \
  --cluster=kubernetes \
  --user=tls-bootstrap-token-user \
  --kubeconfig=bootstrap-kubelet.conf
# 设置默认上下文
kubectl config use-context tls-bootstrap-token-user@kubernetes --kubeconfig=bootstrap-kubelet.conf
```

将生成的`bootstrap-kubelet.conf`文件复制到节点上

```
scp bootstrap-kubelet.conf root@192.168.50.51:/etc/kubernetes/bootstrap-kubelet.conf
```

重启kubelet, 可以看到已工作正常

```bash
systemctl start kubelet
systemctl status kubelet
```

可以看到证书文件已经重新生成

```bash
root@node2:/var/lib/kubelet/pki# tree
.
├── kubelet-client-2023-12-03-11-15-09.pem
├── kubelet-client-current.pem -> /var/lib/kubelet/pki/kubelet-client-2023-12-03-11-15-09.pem
├── kubelet.crt
└── kubelet.key
```

查看csr申请, 可以看到已被自动批准

```
root@node1:~# kubectl get csr
NAME        AGE   SIGNERNAME                                    REQUESTOR                 REQUESTEDDURATION   CONDITION
csr-9rmfc   45s   kubernetes.io/kube-apiserver-client-kubelet   system:bootstrap:dwt5yr   <none>              Approved,Issued
```

## 代码实现

提供一个从`kubeadm` 中扒出来的代码实现

```go
package main

import (
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	bootstraputil "k8s.io/cluster-bootstrap/token/util"
	bootstraptokenv1 "k8s.io/kubernetes/cmd/kubeadm/app/apis/bootstraptoken/v1"
	"k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	"k8s.io/kubernetes/cmd/kubeadm/app/discovery"
	nodebootstraptokenphase "k8s.io/kubernetes/cmd/kubeadm/app/phases/bootstraptoken/node"
	"time"
)

var (
	Endpoint = "192.168.50.50:6443"
)

func main() {
	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	if err != nil {
		panic(err)
	}
	cli, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}
	token, err := createShortLivedBootstrapToken(cli)
	if err != nil {
		panic(err)
	}

	boot(token, Endpoint)
}

func boot(token, Endpoint string) {
	cfg := kubeadm.JoinConfiguration{
		Discovery: kubeadm.Discovery{
			TLSBootstrapToken: token,
			Timeout: &metav1.Duration{
				Duration: 3 * time.Second,
			},
			BootstrapToken: &kubeadm.BootstrapTokenDiscovery{
				APIServerEndpoint: Endpoint,
				Token:             token,
			},
		},
	}

	tlsBootstrapCfg, err := discovery.For(&cfg)
	if err != nil {
		panic(err)
	}
	err = clientcmd.WriteToFile(*tlsBootstrapCfg, "./bootstrap-kubelet.conf")
	if err != nil {
		panic(err)
	}
}
func createShortLivedBootstrapToken(client clientset.Interface) (string, error) {
	tokenStr, err := bootstraputil.GenerateBootstrapToken()
	if err != nil {
		return "", errors.Wrap(err, "error generating token to upload certs")
	}
	token, err := bootstraptokenv1.NewBootstrapTokenString(tokenStr)
	if err != nil {
		return "", errors.Wrap(err, "error creating upload certs token")
	}
	tokens := []bootstraptokenv1.BootstrapToken{{
		Token:       token,
		Description: "Proxy for managing TTL for the kubeadm-certs secret",
		TTL: &metav1.Duration{
			Duration: kubeadmconstants.DefaultCertTokenDuration,
		},
		Groups: []string{kubeadmconstants.NodeBootstrapTokenAuthGroup},
		Usages: []string{"authentication", "signing"},
	}}

	if err := nodebootstraptokenphase.CreateNewTokens(client, tokens); err != nil {
		return "", errors.Wrap(err, "error creating token")
	}
	return tokenStr, nil
}

```

复制到节点上

```bash
scp bootstrap-kubelet.conf root@192.168.50.51:/etc/kubernetes/bootstrap-kubelet.conf
```

