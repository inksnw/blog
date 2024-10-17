---
title: "无apiserver创建kubelet证书"
date: 2024-10-17T14:25:44+08:00
tags: ["k8s"]
---

在 Kubernetes 集群中, 通常情况下,kubelet的证书是由 API Server 自动生成和管理的。然而，在某些情况下，API Server 可能不可用，这时我们需要手动创建和配置 kubelet 证书。

## 方法一

从 kubeconfig 提取证书信息, 临时使用

**提取客户端证书**：
从 `~/.kube/config` 文件中提取客户端证书并保存为 `client-cert.pem`。

```
grep client-certificate-data ~/.kube/config | awk '{ print $2 }' | base64 --decode > client-cert.pem
```

**提取客户端私钥**：
从 `~/.kube/config` 文件中提取客户端私钥并保存为 `client-key.pem`。

```bash
grep client-key-data ~/.kube/config | awk '{ print $2 }' | base64 --decode > client-key.pem
```

**配置 kubelet**：

```bash
cat client-cert.pem client-key.pem > /var/lib/kubelet/pki/kubelet-client-current.pem
```

提取的证书通常属于 `system:masters` 组，而不是 `system:nodes` 组。`system:masters` 组通常具有更高的权限，而 `system:nodes` 组是专门为节点设计的，权限相对较低

## 方法二

使用现有 CA 证书手动签署节点证书

**生成节点私钥和 CSR**：
为节点生成私钥和证书签名请求 (CSR)。

```bash
openssl genpkey -algorithm RSA -out vm87-key.pem -pkeyopt rsa_keygen_bits:2048
openssl req -new -key vm87-key.pem -out vm87.csr -subj "/CN=system:node:vm87/O=system:nodes"
```

**创建配置文件 `vm87-csr.conf`**：

```ini
[ req ]
default_bits = 2048
prompt = no
default_md = sha256
distinguished_name = dn

[ dn ]
CN = system:node:vm87
O = system:nodes

[ v3_ext ]
authorityKeyIdentifier=keyid,issuer:always
basicConstraints=CA:FALSE
keyUsage=digitalSignature,keyEncipherment
extendedKeyUsage=serverAuth,clientAuth
```

**使用 CA 签署 CSR**：
使用现有的 CA 证书和私钥来签署节点的 CSR，生成节点的证书。

```bash
openssl x509 -req -in vm87.csr -CA /etc/kubernetes/pki/ca.crt -CAkey /etc/kubernetes/pki/ca.key -CAcreateserial -out vm87.pem -days 365 -sha256 -extensions v3_ext -extfile vm87-csr.conf
```

**配置 kubelet**：

```bash
cat vm87.pem vm87-key.pem > /var/lib/kubelet/pki/kubelet-client-current.pem
sudo systemctl restart kubelet
```
