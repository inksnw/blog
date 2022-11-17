---
title: "Rook安装与使用"
date: 2022-11-17T11:05:33+08:00
tags: ["k8s"]
---

## 在k8s上安装rook

安装文档 https://rook.io/docs/rook/v1.10/Getting-Started/quickstart/

```bash
git clone --single-branch --branch v1.10.5 https://github.com/rook/rook.git
cd rook/deploy/examples
kubectl create -f crds.yaml -f common.yaml -f operator.yaml
kubectl create -f cluster.yaml
```

查看创建的Pod

```bash
root@node1:~# kubectl -n rook-ceph get pod
NAME                                              READY   STATUS      RESTARTS   AGE
csi-cephfsplugin-7nsvl                            2/2     Running     0          2m19s
csi-cephfsplugin-bshm7                            2/2     Running     0          2m19s
csi-cephfsplugin-provisioner-f88787bbb-8shvz      5/5     Running     0          2m19s
csi-cephfsplugin-provisioner-f88787bbb-9j58k      5/5     Running     0          2m19s
csi-cephfsplugin-tf4f8                            2/2     Running     0          2m19s
csi-rbdplugin-bbn24                               2/2     Running     0          2m20s
csi-rbdplugin-provisioner-d4cc97cf5-cmzgt         5/5     Running     0          2m19s
csi-rbdplugin-provisioner-d4cc97cf5-l5pwz         5/5     Running     0          2m19s
csi-rbdplugin-v5wgr                               2/2     Running     0          2m20s
csi-rbdplugin-zrljs                               2/2     Running     0          2m20s
rook-ceph-crashcollector-node1-6d454c8476-68njg   1/1     Running     0          89s
rook-ceph-crashcollector-node2-55f9b7dd8c-gmt6c   1/1     Running     0          70s
rook-ceph-crashcollector-node3-58b65b7846-g7wnc   1/1     Running     0          89s
rook-ceph-mgr-a-6fb4b574d5-46nlc                  3/3     Running     0          89s
rook-ceph-mgr-b-5f5c669c6d-hsjt8                  3/3     Running     0          89s
rook-ceph-mon-a-749564574d-rpbls                  2/2     Running     0          2m13s
rook-ceph-mon-b-86dd8ddf8c-2zhdv                  2/2     Running     0          109s
rook-ceph-mon-c-5f99786dbc-kz4bh                  2/2     Running     0          100s
rook-ceph-operator-946fb5c77-lz4mf                1/1     Running     0          4m42s
rook-ceph-osd-prepare-node1-ndhpc                 0/1     Completed   0          42s
rook-ceph-osd-prepare-node2-wc25g                 0/1     Completed   0          39s
rook-ceph-osd-prepare-node3-m6shs                 0/1     Completed   0          36s
```

查看dashboard

```bash
kubectl get svc -n rook-ceph
kubectl port-forward svc/rook-ceph-mgr-dashboard 8443:8443 -n rook-ceph --address="0.0.0.0"
```

用户名是`admin`,密码使用下方命令查询

```bash
kubectl -n rook-ceph get secret rook-ceph-dashboard-password -o jsonpath="{['data']['password']}" | base64 --decode && echo
```

![image-20221117112105162](/Users/inksnw/Library/Application Support/typora-user-images/image-20221117112105162.png)

## 配置OSD

查看dashboard界面发现有两个警告

> OSD count 0 < osd_pool_default_size 3

意思是应该有3个`OSD`现在有0个, 查看创建`OSD`的pod日志

```bash
$ kubectl logs rook-ceph-osd-prepare-node3-mxshc  -n rook-ceph
cephosd: skipping OSD configuration as no devices matched the storage settings for this node "node3"
```

显示没有在`node3`上发现匹配的`devices`,查看文档 [OSD pods are not created on my devices](https://rook.io/docs/rook/v1.10/Troubleshooting/ceph-common-issues/#osd-pods-are-not-created-on-my-devices) 发现需要在`cluster.yaml`中配置磁盘相关的信息

在kvm界面上给三台节点添加硬盘,推荐一个kvm管理工具,很好用 https://cockpit-project.org/

进入主机查看磁盘信息,看到新加的磁盘叫 `/dev/vdb`

```bash
$ fdisk -l 
Disk /dev/vdb: 30 GiB, 32212254720 bytes, 62914560 sectors
Units: sectors of 1 * 512 = 512 bytes
Sector size (logical/physical): 512 bytes / 512 bytes
I/O size (minimum/optimal): 512 bytes / 512 bytes
```

修改`cluster.yaml` ,增加磁盘选择条件

```yaml
spec:
  storage: 
    nodes:
      - devices:
          - name: "vdb"
```

重新提交 `cluster.yaml` , 发现`OSD`的警告已经消失

> MON_DISK_LOW: mons b,c are low on available space

这个警告可以先不用管

## 配置StorageClass

### 创建存储池

```bash
kubectl apply -f pool.yaml 
```

### 创建StorageClass

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: rook-ceph-block
provisioner: rook-ceph.rbd.csi.ceph.com
reclaimPolicy: Delete
volumeBindingMode: Immediate
parameters:
  clusterID: rook-ceph
  csi.storage.k8s.io/controller-expand-secret-name: rook-csi-rbd-provisioner
  csi.storage.k8s.io/controller-expand-secret-namespace: rook-ceph
  csi.storage.k8s.io/fstype: ext4
  csi.storage.k8s.io/node-stage-secret-name: rook-csi-rbd-node
  csi.storage.k8s.io/node-stage-secret-namespace: rook-ceph
  csi.storage.k8s.io/provisioner-secret-name: rook-csi-rbd-provisioner
  csi.storage.k8s.io/provisioner-secret-namespace: rook-ceph
  imageFeatures: layering
  imageFormat: "2"
  pool: replicapool
```

查看此时集群的sc

```bash
$ kubectl get sc
NAME              PROVISIONER                  RECLAIMPOLICY   VOLUMEBINDINGMODE      ALLOWVOLUMEEXPANSION   AGE
local (default)   openebs.io/local             Delete          WaitForFirstConsumer   false                  4h35m
rook-ceph-block   rook-ceph.rbd.csi.ceph.com   Delete          Immediate              false                  7m6s
```

查看状态

```bash
$ kubectl get cephcluster -n rook-ceph
NAME        DATADIRHOSTPATH   MONCOUNT   AGE     PHASE   MESSAGE                        HEALTH        EXTERNAL
rook-ceph   /var/lib/rook     3          3h57m   Ready   Cluster created successfully   HEALTH_WARN 
```

创建一个pvc

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: test-nginx
spec:
  storageClassName: rook-ceph-block
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
```

查看

```bash
$ kubectl get pvc
NAME         STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS      AGE
test-nginx   Bound    pvc-35f9dabb-e313-44cc-a642-0efdf307ccd0   1Gi        RWO            rook-ceph-block   5m40s
```

创建一个pod

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: task-pv-pod
spec:
  volumes:
    - name: task-pv-storage
      persistentVolumeClaim:
        claimName: test-nginx
  containers:
    - name: task-pv-container
      image: nginx
      ports:
        - containerPort: 80
          name: "http-server"
      volumeMounts:
        - mountPath: "/usr/share/nginx/html"
          name: task-pv-storage
```

进入容器查看磁盘大小

```bash
$ kubectl exec -it task-pv-pod -- /bin/bash
$ df -h
/dev/rbd0                          974M   24K  958M   1% /usr/share/nginx/html
```

修改pvc大小

```
kubectl edit pvc test-nginx
```
报错

> error: persistentvolumeclaims "test-nginx" could not be patched: persistentvolumeclaims "test-nginx" is forbidden: only dynamically provisioned pvc can be resized and the storageclass that provisions the pvc must support resize

修改`StorageClass` 开放扩容

```bash
$ kubectl edit sc rook-ceph-block
# 添加 allowVolumeExpansion: true
$ kubectl get sc
NAME              PROVISIONER                  RECLAIMPOLICY   VOLUMEBINDINGMODE      ALLOWVOLUMEEXPANSION   AGE
local (default)   openebs.io/local             Delete          WaitForFirstConsumer   false                  4h40m
rook-ceph-block   rook-ceph.rbd.csi.ceph.com   Delete          Immediate              true                   11m
```

再次修改pvc大小

```
kubectl edit pvc test-nginx
```

进入容器查看磁盘大小,发现磁盘大小已经变更

```bash
$ kubectl exec -it task-pv-pod -- /bin/bash
$ df -h
/dev/rbd0                          2.0G   24K  2.0G   1% /usr/share/nginx/html
```

