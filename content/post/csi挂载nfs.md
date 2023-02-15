---
title: "csi挂载nfs"
date: 2023-02-14T19:11:47+08:00
tags: ["k8s"]
---

## 配置NFS

安装nfs

```bash
➜ apt install nfs-kernel-server
# 查看导出的nfs
➜ cat /etc/exports
# 重新导出
➜ exportfs -a
➜ systemctl status nfs-server
# 挂载配置情况
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

挂载与查看

```bash
➜ mount -t nfs 192.168.50.20:/data/public /root/abc
➜ df -h
Filesystem                         Size  Used Avail Use% Mounted on
udev                               3.9G     0  3.9G   0% /dev
/dev/mapper/ubuntu--vg-ubuntu--lv   24G   11G   13G  47% /
/dev/vda2                          974M  106M  801M  12% /boot
192.168.50.20:/data/public         916G     0  870G   0% /root/abc
```

卸载

```
➜ umount /root/abc
```

## 配置CSI

### 安装nfs-csi-driver

官方文档: https://github.com/kubernetes-csi/csi-driver-nfs/blob/master/docs/install-nfs-csi-driver.md

```bash
git clone https://github.com/kubernetes-csi/csi-driver-nfs.git
cd csi-driver-nfs
./deploy/install-driver.sh v4.1.0 local
```

查看安装的pod

```bash
➜ kubectl get pod -n kube-system|grep csi
csi-nfs-controller-78d56d9785-52zgz            3/3     Running   0             74s
csi-nfs-node-c5sq7                             3/3     Running   0             74s
csi-nfs-node-h5zsk                             3/3     Running   0             74s
csi-nfs-node-x64bx                             3/3     Running   0             74s
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

### 创建sc与pvc

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

查看信息

```bash
➜ kubectl get sc
NAME              PROVISIONER        RECLAIMPOLICY   VOLUMEBINDINGMODE      ALLOWVOLUMEEXPANSION   AGE
local (default)   openebs.io/local   Delete          WaitForFirstConsumer   false                  33h
nfs-csi           nfs.csi.k8s.io     Delete          Immediate              false                  9s
➜ kubectl get pvc
NAME                    STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS   AGE
data-dmp-postgresql-0   Bound    pvc-5b0edcbc-73cc-4c6a-84c8-46e4e75cd30c   8Gi        RWO            local          33h
nfs-pvc                 Bound    pvc-b970a9dc-5552-47db-9fa6-2c34513e394c   10Gi       RWX            nfs-csi        8s
```

### 使用pvc

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

进入Pod查看

```bash
# 在nfs server上创建一个文件
➜ cd /data/public/pvc-b970a9dc-5552-47db-9fa6-2c34513e394c
➜ touch a.txt
# 在pod内查看
➜ kubectl exec -it mynginx-7968d6dcd5-df5px -- /bin/bash
➜ cd /data
➜ ls
```

