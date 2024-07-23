---
title: "Helm的oci源"
date: 2024-07-23T11:49:25+08:00
tags: ["k8s"]
---

从Helm 3开始，可以使用具有 [OCI](https://www.opencontainers.org/)支持的容器注册中心来存储和共享chart包。从Helm v3.8.0开始，默认启用OCI支持。

详细的操作[官方文档](https://helm.sh/zh/docs/topics/registries/#v380%E7%89%88%E6%9C%AC%E4%B9%8B%E5%89%8D%E5%AF%B9-oci-%E7%9A%84%E6%94%AF%E6%8C%81)已经很清楚了, 这里就不再抄一遍了

说几点不同的

### 不支持添加为repo

oci模式无法添加为本地repo

```bash
# 无法添加
helm repo add test oci://
Error: looks like "oci://hub.kubesphere.com.cn/kse/ks-core" is not a valid chart repository or cannot be reached: object required
```

### helm拉取

```bash
helm pull oci://hub.kubesphere.com.cn/kse/ks-core --version 1.0.8
```

### curl模拟

由于使用的是oci标准, 那么应该是可以使用标准docker镜像的拉取方式

#### 查看版本

```bash
curl -X GET -H "Accept: application/vnd.oci.image.manifest.v1+json" -u "lanjing:xxx" https://hub.kubesphere.com.cn/v2/kse/ks-core/tags/list   
```
```json
{
    "name": "kse/ks-core",
    "tags": [
        "1.0.8",
        "1.0.8-20240708-1",
        "1.0.8-20240708-2",
        "1.0.9",
        "v4.1.0"
    ]
}
```

#### 查看特定版本manifest

```bash
# 这个地址是开放的, 但必须输入一个密码参数, 验证了下输入错误的也可以
curl -X GET -H "Accept: application/vnd.oci.image.manifest.v1+json" -u "lanjing:xxx" https://hub.kubesphere.com.cn/v2/kse/ks-core/manifests/1.0.8
```

得到oci的manifest文件

```json
{
    "schemaVersion": 2,
    "config": {
        "mediaType": "application/vnd.cncf.helm.config.v1+json",
        "digest": "sha256:28852339ce55e0df692d7e01575a59afbc2856aa31e1cc7e983b7dfee71b71cf",
        "size": 277
    },
    "layers": [
        {
            "mediaType": "application/vnd.cncf.helm.chart.content.v1.tar+gzip",
            "digest": "sha256:df1de00455d13b92aa16314cb7f679e91cd4a70168d2a89da4bb2252689a6945",
            "size": 81533
        }
    ],
    "annotations": {
        "org.opencontainers.image.created": "2024-07-01T06:09:41Z",
        "org.opencontainers.image.description": "A Helm chart for KubeSphere Core components",
        "org.opencontainers.image.title": "ks-core",
        "org.opencontainers.image.version": "1.0.8"
    }
}
```

#### 拉取一层

```bash
curl -X GET -u "lanjing:xxx" -o ks-core-1.0.8.tgz https://hub.kubesphere.com.cn/v2/kse/ks-core/blobs/sha256:df1de00455d13b92aa16314cb7f679e91cd4a70168d2a89da4bb2252689a6945
```

成功拉取

```bash
d➜  tree
.
└── ks-core-1.0.8.tgz

1 directory, 1 file
```

