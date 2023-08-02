---
title: "本地调试k8s"
date: 2023-08-02T10:53:21+08:00
tags: ["k8s"]
---

**背景**: 有时候想详细理解一下k8s的运作流程, 硬看代码实在很难调试, 所以试着在mac上搭建k8s控制平面的编译环境

准备一台干净的ubuntu主机, 关闭防火墙, 关闭selinux, 开启Ipv4转发,禁用swap

## etcd

在mac上安装etcd, 这里最简化安装, 无证书, 单节点

```bash
brew install etcd
brew services start etcd
brew services list
# 清空数据
etcdctl del --prefix /
```

## apiserver

在ubuntu主机上创建一个kubeadm配置文件, `172.31.164.151` `10.8.0.2`都是我mac的ip

```yaml
apiVersion: kubeadm.k8s.io/v1beta3
kind: ClusterConfiguration
kubernetesVersion: v1.26.5
certificatesDir: /etc/kubernetes/pki
controllerManager:
  extraArgs:
    cluster-signing-cert-file: /etc/kubernetes/pki/ca.crt
    cluster-signing-key-file: /etc/kubernetes/pki/ca.key
apiServer:
  certSANs:
  - "10.96.0.1"
  - "127.0.0.1"
  - "inksnwdeiMac.local"
  - "172.31.164.151"
  - "10.8.0.2"
  extraArgs:
    tls-cert-file: /etc/kubernetes/pki/apiserver.crt
    tls-private-key-file: /etc/kubernetes/pki/apiserver.key
```

利用`kubeadm` 生成所有需要的证书
```bash
kubeadm init phase certs all --config=cc.yaml
.
└── pki
    ├── apiserver.crt
    ├── apiserver-etcd-client.crt
    ├── apiserver-etcd-client.key
    ├── apiserver.key
    ├── apiserver-kubelet-client.crt
    ├── apiserver-kubelet-client.key
    ├── ca.crt
    ├── ca.key
    ├── etcd
    │   └── ...
    ├── front-proxy-ca.crt
    ├── front-proxy-ca.key
    ├── front-proxy-client.crt
    ├── front-proxy-client.key
    ├── sa.key
    └── sa.pub
```

在代码目录创建一个`config`目录,复制主机上的`pki`到`config`目录

配置apiserver启动参数

```bash
--advertise-address=0.0.0.0 --allow-privileged=true --authorization-mode=Node,RBAC --bind-address=0.0.0.0 --client-ca-file=pki/ca.crt --enable-admission-plugins=NodeRestriction --enable-bootstrap-token-auth=true  --etcd-servers=http://127.0.0.1:2379 --feature-gates=ExpandCSIVolumes=true,CSIStorageCapacity=true,RotateKubeletServerCertificate=true --kubelet-client-certificate=pki/apiserver-kubelet-client.crt --kubelet-client-key=pki/apiserver-kubelet-client.key --kubelet-preferred-address-types=InternalIP,ExternalIP,Hostname --proxy-client-cert-file=pki/front-proxy-client.crt --proxy-client-key-file=pki/front-proxy-client.key --requestheader-allowed-names=front-proxy-client --requestheader-client-ca-file=pki/front-proxy-ca.crt --requestheader-extra-headers-prefix=X-Remote-Extra- --requestheader-group-headers=X-Remote-Group --requestheader-username-headers=X-Remote-User --secure-port=6443 --service-account-issuer=https://kubernetes.default.svc.cluster.local --service-account-key-file=pki/sa.pub --service-account-signing-key-file=pki/sa.key --service-cluster-ip-range=10.233.0.0/18 --tls-cert-file=pki/apiserver.crt --tls-private-key-file=pki/apiserver.key 
```

<img src="http://inksnw.asuscomm.com:3001/blog/本地调试k8s_2f27f1a6e2a99465d21b16f178efb7b7.png" alt="image-20230802163758775" style="zoom:50%;" />

kubectl配置

在ubuntu主机上生成kubectl使用的config文件, 默认会生成到 `/etc/kubernetes/admin.conf` ,我们把它复制到**mac**和**ubuntu主机**的家目录

```bash
kubeadm init phase kubeconfig admin 
# 修改配置文件中的server地址, 我这里是10.8.0.2
mkdir -p /root/.kube/
cp /etc/kubernetes/admin.conf /root/.kube/config
```

启动apiserver, 查询测试

```bash
kubectl get ns
NAME              STATUS   AGE
default           Active   5h9m
kube-node-lease   Active   5h9m
kube-public       Active   5h9m
kube-system       Active   5h9m
```

可以看到, 此时apiserver已经可以正常工作

## kube-scheduler

配置启动参数, 参考上方apisever, 这里连接使用的rbac相关信息直接暴力的使用了kubectl的

```bash
--authentication-kubeconfig=/Users/inksnw/.kube/config --authorization-kubeconfig=/Users/inksnw/.kube/config --bind-address=0.0.0.0 --feature-gates=RotateKubeletServerCertificate=true,ExpandCSIVolumes=true,CSIStorageCapacity=true --kubeconfig=/Users/inksnw/.kube/config  
```

## controller-manager

配置启动参数, 参考上方apisever, 这里连接使用的rbac相关信息直接暴力的使用了kubectl的

```bash
--allocate-node-cidrs=true --authentication-kubeconfig=/Users/inksnw/.kube/config --authorization-kubeconfig=/Users/inksnw/.kube/config --bind-address=0.0.0.0 --client-ca-file=pki/ca.crt --cluster-cidr=10.233.64.0/18 --cluster-name=cluster.local --cluster-signing-cert-file=pki/ca.crt --cluster-signing-duration=87600h --cluster-signing-key-file=pki/ca.key --controllers=*,bootstrapsigner,tokencleaner --feature-gates=RotateKubeletServerCertificate=true,ExpandCSIVolumes=true,CSIStorageCapacity=true --kubeconfig=/Users/inksnw/.kube/config --leader-elect=true --node-cidr-mask-size=24 --requestheader-client-ca-file=pki/front-proxy-ca.crt --root-ca-file=pki/ca.crt --service-account-private-key-file=pki/sa.key --service-cluster-ip-range=10.233.0.0/18 --use-service-account-credentials=true
```

## 节点配置

由于`kubelet` 的 `cadvisor`不支持运行在mac上, 且cni,cri等一般都是linux环境, 所以kubelet我们就不在mac上跑了(一时搞不定, 后续再想办法吧)

### containerd

在主机安装 `containerd`

```bash
apt install containerd
```

### kubelet

配置`kubelet`的systemd文件`vi /etc/systemd/system/kubelet.service`

```bash
[Unit]
Description=kubelet: The Kubernetes Node Agent
Documentation=http://kubernetes.io/docs/

[Service]
CPUAccounting=true
MemoryAccounting=true
ExecStart=/usr/local/bin/kubelet --kubeconfig=/root/.kube/config --config=/var/lib/kubelet/config.yaml --cgroup-driver=systemd --container-runtime-endpoint=unix:///run/containerd/containerd.sock --pod-infra-container-image=kubesphere/pause:3.9 --node-ip=192.168.50.127 --hostname-override=node1
Restart=always
StartLimitInterval=0
RestartSec=10

[Install]
WantedBy=multi-user.target
```

创建一个配置文件`vi /var/lib/kubelet/config.yaml`

```bash
mkdir -p /var/lib/kubelet
```

```yaml
apiVersion: kubelet.config.k8s.io/v1beta1
authentication:
  anonymous:
    enabled: false
  webhook:
    cacheTTL: 0s
    enabled: true
  x509:
    clientCAFile: /etc/kubernetes/pki/ca.crt
authorization:
  mode: Webhook
  webhook:
    cacheAuthorizedTTL: 0s
    cacheUnauthorizedTTL: 0s
cgroupDriver: systemd
clusterDNS:
- 169.254.25.10
clusterDomain: cluster.local
containerLogMaxFiles: 3
containerLogMaxSize: 5Mi
cpuManagerReconcilePeriod: 0s
evictionHard:
  memory.available: 5%
  pid.available: 10%
evictionMaxPodGracePeriod: 120
evictionPressureTransitionPeriod: 30s
evictionSoft:
  memory.available: 10%
evictionSoftGracePeriod:
  memory.available: 2m
featureGates:
  CSIStorageCapacity: true
  ExpandCSIVolumes: true
  RotateKubeletServerCertificate: true
fileCheckFrequency: 0s
healthzBindAddress: 127.0.0.1
healthzPort: 10248
httpCheckFrequency: 0s
imageMinimumGCAge: 0s
kind: KubeletConfiguration
kubeReserved:
  cpu: 200m
  memory: 250Mi
logging:
  flushFrequency: 0
  options:
    json:
      infoBufferSize: "0"
  verbosity: 0
maxPods: 110
memorySwap: {}
nodeStatusReportFrequency: 0s
nodeStatusUpdateFrequency: 0s
podPidsLimit: 10000
resolvConf: /run/systemd/resolve/resolv.conf
rotateCertificates: true
runtimeRequestTimeout: 0s
shutdownGracePeriod: 0s
shutdownGracePeriodCriticalPods: 0s
streamingConnectionIdleTimeout: 0s
syncFrequency: 0s
systemReserved:
  cpu: 200m
  memory: 250Mi
volumeStatsAggPeriod: 0s
```

启动kubelet

```
systemctl start kubelet
```

查看node1处于NotReady状态 , 因为我们还未安装cni插件

>  暂不清楚没有配置加节点的过程, 怎么就自动加上了

```bash
kubectl get node
NAME    STATUS     ROLES    AGE   VERSION
node1   NotReady   <none>   50s   v1.26.5
```

### 配置cni

>  `kubelet`和容器运行时,都有调用cni的代码, 在一起使用时, 实际工作的是kubelet

```bash
wget https://github.com/containernetworking/plugins/releases/download/v1.3.0/cni-plugins-linux-amd64-v1.3.0.tgz
mkdir -p /opt/cni/bin/
mkdir -p /etc/cni/net.d
tar -zxvf cni-plugins-linux-amd64-v1.3.0.tgz -C /opt/cni/bin/
```

创建cni的配置文件:

```bash
cat << EOF | tee /etc/cni/net.d/mynet.json
{
    "cniVersion": "0.4.0",
    "name": "mynet",
    "type": "bridge",
    "bridge": "cni0",
    "isDefaultGateway": true,
    "forceAddress": false,
    "ipMasq": true,
    "hairpinMode": true,
    "ipam": {
        "type": "host-local",
        "subnet": "10.233.64.0/24"
    }
}
EOF
```

## 验证

查看节点状态

```bash
kubectl get node
NAME    STATUS   ROLES    AGE   VERSION
node1   Ready    <none>   14m   v1.26.5
```

运行pod

```bash
kubectl run nginx --image=nginx
kubectl get pod
NAME    READY   STATUS    RESTARTS   AGE
nginx   1/1     Running   0          31s
```

好了, 至此, 控制平面已经在本地跑起来, 单步调试什么的可以折腾起来了~

