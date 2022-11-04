---
title: "Etcd简单使用"
date: 2022-11-04T15:12:02+08:00
tags: ["k8s"]
---

mac安装

```bash
brew install etcd
brew services list
brew services etcd
```

命令行使用

```bash
$ etcdctl version
etcdctl version: 3.5.5
API version: 3.5
```

创建资源操作

```bash
# 创建
etcdctl put /user/101/name inksnw
etcdctl put /user/101/age 19
# 查看
etcdctl get /user/101/name
# 前缀查看
etcdctl get /user/101  --prefix
```

## 集群版安装

创建 docker-compose文件

```yaml
version: "3.7"
 
services:
  etcd0:
    image: docker.io/bitnami/etcd:3.5
    container_name: etcd0
    ports:
      - "23800:2380" #宿主:容器
      - "23790:2379"
    environment:
      - ALLOW_NONE_AUTHENTICATION=yes
      - ETCD_NAME=etcd0
      - ETCD_DATA_DIR=/var/etcd/etcd0
      - ETCD_LISTEN_PEER_URLS=http://0.0.0.0:2380 #集群内节点数据交换监听地址
      - ETCD_LISTEN_CLIENT_URLS=http://0.0.0.0:2379 #本节点访问地址
      - ETCD_ADVERTISE_CLIENT_URLS=http://192.168.1.105:23790 #通知其他节点，客户端接入本节点的监听地址
      - ETCD_INITIAL_ADVERTISE_PEER_URLS=http://etcd0:2380 #通知其他节点与本节点进行数据交换的地址
      - ETCD_INITIAL_CLUSTER_TOKEN=etcd-cluster #集群唯一标识
      - ETCD_INITIAL_CLUSTER=etcd0=http://etcd0:2380,etcd1=http://etcd1:2380,etcd2=http://etcd2:2380 #集群所有节点
      - ETCD_INITIAL_CLUSTER_STATE=new #new-不存在对应集群时创建新集群，existing-不存在对应集群时节点创建失败
 
  etcd1:
    image: docker.io/bitnami/etcd:3.5
    container_name: etcd1
    ports:
      - "23801:2380"
      - "23791:2379"
    environment:
      - ALLOW_NONE_AUTHENTICATION=yes
      - ETCD_NAME=etcd1
      - ETCD_DATA_DIR=/var/etcd/etcd1
      - ETCD_LISTEN_PEER_URLS=http://0.0.0.0:2380
      - ETCD_LISTEN_CLIENT_URLS=http://0.0.0.0:2379
      - ETCD_ADVERTISE_CLIENT_URLS=http://192.168.1.105:23791
      - ETCD_INITIAL_ADVERTISE_PEER_URLS=http://etcd1:2380
      - ETCD_INITIAL_CLUSTER_TOKEN=etcd-cluster
      - ETCD_INITIAL_CLUSTER=etcd0=http://etcd0:2380,etcd1=http://etcd1:2380,etcd2=http://etcd2:2380
      - ETCD_INITIAL_CLUSTER_STATE=new
 
  etcd2:
    image: docker.io/bitnami/etcd:3.5
    container_name: etcd2
    ports:
      - "23802:2380"
      - "23792:2379"
    environment:
      - ALLOW_NONE_AUTHENTICATION=yes
      - ETCD_NAME=etcd2
      - ETCD_DATA_DIR=/var/etcd/etcd2
      - ETCD_LISTEN_PEER_URLS=http://0.0.0.0:2380
      - ETCD_LISTEN_CLIENT_URLS=http://0.0.0.0:2379
      - ETCD_ADVERTISE_CLIENT_URLS=http://192.168.1.105:23792
      - ETCD_INITIAL_ADVERTISE_PEER_URLS=http://etcd2:2380
      - ETCD_INITIAL_CLUSTER_TOKEN=etcd-cluster
      - ETCD_INITIAL_CLUSTER=etcd0=http://etcd0:2380,etcd1=http://etcd1:2380,etcd2=http://etcd2:2380
      - ETCD_INITIAL_CLUSTER_STATE=new
```

运行查看

```bash
$ docker exec -it 99df13bc0f5b /bin/bash
$ etcdctl -w table --endpoints=etcd0:2379,etcd1:2379,etcd2:2379 endpoint status
+------------+------------------+---------+---------+-----------+------------+-----------+------------+--------------------+--------+
|  ENDPOINT  |        ID        | VERSION | DB SIZE | IS LEADER | IS LEARNER | RAFT TERM | RAFT INDEX | RAFT APPLIED INDEX | ERRORS |
+------------+------------------+---------+---------+-----------+------------+-----------+------------+--------------------+--------+
| etcd0:2379 | cf1d15c5d194b5c9 |   3.5.5 |   20 kB |     false |      false |         8 |         26 |                 26 |        |
| etcd1:2379 | ade526d28b1f92f7 |   3.5.5 |   20 kB |      true |      false |         8 |         26 |                 26 |        |
| etcd2:2379 | d282ac2ce600c1ce |   3.5.5 |   20 kB |     false |      false |         8 |         26 |                 26 |        |
+------------+------------------+---------+---------+-----------+------------+-----------+------------+--------------------+--------+
```

从容器外访问

```bash
etcdctl --endpoints http://192.168.50.231:23790 member list
```

