---
title: "添加公网ip到k8sapiserver"
date: 2023-02-16T16:22:41+08:00
tags: ["k8s"]
---

通常情况下，我们的 kubernetes 集群是内网环境，如果想通过公网ip访问apiserver, 或者说主机增加了一个ip, 如何访问

## 测试

为节点添加一个ip

```
ip addr add 192.168.50.75/24 dev enp1s0
```

报错, 可以看到提示 `192.168.50.75` 不在证书配置内

```bash
E1207 16:23:20.198194   93365 memcache.go:265] couldn't get current server API group list: Get "https://192.168.50.75:6443/api?timeout=32s": tls: failed to verify certificate: x509: certificate is valid for 10.233.0.1, 192.168.50.50, 127.0.0.1, 192.168.50.51, 192.168.50.52, not 192.168.50.75
Unable to connect to the server: tls: failed to verify certificate: x509: certificate is valid for 10.233.0.1, 192.168.50.50, 127.0.0.1, 192.168.50.51, 192.168.50.52, not 192.168.50.75
```

## 解决

查看当前证书支持的ip

```bash
cd /etc/kubernetes/pki 
openssl x509 -in apiserver.crt -noout -text|grep -A 2 "Alternative Name"
            X509v3 Subject Alternative Name: 
                DNS:kubernetes, DNS:kubernetes.default, DNS:kubernetes.default.svc, DNS:kubernetes.default.svc.cluster.local, DNS:lb.kubesphere.local, DNS:localhost, DNS:node1, DNS:node1.cluster.local, DNS:node2, DNS:node2.cluster.local, DNS:node3, DNS:node3.cluster.local, IP Address:10.233.0.1, IP Address:192.168.50.50, IP Address:127.0.0.1, IP Address:192.168.50.51, IP Address:192.168.50.52
    Signature Algorithm: sha256WithRSAEncryption
```

修改配置文件

```bash
# 在apiServer.certSANs 下添加ip
vi /etc/kubernetes/kubeadm-config.yaml
```

```yaml
apiServer:
  extraArgs:
    bind-address: 0.0.0.0
    feature-gates: RotateKubeletServerCertificate=true,ExpandCSIVolumes=true,CSIStorageCapacity=true
  certSANs:
    - "kubernetes"
    - "kubernetes.default"
    - "kubernetes.default.svc"
    - "kubernetes.default.svc.cluster.local"
    - "localhost"
    - "127.0.0.1"
    - "192.168.50.75"
    - ...
```

备份原证书

```bash
cd /etc/kubernetes/pki 
mv apiserver.crt apiserver.crt-bak 
mv apiserver.key apiserver.key-bak
```

执行kubeadm init

```bash
kubeadm init phase certs apiserver --config /etc/kubernetes/kubeadm-config.yaml
[certs] Generating "apiserver" certificate and key
[certs] apiserver serving cert is signed for DNS names [kubernetes kubernetes.default kubernetes.default.svc kubernetes.default.svc.cluster.local lb.kubesphere.local localhost node1 node1.cluster.local node2 node2.cluster.local node3 node3.cluster.local] and IPs [10.233.0.1 192.168.50.50 127.0.0.1 192.168.50.51 192.168.50.52 192.168.50.75]
```

再次查看 apiserver 中证书包含的 ip

```bash
cd /etc/kubernetes/pki 
openssl x509 -in apiserver.crt -noout -text
```

测试, 可以看到已经能正常工作

```bash
d➜ ~ kubectl get ns
NAME              STATUS   AGE
default           Active   4m53s
kube-node-lease   Active   4m54s
kube-public       Active   4m54s
kube-system       Active   4m54s
kubekey-system    Active   4m21s
```



