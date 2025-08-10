---
title: "OCI规范-Image spec"
date: 2022-11-28T17:17:41+08:00
tags: ["k8s"]
---

## OCI 规范说起

OCI（Open Container Initiative）规范是事实上的容器标准,OCI 规范分为 `Image spec` 和[ Runtime spec ](http://inksnw.asuscomm.com:3001/post/runc/)两部分，它们分别覆盖了容器生命周期的不同阶段：

## 镜像规范

镜像规范定义了如何创建一个符合 OCI 规范的镜像，它规定了镜像的构建系统需要输出的内容和格式，输出的容器镜像可以被解包成一个 `runtime bundle` ，`runtime bundle` 是由特定文件`config.json`和目录结构`rootfs`组成的一个文件夹，从中可以根据运行时标准运行容器。

### 镜像里面都有什么

规范要求镜像内容必须包括以下 3 部分：

- **Image Manifest**：提供了镜像的配置和文件系统层定位信息，可以看作是镜像的目录，文件格式为 `json` 。
- **Image Layer Filesystem Changeset**：序列化之后的文件系统和文件系统变更，它们可按顺序一层层应用为一个容器的 rootfs，因此通常也被称为一个 `layer`，文件格式可以是 `tar` ，`gzip` 等存档或压缩格式。
- **Image Configuration**：包含了镜像在运行时所使用的执行参数以及有序的 rootfs 变更信息，文件类型为 `json`。

*rootfs (root file system)即 `/` 根挂载点所挂载的文件系统，是一个操作系统所包含的文件、配置和目录，但并不包括操作系统内核，同一台机器上的所有容器都共享宿主机操作系统的内核。*

```bash
docker pull alpine
docker save alpine:latest -o alpine.tar
mkdir alpine-img
tar -xvf alpine.tar -C alpine-img/
```

查看目录中的文件结构

```bash
tree alpine-img/
alpine-img/
├── 49176f190c7e9cdb51ac85ab6c6d5e4512352218190cd69b08e6fd803ffbf3da.json runc用到的启动文件
├── 50b95c8857dc05c23a9b2373bcb0127cc4c391acf192a7fe7785e80f7bc0911a
│   ├── json
│   ├── layer.tar  容器的实际文件
│   └── VERSION    
├── manifest.json
└── repositories

1 directory, 6 files
```

查看 `manifest.json`

```json
python3 -m json.tool manifest.json 
[
    {
        "Config": "49176f190c7e9cdb51ac85ab6c6d5e4512352218190cd69b08e6fd803ffbf3da.json",
        "RepoTags": [
            "alpine:latest"
        ],
        "Layers": [
            "50b95c8857dc05c23a9b2373bcb0127cc4c391acf192a7fe7785e80f7bc0911a/layer.tar"
        ]
    }
]
```

### 手动构建一层

手动再创建一层,创建一个`Dockerfile`文件

```dockerfile
FROM alpine:latest
RUN mkdir /app
docker build -t myalpine:v1 .
```

再次解压

```bash
docker save myalpine:v1 -o myalpine.tar
mkdir myalpine-img
tar -xvf myalpine.tar -C myalpine-img/
```

查看目录

```
root@base:~/myalpine-img# tree
.
├── 0c4a0f1a7afeb071999833e4a5dca155fa81ffcec12dd868aaa6a221775bf8c5
│   ├── json
│   ├── layer.tar
│   └── VERSION
├── 388f75b341ac497d0e06391476932f25934115c46103aa4b600b1082ca0f843e
│   ├── json
│   ├── layer.tar
│   └── VERSION
├── 5422d176b37133117f885a2963bc44e410c64349430aae2fff84de42dea8ec5a.json
├── manifest.json
└── repositories

2 directories, 9 files
```

再次查看`manifest.json`

```json
root@base:~/myalpine-img# python3 -m json.tool manifest.json 
[
    {
        "Config": "5422d176b37133117f885a2963bc44e410c64349430aae2fff84de42dea8ec5a.json",
        "RepoTags": [
            "myalpine:v1"
        ],
        "Layers": [
            "0c4a0f1a7afeb071999833e4a5dca155fa81ffcec12dd868aaa6a221775bf8c5/layer.tar",
            "388f75b341ac497d0e06391476932f25934115c46103aa4b600b1082ca0f843e/layer.tar"
        ]
    }
]
```

解压查看,可以看到第二层只有一个app目录,与我们的`Dockerfile`配置一致

<img src="http://inksnw.asuscomm.com:3001/blog/容器技术原理-OCI规范_2eef73d14d6af274763c853300f0a7e1.png" alt="image-20221128213021570" style="zoom:50%;" />

## umoci制作镜像

```bash
wget https://github.com/opencontainers/umoci/releases/download/v0.4.7/umoci.amd64
```

解压上一步的`layer.tar` 到一个新的目录`myos`

```
➜ umoci init --layout myimage
➜ tree -L 1
.
├── myimage
└── myos

2 directories, 0 files
```

创建一个镜像,此时 `myimage/index.json` 会多出`manifests`配置信息用于`镜像分发`等

```
umoci new --image myimage:v1
```

将image提取到一个文件夹中

```bash
➜ umoci unpack --image myimage:v1 bundle
tree
.
├── config.json  runc对应的配置文件
├── rootfs       操作系统目录
├── sha256_0339cec20a876292977062c19cdc1ec9490454e88e1d7a1c961b40edf7483052.mtree
└── umoci.json

1 directory, 3 files
```

查看此时的镜像信息

```
➜ umoci stat --image myimage:v1
LAYER CREATED CREATED BY SIZE COMMENT
```

将第一步`myos`中的内容拷贝到`bundle/rootfs`

```
cp -r myos/* bundle/rootfs/
```

重新pack,并查看状态

```
➜ umoci repack --image myimage:v1 bundle
➜ umoci stat --image myimage:v1
LAYER                                                                   CREATED                        CREATED BY   SIZE    COMMENT
sha256:7c659bc85e3b04cfa17fdfb7c881dfd51816e08a4e5eb1adf5db7661afe4d39f 2022-11-28T13:58:06.485326153Z umoci repack 3.485MB 
```

