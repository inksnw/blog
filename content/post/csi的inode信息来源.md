---
title: "Csi的inode信息来源"
date: 2024-09-12T18:28:47+08:00
tags: ["k8s"]
---

## 背景

客户反馈pvc监控页面的inode使用占比超100000%, 手动进入容器df -i  又没有多少inode占用

## 初步验证

稍经google知道inode来自于一个指标 `kubelet_volume_stats_inodes_used`, 这个指标是kubelet 向csi收集后对外提供的, 经过prometheus收集再展示

> prometheus收集的对么, 有没有手动查询的方法

 我们可以使用如下方法查到kubelet的信息

```bash
kubectl get --raw /api/v1/nodes/节点名/proxy/metrics|grep kubelet_volume_stats_inodes_used
```

等价于

```bash
# 测试完记得删除权限
kubectl create clusterrolebinding test:anonymous --clusterrole=cluster-admin --user=system:anonymous
https://192.168.50.131:6443/api/v1/nodes/节点名/proxy/metrics
```

可以看到每个pvc的使用情况, 客户实际使用量 9*10^18 次, 显然过于巨大

```bash
kubelet_volume_stats_inodes_used{namespace="default",persistentvolumeclaim="nfs-pvc"} 13
kubelet_volume_stats_inodes_used{namespace="kubesphere-logging-system",persistentvolumeclaim="opensearch-cluster-data-opensearch-cluster-data-0"} 162521
```

事情到此好像结束了?

> 那kubelet 收集的就正确了么, 怎么证明是 csi 吐的数据不对

google 一下知道这些指标由csi的一个 `NodeGetVolumeStats` 的gprc方法提供, 那有办法手动访问这个接口么

默认使用的 `openebs` 好像没有通过socket与kubelet通信,在`/var/lib/kubelet/plugins` 目录下没有看到相应的插件, 这里我们安装一个nfs 的csi试一下

## 配置NFS csi

安装nfs

```bash
➜ apt install nfs-kernel-server
# 查看导出的nfs
➜ cat /etc/exports
# 重新导出
➜ vim /etc/exports
	/data/public *(rw,sync,no_root_squash,no_subtree_check)
➜ exportfs -a
➜ systemctl status nfs-server
# 挂载配置情况
➜ apt-get install nfs-common
➜ showmount -e localhost
Export list for localhost:
/data/public *
```

在另一台主机上查看情况

```bash
➜ showmount -e 192.168.50.20
Export list for 192.168.50.20:
/data/public *
```

在k8s上安装nfs-csi-driver

官方文档: https://github.com/kubernetes-csi/csi-driver-nfs/blob/master/docs/install-nfs-csi-driver.md

```bash
git clone https://github.com/kubernetes-csi/csi-driver-nfs.git
cd csi-driver-nfs
./deploy/install-driver.sh v4.1.0 local
```

查看csinode信息

```yaml
➜ kubectl get csinode node1 -o yaml
apiVersion: storage.k8s.io/v1
kind: CSINode
metadata:
  name: node1
  ownerReferences:
  - apiVersion: v1
    kind: Node
    name: node1
spec:
  drivers:
  - name: nfs.csi.k8s.io
    nodeID: node1
    topologyKeys: null
```

创建sc与pvc

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: nfs-csi
provisioner: nfs.csi.k8s.io
parameters:
  server: 192.168.50.20
  share: /data/public
reclaimPolicy: Delete
volumeBindingMode: Immediate
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: nfs-pvc
spec:
  accessModes:
    - ReadWriteMany
  resources:
    requests:
      storage: 10Gi
  storageClassName: nfs-csi
```

使用pvc

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: mynginx
  labels:
    app: mynginx
spec:
  replicas: 1
  template:
    metadata:
      name: mynginx
      labels:
        app: mynginx
    spec:
      containers:
        - name: mynginx
          image: nginx
          imagePullPolicy: IfNotPresent
          volumeMounts:
            - mountPath: "/data"
              name: data
      restartPolicy: Always

      volumes:
        - name: data
          persistentVolumeClaim:
            claimName: nfs-pvc
  selector:
    matchLabels:
      app: mynginx
```

再次查看, 可以看到inode占用13个

```bash
kubectl get --raw /api/v1/nodes/节点名/proxy/metrics|grep kubelet_volume_stats_inodes_used
kubelet_volume_stats_inodes_used{namespace="default",persistentvolumeclaim="nfs-pvc"} 13
```

## grpc验证

我们知道kubelet与csi之间是通过grpc通信的, 在`/var/lib/kubelet/plugins/csi-nfsplugin` 目录下可以看到相应的socket

```bash
root@base:/var/lib/kubelet/plugins/csi-nfsplugin# ls
csi.sock
```

把这个socket映射到本地

```bash
 ssh -N -L /tmp/csi.sock:/var/lib/kubelet/plugins/csi-nfsplugin/csi.sock root@192.168.50.131
```

使用 `grpccurl`访问测试, 由于这个csi服务不支持grpc反射, 所以这里需要指定proto文件和依赖的proto文件

proto来自于 https://github.com/container-storage-interface/spec

依赖来自于 https://github.com/protocolbuffers/protobuf/blob/main/src/google/protobuf/descriptor.proto

```bash
grpcurl -plaintext \
-import-path /Users/inksnw/Desktop/spec \
-proto /Users/inksnw/Desktop/spec/csi.proto \
-d '{"volume_id":"192.168.50.20#data/public#pvc-21566fcd-13b5-4bb9-bfaa-0591660713cc#","volume_path":"/var/lib/kubelet/pods/c73005e9-e197-46f9-90b6-8bbc7a8dd86b/volumes/kubernetes.io~csi/pvc-21566fcd-13b5-4bb9-bfaa-0591660713cc/mount"}' \
-unix /tmp/csi.sock  \
csi.v1.Node/NodeGetVolumeStats
```

结果与kubelet拿到的一致, 占用13个

```json
{
  "usage": [
    {
      "available": "933324914688",
      "total": "983351427072",
      "unit": "BYTES"
    },
    {
      "available": "61054963",
      "total": "61054976",
      "used": "13",
      "unit": "INODES"
    }
  ]
}
```

