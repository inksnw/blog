---
title: "Kubelet证书轮换与更新"
date: 2023-08-08T21:03:59+08:00
tags: ["k8s"]
---

## 组件证书

使用如下命令查看证书过期时间：

```
kubeadm  certs check-expiration 
openssl x509 -in /etc/kubernetes/pki/apiserver.crt -noout -text | egrep Not
```

### 手动更新证书

```shell
kubeadm certs renew all 
```

更新各种config

```bash
kubeadm init phase kubeconfig all 
I0811 18:11:06.477258   61845 version.go:256] remote version is much newer: v1.27.4; falling back to: stable-1.26
[kubeconfig] Using kubeconfig folder "/etc/kubernetes"
[kubeconfig] Using existing kubeconfig file: "/etc/kubernetes/admin.conf"
[kubeconfig] Using existing kubeconfig file: "/etc/kubernetes/kubelet.conf"
[kubeconfig] Using existing kubeconfig file: "/etc/kubernetes/controller-manager.conf"
[kubeconfig] Using existing kubeconfig file: "/etc/kubernetes/scheduler.conf"
```

> You must restart the kube-apiserver, kube-controller-manager, kube-scheduler and etcd, so that they can use the new certificates.

## kubelet证书

Kubelet 使用证书进行 Kubernetes API 的认证。 默认情况下，这些证书的签发期限为一年，所以不需要太频繁地进行更新。

### 源码

Kubernetes 包含特性 [kubelet 证书轮换](https://kubernetes.io/zh-cn/docs/reference/access-authn-authz/kubelet-tls-bootstrapping/)， 在当前证书即将过期时， 将自动生成新的秘钥，并从 Kubernetes API 申请新的证书。 一旦新的证书可用，它将被用于与 Kubernetes API 间的连接认证

[官方文档](https://kubernetes.io/zh-cn/docs/tasks/tls/certificate-rotation/) 已经说的很清楚了, 这里简单分析下源码

在kubelet启动中, 如果`/var/lib/kubelet/config.yaml `参数有 `rotateCertificates: true`就会启动CertificateManager

代码位于 kubernetes-1.26.5/cmd/kubelet/app/server.go  818行

```go
if s.RotateCertificates {
  //启动CertificateManager
}
```

核心检测逻辑

代码位于k8s.io/client-go/util/certificate/certificate_manager.go 357行

```go
go wait.Until(func() {
		deadline := m.nextRotationDeadline() //获取下次更新时间, 默认是证书的有效时间剩下 30% 到 10% 之间的任意时间点
		//这里是更新逻辑, 默认一秒检测一次
	}, time.Second, m.stopCh)
```

### demo

这里提供一个手动申请kubelet证书的demo

```go
package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	cryptorand "crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	certificatesv1 "k8s.io/api/certificates/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/certificate/csr"
	"k8s.io/client-go/util/keyutil"
)

func main() {

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), cryptorand.Reader)
	if err != nil {
		panic(err)
	}
	der, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		panic(err)
	}

	keyPEM := pem.EncodeToMemory(&pem.Block{Type: keyutil.ECPrivateKeyBlockType, Bytes: der})

	template := &x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName:   fmt.Sprintf("system:node:%s", "mac"),
			Organization: []string{"system:nodes"},
		},
	}

	csrPEM, err := cert.MakeCSRFromTemplate(privateKey, template)
	if err != nil {
		panic(err)
	}
	sigName := certificatesv1.KubeAPIServerClientKubeletSignerName

	Usages := []certificatesv1.KeyUsage{
		certificatesv1.UsageDigitalSignature,
		certificatesv1.UsageKeyEncipherment,
		certificatesv1.UsageClientAuth,
	}
	clientSet := InitClient()

	reqName, reqUID, err := csr.RequestCertificate(clientSet, csrPEM, "", sigName, nil, Usages, privateKey)
	if err != nil {
		panic(err)
	}
	crtPEM, err := csr.WaitForCertificate(context.TODO(), clientSet, reqName, reqUID)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(keyPEM))
	fmt.Println(string(crtPEM))

}

func InitClient() *kubernetes.Clientset {
	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	if err != nil {
		panic(err)
	}
	c, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}
	return c
}
```

