---
title: "添加公网ip到k8sapiserver"
date: 2023-02-16T16:22:41+08:00
tags: ["k8s"]
---

## 公网ip访问

通常情况下，我们的 kubernetes 集群是内网环境，如果想通过公网ip访问apiserver怎么办

实际上把公网 ip 签到证书里面就可以，其中有 apiServerCertSANs 这个选项，只要把公网 IP 写到这里，再启动这个集群的时候，这个公网 ip 就会签到证书里。

查看当前证书支持的ip

```bash
cd /etc/kubernetes/pki openssl x509 -in apiserver.crt -noout -text
```

修改配置文件

```bash
# 在apiServer.certSANs 下添加ip
vi /etc/kubernetes/kubeadm-config.yaml
```

备份原证书

```bash
mv apiserver.crt apiserver.crt-bak 
mv apiserver.key apiserver.key-bak
```

执行kubeadm init

```bash
kubeadm init phase certs apiserver --config kubeadm-config.yaml
```

再次查看 apiserver 中证书包含的 ip

```bash
cd /etc/kubernetes/pki openssl x509 -in apiserver.crt -noout -text
```

查看cm中保存的信息

```bash
kubectl get cm  kubeadm-config  -n kube-system -o jsonpath='{.data.ClusterConfiguration}'
```

重启kube-apiserver即可使用公网ip访问

