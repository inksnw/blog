---
title: "二进制安装k8s"
date: 2023-03-23T11:56:31+08:00
draft: true
---

## 安装etcd

下载二进制

```
wget https://github.com/etcd-io/etcd/releases/download/v3.5.7/etcd-v3.5.7-linux-amd64.tar.gz
tar -zxvf etcd-v3.5.7-linux-amd64.tar.gz 
mv etcd-v3.5.7-linux-amd64/etcd /usr/local/bin/
```

创建目录

```
mkdir -p /etc/kubernetes/etcd
mkdir -p /var/lib/etcd
```

创建服务

```bash
vi /etc/systemd/system/etcd.service
[Unit]
Description=etcd
After=network.target
[Service]
User=root
Type=notify
EnvironmentFile=/etc/kubernetes/etcd/etcd.conf
ExecStart=/usr/local/bin/etcd
NotifyAccess=all
RestartSec=10s
LimitNOFILE=40000
Restart=always
[Install]
WantedBy=multi-user.target
```

etcd配置文件

```bash
vi /etc/kubernetes/etcd/etcd.conf
ETCD_DATA_DIR=/var/lib/etcd
ETCD_ADVERTISE_CLIENT_URLS=https://192.168.50.50:2379
ETCD_INITIAL_ADVERTISE_PEER_URLS=https://192.168.50.50:2380
ETCD_INITIAL_CLUSTER_STATE=new
ETCD_METRICS=basic
ETCD_LISTEN_CLIENT_URLS=https://192.168.50.50:2379,https://127.0.0.1:2379
ETCD_INITIAL_CLUSTER_TOKEN=k8s_etcd
ETCD_LISTEN_PEER_URLS=https://192.168.50.50:2380
ETCD_NAME=etcd-node1
ETCD_PROXY=off
ETCD_ENABLE_V2=true
ETCD_INITIAL_CLUSTER=etcd-node1=https://192.168.50.50:2380
```

启动

```
root@node1:~# systemctl stop etcd
root@node1:~# systemctl start etcd
root@node1:~# systemctl status etcd
```

访问测试

```bash
etcdctl  member list
```

## 安装api-server

下载二进制

```
wget https://dl.k8s.io/v1.26.3/kubernetes-client-linux-amd64.tar.gz
```



```
vi  /etc/systemd/system/kube-apiserver.service
 
[Unit]
Description=Kubernetes API Server
Documentation=https://github.com/kubernetes/kubernetes
[Service]
ExecStart=/usr/k8s/kube-apiserver \
--advertise-address=192.168.50.50
--allow-privileged=true
--authorization-mode=Node,RBAC
--bind-address=0.0.0.0
--client-ca-file=/etc/kubernetes/pki/ca.crt
--enable-admission-plugins=NodeRestriction
--enable-bootstrap-token-auth=true
--etcd-cafile=/etc/ssl/etcd/ssl/ca.pem
--etcd-certfile=/etc/ssl/etcd/ssl/node-node1.pem
--etcd-keyfile=/etc/ssl/etcd/ssl/node-node1-key.pem
--etcd-servers=https://192.168.50.50:2379
--feature-gates=RotateKubeletServerCertificate=true,ExpandCSIVolumes=true,CSIStorageCapacity=true
--kubelet-client-certificate=/etc/kubernetes/pki/apiserver-kubelet-client.crt
--kubelet-client-key=/etc/kubernetes/pki/apiserver-kubelet-client.key
--kubelet-preferred-address-types=InternalIP,ExternalIP,Hostname
--requestheader-client-ca-file=/etc/kubernetes/pki/front-proxy-ca.crt
--requestheader-extra-headers-prefix=X-Remote-Extra-
--requestheader-group-headers=X-Remote-Group
--requestheader-username-headers=X-Remote-User
--secure-port=6443
--service-account-issuer=https://kubernetes.default.svc.cluster.local
--service-cluster-ip-range=10.233.0.0/18
--tls-cert-file=/etc/kubernetes/pki/apiserver.crt
--tls-private-key-file=/etc/kubernetes/pki/apiserver.key
--token-auth-file=/etc/kubernetes/configs/token.csv 

Restart=on-failure
[Install]
WantedBy=multi-user.target
```

创建目录

```
mkdir -p /etc/kubernetes/{configs,logs,certs}
```

生成token

```
head -c 16 /dev/urandom | od -An -t x | tr -d ' ' > /etc/kubernetes/configs/token.csv 
```

## 配置kubectl

设置集群

```
kubectl config set-cluster kubernetes \
--certificate-authority=/etc/k8s/certs/ca.pem \
--embed-certs=true \
--server=https://192.168.0.13:6443 

```
设置客户端认证参数
```
kubectl config set-credentials kube-admin \
--client-certificate=/home/shenyi/certs/k8s/admin.pem \
--client-key=/home/shenyi/certs/k8s/admin-key.pem \
--embed-certs=true 

```

设置上下文参数

```
kubectl config set-context kube-admin@kubernetes \
--cluster=kubernetes \
--user=kube-admin 

设置默认上下文
kubectl config use-context kube-admin@kubernetes

```

```
kubectl cluster-info
```

## controller-manager

```

[Unit]
Description=Kubernetes Controller Manager
Documentation=https://github.com/kubernetes/kubernetes
[Service]
ExecStart=/usr/k8s/kube-controller-manager \
--allocate-node-cidrs=true
--authentication-kubeconfig=/etc/kubernetes/controller-manager.conf
--authorization-kubeconfig=/etc/kubernetes/controller-manager.conf
--bind-address=0.0.0.0
--client-ca-file=/etc/kubernetes/pki/ca.crt
--cluster-cidr=10.233.64.0/18
--cluster-name=cluster.local
--cluster-signing-cert-file=/etc/kubernetes/pki/ca.crt
--cluster-signing-duration=87600h
--cluster-signing-key-file=/etc/kubernetes/pki/ca.key
--controllers=*,bootstrapsigner,tokencleaner
--feature-gates=RotateKubeletServerCertificate=true,ExpandCSIVolumes=true,CSIStorageCapacity=true
--kubeconfig=/etc/kubernetes/controller-manager.conf
--leader-elect=true
--node-cidr-mask-size=24
--requestheader-client-ca-file=/etc/kubernetes/pki/front-proxy-ca.crt
--root-ca-file=/etc/kubernetes/pki/ca.crt
--service-account-private-key-file=/etc/kubernetes/pki/sa.key
--service-cluster-ip-range=10.233.0.0/18
--use-service-account-credentials=true
Restart=on-failure
[Install]
WantedBy=multi-user.target
```

```
apiVersion: v1
clusters:
- cluster:
    certificate-authority: /etc/k8s/certs/ca.pem
    server: https://192.168.0.13:6443
  name: kubernetes
contexts:
- context:
    cluster: kubernetes
    user: system:kube-controller-manager
  name: default
current-context: default
kind: Config
preferences: {}
users:
- name: system:kube-controller-manager
  user:
    client-certificate: /etc/k8s/certs/kube-controller-manager.pem
    client-key: /etc/k8s/certs/kube-controller-manager-key.pem
	
```

## schedule

```
[Unit]
Description=Kubernetes Scheduler
Documentation=https://github.com/kubernetes/kubernetes
[Service]
ExecStart=/usr/k8s/kube-scheduler \
--authentication-kubeconfig=/etc/kubernetes/scheduler.conf
--authorization-kubeconfig=/etc/kubernetes/scheduler.conf
--bind-address=0.0.0.0
--feature-gates=RotateKubeletServerCertificate=true,ExpandCSIVolumes=true,CSIStorageCapacity=true
--kubeconfig=/etc/kubernetes/scheduler.conf
--leader-elect=true
Restart=on-failure
[Install]
WantedBy=multi-user.target
```

```
apiVersion: v1
clusters:
- cluster:
    certificate-authority: /etc/k8s/certs/ca.pem
    server: https://192.168.0.13:6443
  name: kubernetes
contexts:
- context:
    cluster: kubernetes
    user: system:kube-scheduler
  name: default
current-context: default
kind: Config
preferences: {}
users:
- name: system:kube-scheduler
  user:
    client-certificate: /etc/k8s/certs/kube-scheduler.pem
    client-key: /etc/k8s/certs/kube-scheduler-key.pem
	
```

## kubelet

```
[Unit]
Description=kubelet: The Kubernetes Node Agent
Documentation=http://kubernetes.io/docs/

[Service]
CPUAccounting=true
MemoryAccounting=true
ExecStart=/usr/local/bin/kubelet 
--bootstrap-kubeconfig=/etc/kubernetes/bootstrap-kubelet.conf 
--kubeconfig=/etc/kubernetes/kubelet.conf 
--config=/var/lib/kubelet/config.yaml 
--cgroup-driver=systemd 
--container-runtime=remote 
--container-runtime-endpoint=unix:///run/containerd/containerd.sock 
--pod-infra-container-image=kubesphere/pause:3.8 
--node-ip=192.168.50.50 
--hostname-override=node1

Restart=always
StartLimitInterval=0
RestartSec=10

[Install]
WantedBy=multi-user.target
```

## kube-proxy

```
[Unit]
Description=Kubernetes Proxy
After=network.target
[Service]
ExecStart=/usr/k8s/kube-proxy \
  - --config=/var/lib/kube-proxy/config.conf
    - --hostname-override=$(NODE_NAME)
Restart=on-failure
LimitNOFILE=65536
[Install]
WantedBy=multi-user.target
```



```
kubectl config set-cluster kubernetes \
  --certificate-authority=/etc/k8s/certs/ca.pem \
  --server=https://192.168.0.13:6443 \
  --embed-certs=true \
  --kubeconfig=kube-proxy.kubeconfig
  
kubectl config set-credentials kube-proxy \
  --client-certificate=/etc/k8s/certs/kube-proxy.pem \
  --client-key=/etc/k8s/certs/kube-proxy-key.pem \
  --embed-certs=true \
  --kubeconfig=kube-proxy.kubeconfig
 
kubectl config set-context default \
  --cluster=kubernetes \
  --user=kube-proxy \
  --kubeconfig=kube-proxy.kubeconfig
```

